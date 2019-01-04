package kube

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	configMapName                   = "radix-config"
	clusterNameConfig               = "clustername"
	infrastructureEnvironmentConfig = "infrastructureEnvironment"
)

// GetClusterName Gets the global name of the cluster from config map in default namespace
func (kube *Kube) GetClusterName() (string, error) {
	radixconfigmap, err := kube.kubeClient.CoreV1().ConfigMaps(corev1.NamespaceDefault).Get(configMapName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("Failed to get radix config map: %v", err)
	}
	clustername := radixconfigmap.Data[clusterNameConfig]
	logger.Infof("Cluster name: %s", clustername)
	return clustername, nil
}

// GetInfrastructureEnvironment Gets the global environment (dev/prod) from config map in default namespace
func (kube *Kube) GetInfrastructureEnvironment() (string, error) {
	radixconfigmap, err := kube.kubeClient.CoreV1().ConfigMaps(corev1.NamespaceDefault).Get(configMapName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("Failed to get radix config map: %v", err)
	}
	infrastructureEnvironment := radixconfigmap.Data[infrastructureEnvironmentConfig]
	logger.Infof("Infrastructure environment: %s", infrastructureEnvironment)
	return infrastructureEnvironment, nil
}
