package deployment

import (
	"fmt"
	"os"
	"sort"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

type envVariablesSourceDecorator interface {
	getClusterName() (string, error)
	GetContainerRegistry() (string, error)
}

type radixApplicationEnvVariablesSourceDecorator struct{}
type radixOperatorEnvVariablesSourceDecorator struct {
	kubeutil *kube.Kube
}

func (envVariablesSource *radixApplicationEnvVariablesSourceDecorator) getClusterName() (string, error) {
	return os.Getenv(defaults.ClusternameEnvironmentVariable), nil
}

func (envVariablesSource *radixApplicationEnvVariablesSourceDecorator) GetContainerRegistry() (string, error) {
	return os.Getenv(defaults.ContainerRegistryEnvironmentVariable), nil
}

func (envVariablesSource *radixOperatorEnvVariablesSourceDecorator) getClusterName() (string, error) {
	clusterName, err := envVariablesSource.kubeutil.GetClusterName()
	if err != nil {
		return "", fmt.Errorf("failed to get cluster name from ConfigMap: %v", err)
	}
	return clusterName, nil
}

func (envVariablesSource *radixOperatorEnvVariablesSourceDecorator) GetContainerRegistry() (string, error) {
	containerRegistry, err := envVariablesSource.kubeutil.GetContainerRegistry()
	if err != nil {
		return "", fmt.Errorf("failed to get container registry from ConfigMap: %v", err)
	}
	return containerRegistry, nil
}

//getEnvironmentVariablesForRadixOperator Provides RADIX_* environment variables for Radix operator.
//It requires service account having access to config map in default namespace.
func getEnvironmentVariablesForRadixOperator(appName string, kubeutil *kube.Kube, radixDeployment *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent) []corev1.EnvVar {
	return getEnvironmentVariablesFrom(appName, &radixOperatorEnvVariablesSourceDecorator{kubeutil: kubeutil}, radixDeployment, deployComponent)
}

//GetEnvironmentVariablesFrom Provides RADIX_* environment variables for Radix applications.
func GetEnvironmentVariablesFrom(appName string, radixDeployment *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent) []corev1.EnvVar {
	return getEnvironmentVariablesFrom(appName, &radixApplicationEnvVariablesSourceDecorator{}, radixDeployment, deployComponent)
}

func getEnvironmentVariablesFrom(appName string, envVariablesSource envVariablesSourceDecorator, radixDeployment *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent) []corev1.EnvVar {
	var vars = getEnvironmentVariables(
		appName,
		envVariablesSource,
		radixDeployment,
		deployComponent.GetName(),
		deployComponent.GetEnvironmentVariables(),
		deployComponent.GetSecrets(),
		deployComponent.GetPublicPort() != "" || deployComponent.IsPublic(), // For backwards compatibility
		deployComponent.GetPorts(),
	)
	return vars
}

func getEnvironmentVariables(appName string, environmentVariablesSource envVariablesSourceDecorator, radixDeployment *v1.RadixDeployment, componentName string, radixEnvVars *v1.EnvVarsMap, radixSecrets []string, isPublic bool, ports []v1.ComponentPort) []corev1.EnvVar {
	var (
		radixDeployName       = radixDeployment.Name
		namespace             = radixDeployment.Namespace
		currentEnvironment    = radixDeployment.Spec.Environment
		radixDeploymentLabels = radixDeployment.Labels
	)
	var environmentVariables = appendAppEnvVariables(radixDeployName, *radixEnvVars)
	environmentVariables = appendDefaultVariables(environmentVariablesSource, currentEnvironment, environmentVariables, isPublic, namespace, appName, componentName, ports, radixDeploymentLabels)
	if radixSecrets != nil && len(radixSecrets) > 0 {
		for _, v := range radixSecrets {
			componentSecretName := utils.GetComponentSecretName(componentName)
			secretKeySelector := corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: componentSecretName,
				},
				Key: v,
			}
			envVarSource := corev1.EnvVarSource{
				SecretKeyRef: &secretKeySelector,
			}
			secretEnvVar := corev1.EnvVar{
				Name:      v,
				ValueFrom: &envVarSource,
			}
			environmentVariables = append(environmentVariables, secretEnvVar)
		}
	} else {
		log.Debugf("No secret is set for this RadixDeployment %s", radixDeployName)
	}
	return environmentVariables
}

func appendAppEnvVariables(radixDeployName string, radixEnvVars v1.EnvVarsMap) []corev1.EnvVar {
	var environmentVariables []corev1.EnvVar
	if radixEnvVars != nil {
		// map is not sorted, which lead to random order of env variable in deployment
		// during stop/start/restart of a single component this lead to restart of several other components
		keys := []string{}
		for k := range radixEnvVars {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// environmentVariables
		for _, key := range keys {
			value := radixEnvVars[key]
			envVar := corev1.EnvVar{
				Name:  key,
				Value: value,
			}
			environmentVariables = append(environmentVariables, envVar)
		}
	} else {
		log.Debugf("No environment variable is set for this RadixDeployment %s", radixDeployName)
	}
	return environmentVariables
}

func appendDefaultVariables(envVariablesSource envVariablesSourceDecorator, currentEnvironment string, environmentVariables []corev1.EnvVar, isPublic bool, namespace, appName, componentName string, ports []v1.ComponentPort, radixDeploymentLabels map[string]string) []corev1.EnvVar {
	envVarSet := utils.NewEnvironmentVariablesSet().Init(environmentVariables)
	dnsZone := os.Getenv(defaults.OperatorDNSZoneEnvironmentVariable)
	if dnsZone == "" {
		log.Errorf("Not set environment variable %s", defaults.OperatorDNSZoneEnvironmentVariable)
		return nil
	}

	clusterType := os.Getenv(defaults.OperatorClusterTypeEnvironmentVariable)
	if clusterType != "" {
		envVarSet.Add(defaults.RadixClusterTypeEnvironmentVariable, clusterType)
	} else {
		log.Debugf("Not set environment variable %s", defaults.RadixClusterTypeEnvironmentVariable)
	}

	containerRegistry, err := envVariablesSource.GetContainerRegistry()
	if err != nil {
		log.Error(err)
		return environmentVariables
	}

	envVarSet.Add(defaults.ContainerRegistryEnvironmentVariable, containerRegistry)
	envVarSet.Add(defaults.RadixDNSZoneEnvironmentVariable, dnsZone)
	clusterName, err := envVariablesSource.getClusterName()
	if err != nil {
		log.Error(err)
		return environmentVariables
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
