package kubequery

import (
	"context"
	"fmt"

	"github.com/equinor/radix-common/net/http"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func GetRadixBatchesForJobComponent(ctx context.Context, client radixclient.Interface, appName, envName, jobComponentName string, batchType kube.RadixBatchType) ([]radixv1.RadixBatch, error) {
	namespace := operatorutils.GetEnvironmentNamespace(appName, envName)
	selector := radixlabels.Merge(
		radixlabels.ForApplicationName(appName),
		radixlabels.ForComponentName(jobComponentName),
		radixlabels.ForBatchType(batchType),
	)

	batches, err := client.RadixV1().RadixBatches(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}

	return batches.Items, nil
}

// GetRadixBatches Get Radix batches
func GetRadixBatches(ctx context.Context, radixClient radixclient.Interface, appName, envName, jobComponentName string, batchType kube.RadixBatchType) ([]*radixv1.RadixBatch, error) {
	namespace := operatorutils.GetEnvironmentNamespace(appName, envName)
	selector := radixlabels.Merge(
		radixlabels.ForApplicationName(appName),
		radixlabels.ForComponentName(jobComponentName),
		radixlabels.ForBatchType(batchType),
	)
	radixBatchList, err := radixClient.RadixV1().RadixBatches(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	return slice.PointersOf(radixBatchList.Items).([]*radixv1.RadixBatch), nil
}

// GetRadixBatch Get Radix batch
func GetRadixBatch(ctx context.Context, radixClient radixclient.Interface, appName, envName, jobComponentName, batchName string, batchType kube.RadixBatchType) (*radixv1.RadixBatch, error) {
	namespace := operatorutils.GetEnvironmentNamespace(appName, envName)
	labelSelector := radixlabels.Merge(
		radixlabels.ForApplicationName(appName),
		radixlabels.ForComponentName(jobComponentName),
	)

	if batchType != "" {
		labelSelector = radixlabels.Merge(
			labelSelector,
			radixlabels.ForBatchType(batchType),
		)
	}

	radixBatch, err := radixClient.RadixV1().RadixBatches(namespace).Get(ctx, batchName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, batchNotFoundError(batchName)
		}
		return nil, err
	}

	if !labelSelector.AsSelector().Matches(labels.Set(radixBatch.GetLabels())) {
		return nil, batchNotFoundError(batchName)
	}

	return radixBatch, nil
}

func batchNotFoundError(batchName string) error {
	return http.NotFoundError(fmt.Sprintf("batch %s not found", batchName))
}
