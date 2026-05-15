package kubequery

import (
	"context"

	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetScaledObjectsForEnvironment returns all ScaledObjects for the specified application and environment.
func GetScaledObjectsForEnvironment(ctx context.Context, kedaClient versioned.Interface, appName, envName string) ([]v1alpha1.ScaledObject, error) {
	ns := operatorUtils.GetEnvironmentNamespace(appName, envName)
	scaledObjects, err := kedaClient.KedaV1alpha1().ScaledObjects(ns).List(ctx, metav1.ListOptions{LabelSelector: labels.ForApplicationName(appName).String()})
	if err != nil {
		return nil, err
	}
	return scaledObjects.Items, nil
}
