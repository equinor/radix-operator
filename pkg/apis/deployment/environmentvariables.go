package deployment

import (
	"fmt"
	"os"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

func (deploy *Deployment) getEnvironmentVariables(radixEnvVars v1.EnvVarsMap, radixSecrets []string, isPublic bool, ports []v1.ComponentPort, radixDeployName, namespace, currentEnvironment, appName, componentName string) []corev1.EnvVar {
	var environmentVariables []corev1.EnvVar
	if radixEnvVars != nil {
		// environmentVariables
		for key, value := range radixEnvVars {
			envVar := corev1.EnvVar{
				Name:  key,
				Value: value,
			}
			environmentVariables = append(environmentVariables, envVar)
		}
	} else {
		log.Infof("No environment variable is set for this RadixDeployment %s", radixDeployName)
	}

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
		log.Infof("No secret is set for this RadixDeployment %s", radixDeployName)
	}

	return environmentVariables
}

func (deploy *Deployment) appendDefaultVariables(currentEnvironment string, environmentVariables []corev1.EnvVar, isPublic bool, namespace, appName, componentName string, ports []v1.ComponentPort) []corev1.EnvVar {
	clusterName, err := deploy.kubeutil.GetClusterName()
	if err != nil {
		return environmentVariables
	}

	dnsZone := os.Getenv(OperatorDNSZoneEnvironmentVariable)
	if dnsZone == "" {
		return nil
	}

	containerRegistry, err := deploy.kubeutil.GetContainerRegistry()
	if err != nil {
		return environmentVariables
	}

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  ContainerRegistryEnvironmentVariable,
		Value: containerRegistry,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  RadixDNSZoneEnvironmentVariable,
		Value: dnsZone,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  ClusternameEnvironmentVariable,
		Value: clusterName,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  EnvironmentnameEnvironmentVariable,
		Value: currentEnvironment,
	})

	if isPublic {
		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  PublicEndpointEnvironmentVariable,
			Value: getHostName(componentName, namespace, clusterName, dnsZone),
		})
	}

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  RadixAppEnvironmentVariable,
		Value: appName,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  RadixComponentEnvironmentVariable,
		Value: componentName,
	})

	if len(ports) > 0 {
		portNumbers, portNames := getPortNumbersAndNamesString(ports)
		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  RadixPortsEnvironmentVariable,
			Value: portNumbers,
		})

		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  RadixPortNamesEnvironmentVariable,
			Value: portNames,
		})
	}

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
