package radixconfig

import (
	"context"
	"fmt"
	"maps"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/equality"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ApplyRadixApplicationOperationResult string

const (
	ApplyRadixApplicationOperationResultCreated   ApplyRadixApplicationOperationResult = "created"
	ApplyRadixApplicationOperationResultUpdated   ApplyRadixApplicationOperationResult = "updated"
	ApplyRadixApplicationOperationResultUnchanged ApplyRadixApplicationOperationResult = "unchanged"
)

func ApplyRadixApplication(ctx context.Context, client radixclient.Interface, ra *radixv1.RadixApplication, dryRun bool) (*radixv1.RadixApplication, ApplyRadixApplicationOperationResult, error) {
	namespace := operatorutils.GetAppNamespace(ra.GetName())
	ra = ra.DeepCopy()

	var dryRunOptions []string
	if dryRun {
		dryRunOptions = []string{metav1.DryRunAll}
	}

	existingRA, err := client.RadixV1().RadixApplications(namespace).Get(ctx, ra.Name, metav1.GetOptions{})
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			created, err := client.RadixV1().RadixApplications(namespace).Create(ctx, ra, metav1.CreateOptions{DryRun: dryRunOptions})
			if err != nil {
				return nil, "", fmt.Errorf("failed to create RadixApplication: %w", err)
			}

			return created, ApplyRadixApplicationOperationResultCreated, nil
		}
		return nil, "", fmt.Errorf("failed to get existing RadixApplication: %w", err)
	}

	// Annotations are compared since they can change admission webhook behavior (e.g. radix-easyauth)
	if !dryRun && equality.Semantic.DeepEqual(ra.Spec, existingRA.Spec) && maps.Equal(ra.GetAnnotations(), existingRA.GetAnnotations()) {
		return existingRA, ApplyRadixApplicationOperationResultUnchanged, nil
	}

	// For an update, ResourceVersion of the new object must be the same with the old object
	ra.SetResourceVersion(existingRA.GetResourceVersion())
	updated, err := client.RadixV1().RadixApplications(namespace).Update(ctx, ra, metav1.UpdateOptions{DryRun: dryRunOptions})
	if err != nil {
		return nil, "", fmt.Errorf("failed to update RadixApplication: %w", err)
	}

	return updated, ApplyRadixApplicationOperationResultUpdated, nil
}
