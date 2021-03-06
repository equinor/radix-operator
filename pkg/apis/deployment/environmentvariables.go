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

func (deploy *Deployment) getEnvironmentVariables(radixEnvVars v1.EnvVarsMap, radixSecrets []string, isPublic bool, ports []v1.ComponentPort, radixDeployName, namespace, currentEnvironment, appName, componentName string) []corev1.EnvVar {
	var environmentVariables = appendAppEnvVariables(radixDeployName, radixEnvVars)
	environmentVariables = deploy.appendDefaultVariables(currentEnvironment, environmentVariables, isPublic, namespace, appName, componentName, ports)

	// secrets
	if radixSecrets != nil {
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

func (deploy *Deployment) appendDefaultVariables(currentEnvironment string, environmentVariables []corev1.EnvVar, isPublic bool, namespace, appName, componentName string, ports []v1.ComponentPort) []corev1.EnvVar {
	clusterName, err := deploy.kubeutil.GetClusterName()
	if err != nil {
		return environmentVariables
	}

	dnsZone := os.Getenv(defaults.OperatorDNSZoneEnvironmentVariable)
	if dnsZone == "" {
		return nil
	}

	clusterType := os.Getenv(defaults.OperatorClusterTypeEnvironmentVariable)
	if clusterType != "" {
		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  defaults.RadixClusterTypeEnvironmentVariable,
			Value: clusterType,
		})
	}

	containerRegistry, err := deploy.kubeutil.GetContainerRegistry()
	if err != nil {
		return environmentVariables
	}

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.ContainerRegistryEnvironmentVariable,
		Value: containerRegistry,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.RadixDNSZoneEnvironmentVariable,
		Value: dnsZone,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.ClusternameEnvironmentVariable,
		Value: clusterName,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.EnvironmentnameEnvironmentVariable,
		Value: currentEnvironment,
	})

	if isPublic {
		canonicalHostName := getHostName(componentName, namespace, clusterName, dnsZone)
		publicHostName := ""

		if isActiveCluster(clusterName) {
			publicHostName = getActiveClusterHostName(componentName, namespace)
		} else {
			publicHostName = canonicalHostName
		}

		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  defaults.PublicEndpointEnvironmentVariable,
			Value: publicHostName,
		})
		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  defaults.CanonicalEndpointEnvironmentVariable,
			Value: canonicalHostName,
		})
	}

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.RadixAppEnvironmentVariable,
		Value: appName,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.RadixComponentEnvironmentVariable,
		Value: componentName,
	})

	if len(ports) > 0 {
		portNumbers, portNames := getPortNumbersAndNamesString(ports)
		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  defaults.RadixPortsEnvironmentVariable,
			Value: portNumbers,
		})

		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  defaults.RadixPortNamesEnvironmentVariable,
			Value: portNames,
		})
	}

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.RadixCommitHashEnvironmentVariable,
		Value: deploy.radixDeployment.Labels[kube.RadixCommitLabel],
	})

	return environmentVariables
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
