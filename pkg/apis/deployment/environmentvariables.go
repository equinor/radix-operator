package deployment

import (
	"fmt"
	"os"
	"sort"
	"strings"

	radixmaps "github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

type environmentVariablesSourceDecorator interface {
	getClusterName() (string, error)
	getContainerRegistry() (string, error)
	getDnsZone() (string, error)
	getClusterType() (string, error)
}

type radixApplicationEnvironmentVariablesSourceDecorator struct{}
type radixOperatorEnvironmentVariablesSourceDecorator struct {
	kubeutil *kube.Kube
}

func (envVarsSource *radixApplicationEnvironmentVariablesSourceDecorator) getClusterName() (string, error) {
	return getEnvVar(defaults.ClusternameEnvironmentVariable)
}

func (envVarsSource *radixApplicationEnvironmentVariablesSourceDecorator) getContainerRegistry() (string, error) {
	return getEnvVar(defaults.ContainerRegistryEnvironmentVariable)
}

func (envVarsSource *radixApplicationEnvironmentVariablesSourceDecorator) getDnsZone() (string, error) {
	return getEnvVar(defaults.RadixDNSZoneEnvironmentVariable)
}

func (envVarsSource *radixApplicationEnvironmentVariablesSourceDecorator) getClusterType() (string, error) {
	return getEnvVar(defaults.RadixClusterTypeEnvironmentVariable)
}

func (envVarsSource *radixOperatorEnvironmentVariablesSourceDecorator) getClusterName() (string, error) {
	clusterName, err := envVarsSource.kubeutil.GetClusterName()
	if err != nil {
		return "", fmt.Errorf("failed to get cluster name from ConfigMap: %v", err)
	}
	return clusterName, nil
}

func (envVarsSource *radixOperatorEnvironmentVariablesSourceDecorator) getContainerRegistry() (string, error) {
	containerRegistry, err := envVarsSource.kubeutil.GetContainerRegistry()
	if err != nil {
		return "", fmt.Errorf("failed to get container registry from ConfigMap: %v", err)
	}
	return containerRegistry, nil
}

func (envVarsSource *radixOperatorEnvironmentVariablesSourceDecorator) getDnsZone() (string, error) {
	return getEnvVar(defaults.OperatorDNSZoneEnvironmentVariable)
}

func (envVarsSource *radixOperatorEnvironmentVariablesSourceDecorator) getClusterType() (string, error) {
	return getEnvVar(defaults.OperatorClusterTypeEnvironmentVariable)
}

//getEnvironmentVariablesForRadixOperator Provides RADIX_* environment variables for Radix operator.
//It requires service account having access to config map in default namespace.
func getEnvironmentVariablesForRadixOperator(appName string, kubeutil *kube.Kube, radixDeployment *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent, envVarConfigMap *corev1.ConfigMap, envVarMetadataMap map[string]v1.EnvVarsMetadata) []corev1.EnvVar {
	return getEnvironmentVariablesFrom(appName, &radixOperatorEnvironmentVariablesSourceDecorator{kubeutil: kubeutil}, radixDeployment, deployComponent, envVarConfigMap, envVarMetadataMap)
}

//GetEnvironmentVariablesFrom Provides RADIX_* environment variables for Radix applications.
func GetEnvironmentVariablesFrom(appName string, radixDeployment *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent, envVarConfigMap *corev1.ConfigMap, envVarMetadataMap map[string]v1.EnvVarsMetadata) []corev1.EnvVar {
	return getEnvironmentVariablesFrom(appName, &radixApplicationEnvironmentVariablesSourceDecorator{}, radixDeployment, deployComponent, envVarConfigMap, envVarMetadataMap)
}

func getEnvironmentVariablesFrom(appName string, envVarsSource environmentVariablesSourceDecorator, radixDeployment *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent, radixConfigEnvVarConfigMap *corev1.ConfigMap, envVarMetadataMap map[string]v1.EnvVarsMetadata) []corev1.EnvVar {
	var vars = getEnvironmentVariables(
		appName,
		envVarsSource,
		radixDeployment,
		deployComponent.GetName(),
		deployComponent.GetEnvironmentVariables(),
		deployComponent.GetSecrets(),
		deployComponent.GetPublicPort() != "" || deployComponent.IsPublic(), // For backwards compatibility
		deployComponent.GetPorts(),
		radixConfigEnvVarConfigMap,
		envVarMetadataMap,
	)
	return vars
}

func getEnvironmentVariables(appName string, envVarsSource environmentVariablesSourceDecorator, radixDeployment *v1.RadixDeployment, componentName string, radixEnvVars v1.EnvVarsMap, radixSecretNames []string, isPublic bool, ports []v1.ComponentPort, envVarConfigMap *corev1.ConfigMap, envVarMetadataMap map[string]v1.EnvVarsMetadata) []corev1.EnvVar {
	var (
		namespace             = radixDeployment.Namespace
		currentEnvironment    = radixDeployment.Spec.Environment
		radixDeploymentLabels = radixDeployment.Labels
	)
	var envVars = getEnvVarsFromRadixConfig(radixEnvVars, envVarConfigMap, envVarMetadataMap)
	envVars = appendDefaultEnvVars(envVars, envVarsSource, currentEnvironment, isPublic, namespace, appName, componentName, ports, radixDeploymentLabels)
	envVars = appendEnvVarsFromSecrets(envVars, radixSecretNames, utils.GetComponentSecretName(componentName))
	return envVars
}

func appendEnvVarsFromSecrets(envVars []corev1.EnvVar, radixSecretNames []string, componentSecretName string) []corev1.EnvVar {
	if radixSecretNames != nil && len(radixSecretNames) > 0 {
		for _, secretName := range radixSecretNames {
			secretEnvVar := createEnvVarWithSecretRef(componentSecretName, secretName)
			envVars = append(envVars, secretEnvVar)
		}
	} else {
		log.Debugf("No secret is set for this RadixDeployment")
	}
	return envVars
}

func getEnvVarsFromRadixConfig(envVarsMap v1.EnvVarsMap, envVarConfigMap *corev1.ConfigMap, envVarMetadataMap map[string]v1.EnvVarsMetadata) []corev1.EnvVar {
	if envVarsMap == nil {
		log.Debugf("No environment variable is set for this RadixDeployment")
		return []corev1.EnvVar{}
	}

	envVarConfigMapName := envVarConfigMap.GetName()
	// map is not sorted, which lead to random order of env variable in deployment
	// during stop/start/restart of a single component this lead to restart of several other components
	envVarNames := getEnvVarNamesSorted(envVarsMap)
	var envVars []corev1.EnvVar
	for _, envVarName := range envVarNames {
		envVars = append(envVars, createEnvVarWithConfigMapRef(envVarConfigMapName, envVarName))
		envVarValue := envVarsMap[envVarName]

		existingValue, foundExistingValue := envVarConfigMap.Data[envVarName]
		metadata, foundMetadata := envVarMetadataMap[envVarName]

		if !foundExistingValue || !foundMetadata {
			envVarConfigMap.Data[envVarName] = envVarValue
			if foundMetadata {
				delete(envVarMetadataMap, envVarName)
			}
			continue
		}

		if !foundMetadata {
			continue
		}

		if strings.EqualFold(metadata.RadixConfigValue, existingValue) {
			envVarConfigMap.Data[envVarName] = envVarValue
			delete(envVarMetadataMap, envVarName)
		} else {
			metadata.RadixConfigValue = envVarValue
			envVarMetadataMap[envVarName] = metadata
		}
	}
	return envVars
}

func removeFromConfigMapEnvVarsNotExistingInRadixconfig(envVarsMap v1.EnvVarsMap, envVarConfigMap *corev1.ConfigMap) {
	envVarCmRefs := radixmaps.GetKeysFromStringMap(envVarConfigMap.Data)
	for _, envVarName := range envVarCmRefs {
		if _, ok := envVarsMap[envVarName]; !ok {
			delete(envVarConfigMap.Data, envVarName)
		}
	}
}

func getEnvVarNamesSorted(envVarsMap v1.EnvVarsMap) []string {
	var envVarNames []string
	for k := range envVarsMap {
		envVarNames = append(envVarNames, k)
	}
	sort.Strings(envVarNames)
	return envVarNames
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

func appendDefaultEnvVars(envVars []corev1.EnvVar, envVarsSource environmentVariablesSourceDecorator, currentEnvironment string, isPublic bool, namespace, appName, componentName string, ports []v1.ComponentPort, radixDeploymentLabels map[string]string) []corev1.EnvVar {
	envVarSet := utils.NewEnvironmentVariablesSet().Init(envVars)
	dnsZone, err := envVarsSource.getDnsZone()
	if err != nil {
		log.Error(err)
		return envVarSet.Items()
	}

	clusterType, err := envVarsSource.getClusterType()
	if err == nil {
		envVarSet.Add(defaults.RadixClusterTypeEnvironmentVariable, clusterType)
	} else {
		log.Debug(err)
	}
	containerRegistry, err := envVarsSource.getContainerRegistry()
	if err != nil {
		log.Error(err)
		return envVarSet.Items()
	}
	envVarSet.Add(defaults.ContainerRegistryEnvironmentVariable, containerRegistry)
	envVarSet.Add(defaults.RadixDNSZoneEnvironmentVariable, dnsZone)
	clusterName, err := envVarsSource.getClusterName()
	if err != nil {
		log.Error(err)
		return envVarSet.Items()
	}
	envVarSet.Add(defaults.ClusternameEnvironmentVariable, clusterName)
	envVarSet.Add(defaults.EnvironmentnameEnvironmentVariable, currentEnvironment)
	if isPublic {
		canonicalHostName := getHostName(componentName, namespace, clusterName, dnsZone)
		publicHostName := ""
		if isActiveCluster(clusterName) {
			publicHostName = getActiveClusterHostName(componentName, namespace)
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
	envVarSet.Add(defaults.RadixCommitHashEnvironmentVariable, radixDeploymentLabels[kube.RadixCommitLabel])

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
