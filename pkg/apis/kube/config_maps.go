package kube

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	configMapName           = "radix-config"
	clusterNameConfig       = "clustername"
	containerRegistryConfig = "containerRegistry"
)

// GetClusterName Gets the global name of the cluster from config map in default namespace
func (kube *Kube) GetClusterName() (string, error) {
	return kube.getConfigFromMap(clusterNameConfig)
}

// GetContainerRegistry Gets the container registry from config map in default namespace
func (kube *Kube) GetContainerRegistry() (string, error) {
	return kube.getConfigFromMap(containerRegistryConfig)
}

func (kube *Kube) getConfigFromMap(config string) (string, error) {
	radixconfigmap, err := kube.GetConfigMap(corev1.NamespaceDefault, configMapName)
	if err != nil {
		return "", fmt.Errorf("Failed to get radix config map: %v", err)
	}
	configValue := radixconfigmap.Data[config]
	logger.Debugf("%s: %s", config, configValue)
	return configValue, nil
}

// GetConfigMap Gets config map by name
func (kube *Kube) GetConfigMap(namespace, name string) (*corev1.ConfigMap, error) {
	var configMap *corev1.ConfigMap
	var err error

	if kube.ConfigMapLister != nil {
		configMap, err = kube.ConfigMapLister.ConfigMaps(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		configMap, err = kube.kubeClient.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return configMap, nil
}
