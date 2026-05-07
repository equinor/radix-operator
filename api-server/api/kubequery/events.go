package kubequery

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetEventsForEnvironment returns all Events for the specified application and environment.
func GetEventsForEnvironment(ctx context.Context, client kubernetes.Interface, appName, envName string) ([]corev1.Event, error) {
	ns := utils.GetEnvironmentNamespace(appName, envName)
	eventList, err := client.CoreV1().Events(ns).List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return eventList.Items, nil
}
