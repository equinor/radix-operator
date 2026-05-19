package kubequery

import (
	"context"

	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetHorizontalPodAutoscalersForEnvironment returns all HorizontalPodAutoscalers for the specified application and environment.
func GetHorizontalPodAutoscalersForEnvironment(ctx context.Context, client kubernetes.Interface, appName, envName string) ([]autoscalingv2.HorizontalPodAutoscaler, error) {
	ns := operatorUtils.GetEnvironmentNamespace(appName, envName)
	hpas, err := client.AutoscalingV2().HorizontalPodAutoscalers(ns).List(ctx, metav1.ListOptions{LabelSelector: labels.ForApplicationName(appName).String()})
	if err != nil {
		return nil, err
	}
	return hpas.Items, nil
}
