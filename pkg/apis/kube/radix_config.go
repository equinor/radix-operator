package kube

import (
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

const (
	configMapName                 = "radix-config"
	clusterNameConfig             = "clustername"
	containerRegistryConfig       = "containerRegistry"
	subscriptionIdConfig          = "subscriptionId"
	clusterActiveEgressIps        = "clusterActiveEgressIps"
	registrationControllerThreads = "registrationControllerThreads"
	applicationControllerThreads  = "applicationControllerThreads"
	environmentControllerThreads  = "environmentControllerThreads"
	deploymentControllerThreads   = "deploymentControllerThreads"
	jobControllerThreads          = "jobControllerThreads"
	alertControllerThreads        = "alertControllerThreads"
	kubeClientRateLimitBurst      = "kubeClientRateLimitBurst"
	kubeClientRateLimitQPS        = "kubeClientRateLimitQPS"
)

func (kubeutil *Kube) getIntValFromConfigMap(varName string) (int, error) {
	strVal, err := kubeutil.getRadixConfigFromMap(varName)
	if err != nil {
		return 0, err
	}
	intVal, err := strconv.Atoi(strVal)
	if err != nil {
		return 0, err
	}
	return intVal, nil
}

// GetClusterName Gets the global name of the cluster from config map in default namespace
func (kubeutil *Kube) GetClusterName() (string, error) {
	return kubeutil.getRadixConfigFromMap(clusterNameConfig)
}

// GetContainerRegistry Gets the container registry from config map in default namespace
func (kubeutil *Kube) GetContainerRegistry() (string, error) {
	return kubeutil.getRadixConfigFromMap(containerRegistryConfig)
}

//GetSubscriptionId Gets the subscription-id from config map in default namespace
func (kubeutil *Kube) GetSubscriptionId() (string, error) {
	return kubeutil.getRadixConfigFromMap(subscriptionIdConfig)
}

// GetClusterActiveEgressIps Gets cluster active ips from config map in default namespace
func (kubeutil *Kube) GetClusterActiveEgressIps() (string, error) {
	return kubeutil.getRadixConfigFromMap(clusterActiveEgressIps)
}

// GetRegistrationControllerThreads Gets number of parallel threads for registration controller
func (kubeutil *Kube) GetRegistrationControllerThreads() (int, error) {
	return kubeutil.getIntValFromConfigMap(registrationControllerThreads)
}

// GetApplicationControllerThreads Gets number of parallel threads for application controller
func (kubeutil *Kube) GetApplicationControllerThreads() (int, error) {
	return kubeutil.getIntValFromConfigMap(applicationControllerThreads)
}

// GetEnvironmentControllerThreads Gets number of parallel threads for environment controller
func (kubeutil *Kube) GetEnvironmentControllerThreads() (int, error) {
	return kubeutil.getIntValFromConfigMap(environmentControllerThreads)
}

// GetDeploymentControllerThreads Gets number of parallel threads for deployment controller
func (kubeutil *Kube) GetDeploymentControllerThreads() (int, error) {
	return kubeutil.getIntValFromConfigMap(deploymentControllerThreads)
}

// GetJobControllerThreads Gets number of parallel threads for job controller
func (kubeutil *Kube) GetJobControllerThreads() (int, error) {
	return kubeutil.getIntValFromConfigMap(jobControllerThreads)
}

// GetAlertControllerThreads Gets number of parallel threads for alert controller
func (kubeutil *Kube) GetAlertControllerThreads() (int, error) {
	return kubeutil.getIntValFromConfigMap(alertControllerThreads)
}

// GetKubeClientRateLimitBurst Gets burst limit for kube client
func (kubeutil *Kube) GetKubeClientRateLimitBurst() (int, error) {
	return kubeutil.getIntValFromConfigMap(kubeClientRateLimitBurst)
}

// GetKubeClientRateLimitQPS Gets burst limit for kube client
func (kubeutil *Kube) GetKubeClientRateLimitQPS() (float32, error) {
	strVal, err := kubeutil.getRadixConfigFromMap(kubeClientRateLimitQPS)
	if err != nil {
		return 0, err
	}
	intVal, err := strconv.ParseFloat(strVal, 32)
	if err != nil {
		return 0, err
	}
	return float32(intVal), nil
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
