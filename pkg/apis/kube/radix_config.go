package kube

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	configMapName          = "radix-config"
	clusterNameConfig      = "clustername"
	subscriptionIdConfig   = "subscriptionId"
	clusterActiveEgressIps = "clusterActiveEgressIps"
)

// GetClusterName Gets the global name of the cluster from config map in default namespace
func (kubeutil *Kube) GetClusterName(ctx context.Context) (string, error) {
	return kubeutil.getRadixConfigFromMap(ctx, clusterNameConfig)
}

// GetSubscriptionId Gets the subscription-id from config map in default namespace
func (kubeutil *Kube) GetSubscriptionId(ctx context.Context) (string, error) {
	return kubeutil.getRadixConfigFromMap(ctx, subscriptionIdConfig)
}

// GetClusterActiveEgressIps Gets cluster active ips from config map in default namespace
func (kubeutil *Kube) GetClusterActiveEgressIps(ctx context.Context) (string, error) {
	return kubeutil.getRadixConfigFromMap(ctx, clusterActiveEgressIps)
}

func (kubeutil *Kube) getRadixConfigFromMap(ctx context.Context, config string) (string, error) {
	radixconfigmap, err := kubeutil.GetConfigMap(ctx, corev1.NamespaceDefault, configMapName)
	if err != nil {
		return "", fmt.Errorf("failed to get radix config map: %v", err)
	}
	configValue := radixconfigmap.Data[config]
	return configValue, nil
}
