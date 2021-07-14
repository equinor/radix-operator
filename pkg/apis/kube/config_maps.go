package kube

import (
	"context"
	"encoding/json"
	"fmt"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	configMapName               = "radix-config"
	clusterNameConfig           = "clustername"
	containerRegistryConfig     = "containerRegistry"
	envVarsPrefix               = "env-vars"          //Environment variables
	envVarsMetadataPrefix       = "env-vars-metadata" //Metadata for environment variables
	envVarsMetadataPropertyName = "metadata"          //Metadata property for environment variables in config-map
)

//GetEnvVarsConfigMapName Get config-map name for environment variables
func GetEnvVarsConfigMapName(componentName string) string {
	return fmt.Sprintf("%s-%s", envVarsPrefix, componentName)
}

//GetEnvVarsMetadataConfigMapName Get config-map name for environment variables metadata
func GetEnvVarsMetadataConfigMapName(componentName string) string {
	return fmt.Sprintf("%s-%s", envVarsMetadataPrefix, componentName)
}

// GetClusterName Gets the global name of the cluster from config map in default namespace
func (kubeutil *Kube) GetClusterName() (string, error) {
	return kubeutil.getConfigFromMap(clusterNameConfig)
}

// GetContainerRegistry Gets the container registry from config map in default namespace
func (kubeutil *Kube) GetContainerRegistry() (string, error) {
	return kubeutil.getConfigFromMap(containerRegistryConfig)
}

func (kubeutil *Kube) getConfigFromMap(config string) (string, error) {
	radixconfigmap, err := kubeutil.GetConfigMap(corev1.NamespaceDefault, configMapName)
	if err != nil {
		return "", fmt.Errorf("Failed to get radix config map: %v", err)
	}
	configValue := radixconfigmap.Data[config]
	logger.Debugf("%s: %s", config, configValue)
	return configValue, nil
}

// CreateConfigMap Create config map by name
func (kubeutil *Kube) CreateConfigMap(namespace, name string, labels map[string]string) (*corev1.ConfigMap, error) {
	return kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).Create(context.TODO(),
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: labels,
			},
			Data: make(map[string]string, 0),
		},
		metav1.CreateOptions{})
}

// GetConfigMap Gets config map by name
func (kubeutil *Kube) GetConfigMap(namespace, name string) (*corev1.ConfigMap, error) {
	var configMap *corev1.ConfigMap
	var err error

	if kubeutil.ConfigMapLister != nil {
		configMap, err = kubeutil.ConfigMapLister.ConfigMaps(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		configMap, err = kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		log.Debugf("Created environment variables ConfigMap  '%s'", configMap.GetName())
	}

	return configMap, nil
}

//ApplyConfigMap Save changes of environment-variables to config-map
func (kubeutil *Kube) ApplyConfigMap(namespace string, currentConfigMap, desiredConfigMap *corev1.ConfigMap) error {
	currentConfigMapJSON, err := json.Marshal(currentConfigMap)
	if err != nil {
		return fmt.Errorf("Failed to marshal old config-map object: %v", err)
	}

	desiredConfigMapJSON, err := json.Marshal(desiredConfigMap)
	if err != nil {
		return fmt.Errorf("Failed to marshal new config-map object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(currentConfigMapJSON, desiredConfigMapJSON, corev1.ConfigMap{})
	if err != nil {
		return fmt.Errorf("Failed to create two way merge patch config-map objects: %v", err)
	}

	if IsEmptyPatch(patchBytes) {
		log.Debugf("No need to patch config-map: %s ", currentConfigMap.GetName())
		return nil
	}

	log.Debugf("Patch: %s", string(patchBytes))
	patchedConfigMap, err := kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).Patch(context.TODO(), currentConfigMap.GetName(), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("Failed to patch config-map object: %v", err)
	}
	log.Debugf("Patched config-map: %s in namespace %s", patchedConfigMap.Name, namespace)
	return err
}

//GetEnvVarsMetadataFromConfigMap Get environment-variables metadata from config-map
func GetEnvVarsMetadataFromConfigMap(envVarsMetadataConfigMap *corev1.ConfigMap) (map[string]v1.EnvVarMetadata, error) {
	envVarsMetadata, ok := envVarsMetadataConfigMap.Data[envVarsMetadataPropertyName]
	if !ok {
		return map[string]v1.EnvVarMetadata{}, nil
	}
	envVarsMetadataMap := make(map[string]v1.EnvVarMetadata, 0)
	err := json.Unmarshal([]byte(envVarsMetadata), &envVarsMetadataMap)
	if err != nil {
		return nil, err
	}
	return envVarsMetadataMap, nil
}

//ApplyEnvVarsMetadataConfigMap Save changes of environment-variables metadata to config-map
func (kubeutil *Kube) ApplyEnvVarsMetadataConfigMap(namespace string, currentEnvVarsMetadataConfigMap *corev1.ConfigMap, envVarsMetadataMap map[string]v1.EnvVarMetadata) error {
	desiredEnvVarsMetadataConfigMap := currentEnvVarsMetadataConfigMap.DeepCopy()
	envVarsMetadata, err := json.Marshal(envVarsMetadataMap)
	if err != nil {
		return err
	}
	desiredEnvVarsMetadataConfigMap.Data[envVarsMetadataPropertyName] = string(envVarsMetadata)
	return kubeutil.ApplyConfigMap(namespace, currentEnvVarsMetadataConfigMap, desiredEnvVarsMetadataConfigMap)
}

//GetOrCreateEnvVarsConfigMapAndMetadataMap Get environment variables and its metadata config-maps
func (kubeutil *Kube) GetOrCreateEnvVarsConfigMapAndMetadataMap(namespace, appName, componentName string) (*corev1.ConfigMap, *corev1.ConfigMap, error) {
	envVarConfigMap, err := kubeutil.getOrCreateRadixConfigEnvVarsConfigMap(namespace, appName, componentName)
	if err != nil {
		err := fmt.Errorf("failed to create config-map for environment variables methadata: %v", err)
		log.Error(err)
		return nil, nil, err
	}
	envVarMetadataConfigMap, err := kubeutil.getOrCreateRadixConfigEnvVarsMetadataConfigMap(namespace, appName, componentName)
	if err != nil {
		err := fmt.Errorf("failed to create config-map for environment variables methadata: %v", err)
		log.Error(err)
		return nil, nil, err
	}
	return envVarConfigMap, envVarMetadataConfigMap, err
}

func (kubeutil *Kube) getOrCreateRadixConfigEnvVarsConfigMap(namespace, appName, componentName string) (*corev1.ConfigMap, error) {
	configMapName := GetEnvVarsConfigMapName(componentName)
	configMapType := v1.EnvVarsConfigMap
	return kubeutil.getOrCreateRadixConfigEnvVarsConfigMapForType(configMapType, configMapName, namespace, appName, componentName)
}

func (kubeutil *Kube) getOrCreateRadixConfigEnvVarsMetadataConfigMap(namespace, appName, componentName string) (*corev1.ConfigMap, error) {
	configMapName := GetEnvVarsMetadataConfigMapName(componentName)
	configMapType := v1.EnvVarsMetadataConfigMap
	return kubeutil.getOrCreateRadixConfigEnvVarsConfigMapForType(configMapType, configMapName, namespace, appName, componentName)
}

func (kubeutil *Kube) getOrCreateRadixConfigEnvVarsConfigMapForType(configMapType v1.RadixConfigMapType, configMapName, namespace, appName, componentName string) (*corev1.ConfigMap, error) {
	configMap, err := kubeutil.GetConfigMap(namespace, configMapName)
	if err != nil {
		statusError := err.(*k8sErrors.StatusError)
		if statusError == nil || statusError.ErrStatus.Reason != metav1.StatusReasonNotFound {
			return nil, err
		}
	}
	if configMap == nil {
		labels := map[string]string{
			RadixAppLabel:       appName,
			RadixComponentLabel: componentName,
			RadixConfigMapType:  string(configMapType),
		}
		configMap, err = kubeutil.CreateConfigMap(namespace, configMapName, labels)
		if err != nil {
			return nil, err
		}
	}
	if configMap.Data == nil {
		configMap.Data = make(map[string]string, 0)
	}
	return configMap, err
}
