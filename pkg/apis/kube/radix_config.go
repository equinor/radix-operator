package kube

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	configMapName           = "radix-config"
	clusterNameConfig       = "clustername"
	containerRegistryConfig = "containerRegistry"
	subscriptionIdConfig    = "subscriptionId"
	clusterActiveEgressIps  = "clusterActiveEgressIps"
)

// GetClusterName Gets the global name of the cluster from config map in default namespace
func (kubeutil *Kube) GetClusterName() (string, error) {
	return kubeutil.getRadixConfigFromMap(clusterNameConfig)
}

//GetSubscriptionId Gets the subscription-id from config map in default namespace
func (kubeutil *Kube) GetSubscriptionId() (string, error) {
	return kubeutil.getRadixConfigFromMap(subscriptionIdConfig)
}

// GetClusterActiveEgressIps Gets cluster active ips from config map in default namespace
func (kubeutil *Kube) GetClusterActiveEgressIps() (string, error) {
	return kubeutil.getRadixConfigFromMap(clusterActiveEgressIps)
}

func (kubeutil *Kube) getRadixConfigFromMap(config string) (string, error) {
	radixconfigmap, err := kubeutil.GetConfigMap(corev1.NamespaceDefault, configMapName)
	if err != nil {
		return "", fmt.Errorf("failed to get radix config map: %v", err)
	}
	configValue := radixconfigmap.Data[config]
	logger.Debugf("%s: %s", config, configValue)
	return configValue, nil
}
