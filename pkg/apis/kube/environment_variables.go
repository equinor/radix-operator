package kube

import (
	"encoding/json"
	"fmt"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
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

//GetEnvVarsConfigMapAndMetadataMap Get environment-variables config-map, environment-variables metadata config-map and metadata map from it
func (kubeutil *Kube) GetEnvVarsConfigMapAndMetadataMap(namespace string, componentName string) (*corev1.ConfigMap, *corev1.ConfigMap, map[string]v1.EnvVarMetadata, error) {
	envVarsConfigMap, err := kubeutil.GetConfigMap(namespace, GetEnvVarsConfigMapName(componentName))
	if err != nil {
		return nil, nil, nil, err
	}
	envVarsMetadataConfigMap, envVarsMetadataMap, err := kubeutil.GetEnvVarsMetadataConfigMapAndMap(namespace, componentName)
	if err != nil {
		return nil, nil, nil, err
	}
	return envVarsConfigMap, envVarsMetadataConfigMap, envVarsMetadataMap, nil
}

//GetEnvVarsMetadataConfigMapAndMap Get environment-variables metadata config-map and map from it
func (kubeutil *Kube) GetEnvVarsMetadataConfigMapAndMap(namespace string, componentName string) (*corev1.ConfigMap, map[string]v1.EnvVarMetadata, error) {
	envVarsMetadataConfigMap, err := kubeutil.GetConfigMap(namespace, GetEnvVarsMetadataConfigMapName(componentName))
	if err != nil {
		return nil, nil, err
	}
	envVarsMetadataMap, err := GetEnvVarsMetadataFromConfigMap(envVarsMetadataConfigMap)
	if err != nil {
		return nil, nil, err
	}
	return envVarsMetadataConfigMap, envVarsMetadataMap, nil
}

//ApplyEnvVarsMetadataConfigMap Save changes of environment-variables metadata to config-map
func (kubeutil *Kube) ApplyEnvVarsMetadataConfigMap(namespace string, currentEnvVarsMetadataConfigMap *corev1.ConfigMap, envVarsMetadataMap map[string]v1.EnvVarMetadata) error {
	desiredEnvVarsMetadataConfigMap := currentEnvVarsMetadataConfigMap.DeepCopy()
	err := kubeutil.SetEnvVarsMetadataMapToConfigMap(envVarsMetadataMap, desiredEnvVarsMetadataConfigMap)
	if err != nil {
		return err
	}
	return kubeutil.ApplyConfigMap(namespace, currentEnvVarsMetadataConfigMap, desiredEnvVarsMetadataConfigMap)
}

//SetEnvVarsMetadataMapToConfigMap Set environment-variables metadata to config-map
func (kubeutil *Kube) SetEnvVarsMetadataMapToConfigMap(envVarsMetadataMap map[string]v1.EnvVarMetadata, configMap *corev1.ConfigMap) error {
	envVarsMetadata, err := json.Marshal(envVarsMetadataMap)
	if err != nil {
		return err
	}
	configMap.Data[envVarsMetadataPropertyName] = string(envVarsMetadata)
	return nil
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
	configMap, err := kubeutil.getRadixConfigEnvVarsConfigMap(namespace, GetEnvVarsConfigMapName(componentName))
	if err != nil {
		return nil, err
	}
	if configMap != nil {
		return configMap, nil
	}
	configMap = BuildRadixConfigEnvVarsConfigMap(appName, componentName)
	return kubeutil.CreateConfigMap(namespace, configMap)
}

func (kubeutil *Kube) getOrCreateRadixConfigEnvVarsMetadataConfigMap(namespace, appName, componentName string) (*corev1.ConfigMap, error) {
	configMap, err := kubeutil.getRadixConfigEnvVarsConfigMap(namespace, GetEnvVarsMetadataConfigMapName(componentName))
	if err != nil {
		return nil, err
	}
	if configMap != nil {
		return configMap, nil
	}
	configMap = BuildRadixConfigEnvVarsMetadataConfigMap(appName, componentName)
	return kubeutil.CreateConfigMap(namespace, configMap)
}

//BuildRadixConfigEnvVarsConfigMap Build environment-variables config-map
func BuildRadixConfigEnvVarsConfigMap(appName, componentName string) *corev1.ConfigMap {
	return buildRadixConfigEnvVarsConfigMapForType(v1.EnvVarsConfigMap, appName, componentName, GetEnvVarsConfigMapName(componentName))
}

//BuildRadixConfigEnvVarsMetadataConfigMap Build environment-variables metadata config-map
func BuildRadixConfigEnvVarsMetadataConfigMap(appName, componentName string) *corev1.ConfigMap {
	return buildRadixConfigEnvVarsConfigMapForType(v1.EnvVarsMetadataConfigMap, appName, componentName, GetEnvVarsMetadataConfigMapName(componentName))
}

func buildRadixConfigEnvVarsConfigMapForType(configMapType v1.RadixConfigMapType, appName, componentName, name string) *corev1.ConfigMap {
	labels := map[string]string{
		RadixAppLabel:       appName,
		RadixComponentLabel: componentName,
		RadixConfigMapType:  string(configMapType),
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Data: make(map[string]string, 0),
	}
}

func (kubeutil *Kube) getRadixConfigEnvVarsConfigMap(namespace, configMapName string) (*corev1.ConfigMap, error) {
	configMap, err := kubeutil.GetConfigMap(namespace, configMapName)
	if err != nil {
		statusError := err.(*k8sErrors.StatusError)
		if statusError == nil || statusError.ErrStatus.Reason != metav1.StatusReasonNotFound {
			return nil, err
		}
	}
	if configMap != nil && configMap.Data == nil {
		configMap.Data = make(map[string]string, 0)
	}
	return configMap, nil
}
