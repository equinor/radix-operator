package kubequery

import (
	"context"

	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetDeploymentsForEnvironment returns all Deployments for the specified application and environment.
func GetDeploymentsForEnvironment(ctx context.Context, client kubernetes.Interface, appName, envName string) ([]appsv1.Deployment, error) {
	ns := operatorUtils.GetEnvironmentNamespace(appName, envName)
	deployments, err := client.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{LabelSelector: labels.ForApplicationName(appName).String()})
	if err != nil {
		return nil, err
	}
	return deployments.Items, nil
}
