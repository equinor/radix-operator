package kube

import (
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	envVarsPrefix               = "env-vars"          //Environment variables
	envVarsMetadataPrefix       = "env-vars-metadata" //Metadata for environment variables
	envVarsMetadataPropertyName = "metadata"          //Metadata property for environment variables in config-map
)

// EnvVarMetadata Metadata for environment variables
type EnvVarMetadata struct {
	RadixConfigValue string
}

// GetEnvVarsConfigMapName Get config-map name for environment variables
func GetEnvVarsConfigMapName(componentName string) string {
	return fmt.Sprintf("%s-%s", envVarsPrefix, componentName)
}

// GetEnvVarsMetadataConfigMapName Get config-map name for environment variables metadata
func GetEnvVarsMetadataConfigMapName(componentName string) string {
	return fmt.Sprintf("%s-%s", envVarsMetadataPrefix, componentName)
}

// GetEnvVarsMetadataFromConfigMap Get environment-variables metadata from config-map
func GetEnvVarsMetadataFromConfigMap(envVarsMetadataConfigMap *corev1.ConfigMap) (map[string]EnvVarMetadata, error) {
	envVarsMetadata, ok := envVarsMetadataConfigMap.Data[envVarsMetadataPropertyName]
	if !ok {
		return map[string]EnvVarMetadata{}, nil
	}
	envVarsMetadataMap := make(map[string]EnvVarMetadata, 0)
	err := json.Unmarshal([]byte(envVarsMetadata), &envVarsMetadataMap)
	if err != nil {
		return nil, err
	}
	return envVarsMetadataMap, nil
}

// GetEnvVarsConfigMapAndMetadataMap Get environment-variables config-map, environment-variables metadata config-map and metadata map from it
func (kubeutil *Kube) GetEnvVarsConfigMapAndMetadataMap(namespace string, componentName string) (*corev1.ConfigMap, *corev1.ConfigMap, map[string]EnvVarMetadata, error) {
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

// GetEnvVarsMetadataConfigMapAndMap Get environment-variables metadata config-map and map from it
func (kubeutil *Kube) GetEnvVarsMetadataConfigMapAndMap(namespace string, componentName string) (*corev1.ConfigMap, map[string]EnvVarMetadata, error) {
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

// ApplyEnvVarsMetadataConfigMap Save changes of environment-variables metadata to config-map
func (kubeutil *Kube) ApplyEnvVarsMetadataConfigMap(namespace string, envVarsMetadataConfigMap *corev1.ConfigMap, envVarsMetadataMap map[string]EnvVarMetadata) error {
	desiredEnvVarsMetadataConfigMap := envVarsMetadataConfigMap.DeepCopy()
	err := SetEnvVarsMetadataMapToConfigMap(desiredEnvVarsMetadataConfigMap, envVarsMetadataMap)
	if err != nil {
		return err
	}
	return kubeutil.ApplyConfigMap(namespace, envVarsMetadataConfigMap, desiredEnvVarsMetadataConfigMap)
}

// SetEnvVarsMetadataMapToConfigMap Set environment-variables metadata to config-map
func SetEnvVarsMetadataMapToConfigMap(configMap *corev1.ConfigMap, envVarsMetadataMap map[string]EnvVarMetadata) error {
	envVarsMetadata, err := json.Marshal(envVarsMetadataMap)
	if err != nil {
		return err
	}
	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}
	configMap.Data[envVarsMetadataPropertyName] = string(envVarsMetadata)
	return nil
}

// GetOrCreateEnvVarsConfigMapAndMetadataMap Get environment variables and its metadata config-maps
func (kubeutil *Kube) GetOrCreateEnvVarsConfigMapAndMetadataMap(namespace, appName, componentName string) (*corev1.ConfigMap, *corev1.ConfigMap, error) {
	envVarConfigMap, err := kubeutil.getOrCreateRadixConfigEnvVarsConfigMap(namespace, appName, componentName)
	if err != nil {
		err := fmt.Errorf("failed to create config-map for environment variables methadata: %w", err)
		return nil, nil, err
	}
	if envVarConfigMap.Data == nil {
		envVarConfigMap.Data = make(map[string]string)
	}
	envVarMetadataConfigMap, err := kubeutil.getOrCreateRadixConfigEnvVarsMetadataConfigMap(namespace, appName, componentName)
	if err != nil {
		err := fmt.Errorf("failed to create config-map for environment variables methadata: %w", err)
		return nil, nil, err
	}
	if envVarMetadataConfigMap.Data == nil {
		envVarMetadataConfigMap.Data = make(map[string]string)
	}
	return envVarConfigMap, envVarMetadataConfigMap, err
}

func (kubeutil *Kube) getOrCreateRadixConfigEnvVarsConfigMap(namespace, appName, componentName string) (*corev1.ConfigMap, error) {
	configMap, err := kubeutil.getRadixConfigEnvVarsConfigMap(namespace, GetEnvVarsConfigMapName(componentName))
	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, err
	}
	if configMap != nil {
		//HACK: fix a label radix-app, if it is wrong. Delete this code after radix-operator fix all env-var config-maps
		configMap, err = kubeutil.fixRadixAppNameLabel(namespace, appName, configMap)
		if err != nil {
			return nil, err
		}
		return configMap, nil
	}
	configMap = BuildRadixConfigEnvVarsConfigMap(appName, componentName)
	return kubeutil.CreateConfigMap(namespace, configMap)
}

func (kubeutil *Kube) getOrCreateRadixConfigEnvVarsMetadataConfigMap(namespace, appName, componentName string) (*corev1.ConfigMap, error) {
	configMap, err := kubeutil.getRadixConfigEnvVarsConfigMap(namespace, GetEnvVarsMetadataConfigMapName(componentName))
	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, err
	}
	if configMap != nil {
		//HACK: fix a label radix-app, if it is wrong. Delete this code after radix-operator fix all env-var config-maps
		configMap, err = kubeutil.fixRadixAppNameLabel(namespace, appName, configMap)
		if err != nil {
			return nil, err
		}
		return configMap, nil
	}
	configMap = BuildRadixConfigEnvVarsMetadataConfigMap(appName, componentName)
	return kubeutil.CreateConfigMap(namespace, configMap)
}

func (kubeutil *Kube) fixRadixAppNameLabel(namespace string, appName string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	if appNameLabel, ok := configMap.ObjectMeta.Labels[RadixAppLabel]; ok && !strings.EqualFold(appNameLabel, appName) {
		desiredConfigMap := configMap.DeepCopy()
		desiredConfigMap.ObjectMeta.Labels[RadixAppLabel] = appName
		err := kubeutil.ApplyConfigMap(namespace, configMap, desiredConfigMap)
		if err != nil {
			return nil, err
		}
		return desiredConfigMap, nil
	}
	return configMap, nil
}

// BuildRadixConfigEnvVarsConfigMap Build environment-variables config-map
func BuildRadixConfigEnvVarsConfigMap(appName, componentName string) *corev1.ConfigMap {
	return buildRadixConfigEnvVarsConfigMapForType(EnvVarsConfigMap, appName, componentName, GetEnvVarsConfigMapName(componentName))
}

// BuildRadixConfigEnvVarsMetadataConfigMap Build environment-variables metadata config-map
func BuildRadixConfigEnvVarsMetadataConfigMap(appName, componentName string) *corev1.ConfigMap {
	return buildRadixConfigEnvVarsConfigMapForType(EnvVarsMetadataConfigMap, appName, componentName, GetEnvVarsMetadataConfigMapName(componentName))
}

func buildRadixConfigEnvVarsConfigMapForType(configMapType RadixConfigMapType, appName, componentName, name string) *corev1.ConfigMap {
	labels := map[string]string{
		RadixAppLabel:           appName,
		RadixComponentLabel:     componentName,
		RadixConfigMapTypeLabel: string(configMapType),
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
		if k8sErrors.IsNotFound(err) {
			return nil, nil //due to k8s api returns pointer to an empty config-map
		}
		return nil, err
	}
	if configMap != nil && configMap.Data == nil {
		configMap.Data = make(map[string]string, 0)
	}
	return configMap, nil
}
