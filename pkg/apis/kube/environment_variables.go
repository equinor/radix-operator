package kube

import (
	"encoding/json"
	"fmt"
	radixmaps "github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"sort"
	"strings"
)

const (
	envVarsPrefix               = "env-vars"          //Environment variables
	envVarsMetadataPrefix       = "env-vars-metadata" //Metadata for environment variables
	envVarsMetadataPropertyName = "metadata"          //Metadata property for environment variables in config-map
	radixEnvVariablePrefix      = "RADIX_"
)

//RadixConfigMapType Purpose of ConfigMap
type RadixConfigMapType string

const (
	//EnvVarsConfigMap ConfigMap contains environment variables
	EnvVarsConfigMap RadixConfigMapType = "env-vars"
	//EnvVarsMetadataConfigMap ConfigMap contains environment variables metadata
	EnvVarsMetadataConfigMap RadixConfigMapType = "env-vars-metadata"
)

//EnvVarMetadata Metadata for environment variables
type EnvVarMetadata struct {
	RadixConfigValue string
}

//GetEnvVarsConfigMapName Get config-map name for environment variables
func GetEnvVarsConfigMapName(componentName string) string {
	return fmt.Sprintf("%s-%s", envVarsPrefix, componentName)
}

//GetEnvVarsMetadataConfigMapName Get config-map name for environment variables metadata
func GetEnvVarsMetadataConfigMapName(componentName string) string {
	return fmt.Sprintf("%s-%s", envVarsMetadataPrefix, componentName)
}

//GetEnvVarsMetadataFromConfigMap Get environment-variables metadata from config-map
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

//GetEnvVarsConfigMapAndMetadataMap Get environment-variables config-map, environment-variables metadata config-map and metadata map from it
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

//GetEnvVarsMetadataConfigMapAndMap Get environment-variables metadata config-map and map from it
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

//ApplyEnvVarsMetadataConfigMap Save changes of environment-variables metadata to config-map
func (kubeutil *Kube) ApplyEnvVarsMetadataConfigMap(namespace string, currentEnvVarsMetadataConfigMap *corev1.ConfigMap, envVarsMetadataMap map[string]EnvVarMetadata) error {
	desiredEnvVarsMetadataConfigMap := currentEnvVarsMetadataConfigMap.DeepCopy()
	err := SetEnvVarsMetadataMapToConfigMap(desiredEnvVarsMetadataConfigMap, envVarsMetadataMap)
	if err != nil {
		return err
	}
	return kubeutil.ApplyConfigMap(namespace, currentEnvVarsMetadataConfigMap, desiredEnvVarsMetadataConfigMap)
}

//SetEnvVarsMetadataMapToConfigMap Set environment-variables metadata to config-map
func SetEnvVarsMetadataMapToConfigMap(configMap *corev1.ConfigMap, envVarsMetadataMap map[string]EnvVarMetadata) error {
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
	if envVarConfigMap.Data == nil {
		envVarConfigMap.Data = make(map[string]string, 0)
	}
	envVarMetadataConfigMap, err := kubeutil.getOrCreateRadixConfigEnvVarsMetadataConfigMap(namespace, appName, componentName)
	if err != nil {
		err := fmt.Errorf("failed to create config-map for environment variables methadata: %v", err)
		log.Error(err)
		return nil, nil, err
	}
	if envVarMetadataConfigMap.Data == nil {
		envVarMetadataConfigMap.Data = make(map[string]string, 0)
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
	return buildRadixConfigEnvVarsConfigMapForType(EnvVarsConfigMap, appName, componentName, GetEnvVarsConfigMapName(componentName))
}

//BuildRadixConfigEnvVarsMetadataConfigMap Build environment-variables metadata config-map
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
		statusError := err.(*k8sErrors.StatusError)
		if statusError == nil || !k8sErrors.IsNotFound(statusError) {
			return nil, err
		}
	}
	if configMap != nil && configMap.Data == nil {
		configMap.Data = make(map[string]string, 0)
	}
	return configMap, nil
}

//IsRadixEnvVar Indicates if environment-variable is created by Radix
func IsRadixEnvVar(envVarName string) bool {
	return strings.HasPrefix(strings.TrimSpace(envVarName), radixEnvVariablePrefix)
}

//GetEnvironmentVariablesForRadixOperator Provides RADIX_* environment variables for Radix operator.
//It requires service account having access to config map in default namespace.
func (kubeutil *Kube) GetEnvironmentVariablesForRadixOperator(appName string, radixDeployment *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent) ([]corev1.EnvVar, error) {
	return kubeutil.getEnvironmentVariablesFrom(appName, &RadixOperatorEnvironmentVariablesSourceDecorator{kubeutil: kubeutil}, radixDeployment, deployComponent)
}

//GetEnvironmentVariables Provides environment variables for Radix application.
func (kubeutil *Kube) GetEnvironmentVariables(appName string, radixDeployment *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent) ([]corev1.EnvVar, error) {
	return kubeutil.GetEnvironmentVariablesFrom(appName, radixDeployment, deployComponent)
}

//GetEnvironmentVariablesFrom Provides environment variables for Radix application by given config-maps.
func (kubeutil *Kube) GetEnvironmentVariablesFrom(appName string, radixDeployment *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent) ([]corev1.EnvVar, error) {
	return kubeutil.getEnvironmentVariablesFrom(appName, &RadixApplicationEnvironmentVariablesSourceDecorator{}, radixDeployment, deployComponent)
}

func (kubeutil *Kube) getEnvironmentVariablesFrom(appName string, envVarsSource EnvironmentVariablesSourceDecorator, radixDeployment *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent) ([]corev1.EnvVar, error) {
	return kubeutil.getEnvironmentVariables(appName, envVarsSource, radixDeployment, deployComponent.GetName(), deployComponent.GetEnvironmentVariables(), deployComponent.GetSecrets(), deployComponent.GetPublicPort() != "" || deployComponent.IsPublic(), deployComponent.GetPorts())
}

func (kubeutil *Kube) getEnvironmentVariables(appName string, envVarsSource EnvironmentVariablesSourceDecorator, radixDeployment *v1.RadixDeployment, componentName string, radixConfigEnvVars v1.EnvVarsMap, radixSecretNames []string, isPublic bool, ports []v1.ComponentPort) ([]corev1.EnvVar, error) {
	var (
		namespace             = radixDeployment.Namespace
		currentEnvironment    = radixDeployment.Spec.Environment
		radixDeploymentLabels = radixDeployment.Labels
	)
	envVarsConfigMap, _, _, err := kubeutil.GetEnvVarsConfigMapAndMetadataMap(radixDeployment.GetNamespace(), componentName)
	if err != nil {
		return nil, err
	}
	var envVars = getEnvVarsFromRadixConfig(envVarsConfigMap)
	envVars = appendDefaultEnvVars(envVars, envVarsSource, currentEnvironment, isPublic, namespace, appName, componentName, ports, radixDeploymentLabels)
	envVars = appendEnvVarsFromSecrets(envVars, radixSecretNames, utils.GetComponentSecretName(componentName))
	return envVars, nil
}

func appendEnvVarsFromSecrets(envVars []corev1.EnvVar, radixSecretNames []string, componentSecretName string) []corev1.EnvVar {
	if len(radixSecretNames) > 0 {
		for _, secretName := range radixSecretNames {
			secretEnvVar := createEnvVarWithSecretRef(componentSecretName, secretName)
			envVars = append(envVars, secretEnvVar)
		}
	} else {
		log.Debugf("No secret is set for this RadixDeployment")
	}
	return envVars
}

//BuildEnvVarsFromRadixConfig Build environment-variable config-maps, based on radixconfig environment variables
func BuildEnvVarsFromRadixConfig(radixConfigEnvVars v1.EnvVarsMap, envVarConfigMap *corev1.ConfigMap, envVarMetadataMap map[string]EnvVarMetadata) {
	if radixConfigEnvVars == nil {
		log.Debugf("No environment variables are set for this RadixDeployment in radixconfig")
		return
	}

	for envVarName, radixConfigEnvVarValue := range radixConfigEnvVars {
		envVarConfigMapValue, foundValueInEnvVarConfigMap := envVarConfigMap.Data[envVarName]
		envVarMetadata, foundEnvVarMetadata := envVarMetadataMap[envVarName]

		if !foundValueInEnvVarConfigMap || !foundEnvVarMetadata { //no such env-var, created or changed in Radix console
			envVarConfigMap.Data[envVarName] = radixConfigEnvVarValue //use env-var from radixconfig
			if foundEnvVarMetadata {                                  //exists metadata without config-map env-var
				delete(envVarMetadataMap, envVarName) //remove this orphaned metadata
			}
			continue
		}

		if !foundEnvVarMetadata { //no metadata to update
			continue
		}

		if strings.EqualFold(envVarConfigMapValue, envVarMetadata.RadixConfigValue) { //config-map env-var is the same as in metadata
			delete(envVarMetadataMap, envVarName)                     //remove metadata (it is not necessary anymore)
			envVarConfigMap.Data[envVarName] = radixConfigEnvVarValue //use env-var from radixconfig
			continue
		}

		//save radixconfig env-var value to metadata
		envVarMetadata.RadixConfigValue = radixConfigEnvVarValue
		envVarMetadataMap[envVarName] = envVarMetadata
		log.Debugf("RadixConfig environment variable '%s' has been set or changed in Radix console", envVarName)
	}
	removeFromConfigMapEnvVarsNotExistingInRadixconfig(radixConfigEnvVars, envVarConfigMap)
}

func getEnvVarsFromRadixConfig(envVarConfigMap *corev1.ConfigMap) []corev1.EnvVar {
	envVarConfigMapName := envVarConfigMap.GetName()
	// map is not sorted, which lead to random order of env variable in deployment
	// during stop/start/restart of a single component this lead to restart of several other components
	envVarNames := getMapKeysSorted(envVarConfigMap.Data)
	var resultEnvVars []corev1.EnvVar
	for _, envVarName := range envVarNames {
		resultEnvVars = append(resultEnvVars, createEnvVarWithConfigMapRef(envVarConfigMapName, envVarName))
	}
	return resultEnvVars
}

func removeFromConfigMapEnvVarsNotExistingInRadixconfig(envVarsMap v1.EnvVarsMap, envVarConfigMap *corev1.ConfigMap) {
	envVarCmRefs := radixmaps.GetKeysFromStringMap(envVarConfigMap.Data)
	for _, envVarName := range envVarCmRefs {
		if _, ok := envVarsMap[envVarName]; !ok {
			delete(envVarConfigMap.Data, envVarName)
		}
	}
}

func getMapKeysSorted(stringMap map[string]string) []string {
	keys := radixmaps.GetKeysFromStringMap(stringMap)
	sort.Strings(keys)
	return keys
}

func createEnvVarWithSecretRef(componentSecretName string, secretName string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: secretName,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: componentSecretName,
				},
				Key: secretName,
			},
		},
	}
}

func createEnvVarWithConfigMapRef(envVarConfigMapName string, envVarName string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: envVarName,
		ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: envVarConfigMapName,
				},
				Key: envVarName,
			},
		},
	}
}

func appendDefaultEnvVars(envVars []corev1.EnvVar, envVarsSource EnvironmentVariablesSourceDecorator, currentEnvironment string, isPublic bool, namespace, appName, componentName string, ports []v1.ComponentPort, radixDeploymentLabels map[string]string) []corev1.EnvVar {
	envVarSet := utils.NewEnvironmentVariablesSet().Init(envVars)
	dnsZone, err := envVarsSource.GetDnsZone()
	if err != nil {
		log.Error(err)
		return envVarSet.Items()
	}

	clusterType, err := envVarsSource.GetClusterType()
	if err == nil {
		envVarSet.Add(defaults.RadixClusterTypeEnvironmentVariable, clusterType)
	} else {
		log.Debug(err)
	}
	containerRegistry, err := envVarsSource.GetContainerRegistry()
	if err != nil {
		log.Error(err)
		return envVarSet.Items()
	}
	envVarSet.Add(defaults.ContainerRegistryEnvironmentVariable, containerRegistry)
	envVarSet.Add(defaults.RadixDNSZoneEnvironmentVariable, dnsZone)
	clusterName, err := envVarsSource.GetClusterName()
	if err != nil {
		log.Error(err)
		return envVarSet.Items()
	}
	envVarSet.Add(defaults.ClusternameEnvironmentVariable, clusterName)
	envVarSet.Add(defaults.EnvironmentnameEnvironmentVariable, currentEnvironment)
	if isPublic {
		canonicalHostName := utils.GetHostName(componentName, namespace, clusterName, dnsZone)
		publicHostName := ""
		if utils.IsActiveCluster(clusterName) {
			publicHostName = utils.GetActiveClusterHostName(componentName, namespace)
		} else {
			publicHostName = canonicalHostName
		}
		envVarSet.Add(defaults.PublicEndpointEnvironmentVariable, publicHostName)
		envVarSet.Add(defaults.CanonicalEndpointEnvironmentVariable, canonicalHostName)
	}
	envVarSet.Add(defaults.RadixAppEnvironmentVariable, appName)
	envVarSet.Add(defaults.RadixComponentEnvironmentVariable, componentName)
	if len(ports) > 0 {
		portNumbers, portNames := getPortNumbersAndNamesString(ports)
		envVarSet.Add(defaults.RadixPortsEnvironmentVariable, portNumbers)
		envVarSet.Add(defaults.RadixPortNamesEnvironmentVariable, portNames)
	} else {
		log.Debugf("No ports defined for the component")
	}
	envVarSet.Add(defaults.RadixCommitHashEnvironmentVariable, radixDeploymentLabels[RadixCommitLabel])

	return envVarSet.Items()
}

func getPortNumbersAndNamesString(ports []v1.ComponentPort) (string, string) {
	portNumbers := "("
	portNames := "("
	portsSize := len(ports)
	for i, portObj := range ports {
		if i < portsSize-1 {
			portNumbers += fmt.Sprint(portObj.Port) + " "
			portNames += fmt.Sprint(portObj.Name) + " "
		} else {
			portNumbers += fmt.Sprint(portObj.Port) + ")"
			portNames += fmt.Sprint(portObj.Name) + ")"
		}
	}
	return portNumbers, portNames
}

func getEnvVar(name string) (string, error) {
	envVar := os.Getenv(name)
	if len(envVar) > 0 {
		return envVar, nil
	}
	return "", fmt.Errorf("not set environment variable %s", name)
}
