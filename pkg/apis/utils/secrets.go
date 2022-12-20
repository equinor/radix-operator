package utils

import (
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
)

// GetComponentSecretName Gets unique name of the component secret
func GetComponentSecretName(componentName string) string {
	// include a hash so that users cannot get access to a secret they should not get
	// by naming component the same as secret object
	hash := strings.ToLower(RandStringStrSeed(8, componentName))
	return strings.ToLower(fmt.Sprintf("%s-%s", componentName, hash))
}

// GetComponentClientCertificateSecretName Gets name of the component secret that holds the ca.crt public key for client certificate authentication
func GetComponentClientCertificateSecretName(componentame string) string {
	return strings.ToLower(fmt.Sprintf("%s-clientcertca", componentame))
}

// GetAuxiliaryComponentSecretName Get secret name for AuxiliaryComponent
func GetAuxiliaryComponentSecretName(componentName string, suffix string) string {
	return GetComponentSecretName(GetAuxiliaryComponentDeploymentName(componentName, suffix))
}

// GetEnvVarsFromAzureKeyVaultSecretRefs Get EnvVars from AzureKeyVaultSecretRefs
func GetEnvVarsFromAzureKeyVaultSecretRefs(radixDeploymentName, componentName string, secretRefs radixv1.RadixSecretRefs) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	for _, azureKeyVault := range secretRefs.AzureKeyVaults {
		for _, keyVaultItem := range azureKeyVault.Items {
			if len(keyVaultItem.EnvVar) == 0 {
				continue //Do not add cert,secret or key as environment variable - it will exist only as s file
			}
			kubeSecretType := kube.GetSecretTypeForRadixAzureKeyVault(keyVaultItem.K8sSecretType)
			secretName := kube.GetAzureKeyVaultSecretRefSecretName(componentName, radixDeploymentName, azureKeyVault.Name, kubeSecretType)
			secretEnvVar := createEnvVarWithSecretRef(secretName, keyVaultItem.EnvVar)
			envVars = append(envVars, secretEnvVar)
		}
	}
	return envVars
}

// GetEnvVarsFromSecrets Get EnvVars from secret names
func GetEnvVarsFromSecrets(componentName string, secretNames []string) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	componentSecretName := GetComponentSecretName(componentName)
	for _, secretName := range secretNames {
		secretEnvVar := createEnvVarWithSecretRef(componentSecretName, secretName)
		envVars = append(envVars, secretEnvVar)
	}
	return envVars
}

// GetServiceAccountSecretNamePrefix Gets name prefix of the service account secret
func GetServiceAccountSecretNamePrefix(serviceAccountName string) string {
	return fmt.Sprintf("%s-token", serviceAccountName)
}

func createEnvVarWithSecretRef(secretName, envVarName string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: envVarName,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretName,
				},
				Key: envVarName,
			},
		},
	}
}
