package kubequery

import (
	"context"

	"github.com/equinor/radix-operator/api-server/api/utils/labelselector"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
)

// GetPodsForEnvironmentComponents returns all Radix component pods for the specified application and environments.
func GetPodsForEnvironmentComponents(ctx context.Context, client kubernetes.Interface, appName, envName string) ([]corev1.Pod, error) {
	ns := operatorUtils.GetEnvironmentNamespace(appName, envName)
	noJobTypeLabel, err := labels.NewRequirement(kube.RadixJobTypeLabel, selection.DoesNotExist, nil)
	if err != nil {
		return nil, err
	}
	noBatchNameLabel, err := labels.NewRequirement(kube.RadixBatchNameLabel, selection.DoesNotExist, nil)
	if err != nil {
		return nil, err
	}
	selector := labelselector.ForApplication(appName).AsSelector().Add(*noJobTypeLabel, *noBatchNameLabel)
	pods, err := client.CoreV1().Pods(ns).List(ctx, v1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	return pods.Items, nil
}
