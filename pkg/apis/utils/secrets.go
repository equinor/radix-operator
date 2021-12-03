package utils

import (
	"fmt"
	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"strings"
)

// GetComponentSecretName Gets unique name of the component secret
func GetComponentSecretName(componentName string) string {
	// include a hash so that users cannot get access to a secret they should not ,
	// by naming component the same as secret object
	hash := strings.ToLower(RandStringStrSeed(8, componentName))
	return fmt.Sprintf("%s-%s", componentName, hash)
}

// GetComponentAzureKeyVaultCredentialsSecretName Gets unique name of the component key-vault secret
func GetComponentAzureKeyVaultCredentialsSecretName(componentName, secretRefType, secretRefName string) string {
	// include a hash so that users cannot get access to a key-vault secret they should not ,
	// by naming component the same as key-vault secret object
	hash := strings.ToLower(RandStringStrSeed(8, componentName))
	return fmt.Sprintf("%s-%s-cred-%s-%s", componentName, secretRefType, secretRefName, hash)
}

// GetComponentSecretProviderClassName Gets unique name of the component secret storage class
func GetComponentSecretProviderClassName(componentName, secretRefType, secretRefName string) string {
	// include a hash so that users cannot get access to a secret-ref they should not ,
	// by naming component the same as secret-ref object
	hash := strings.ToLower(RandStringStrSeed(8, componentName))
	return fmt.Sprintf("%s-%s-%s-%s", componentName, secretRefType, secretRefName, hash)
}

// GetComponentClientCertificateSecretName Gets name of the component secret that holds the ca.crt public key for clientcertificate authentication
func GetComponentClientCertificateSecretName(componentame string) string {
	return fmt.Sprintf("%s-clientcertca", componentame)
}

// GetSecretTypeForRadixAzureKeyVault Gets SecretType by RadixAzureKeyVaultK8sSecretType
func GetSecretTypeForRadixAzureKeyVault(k8sSecretType *radixv1.RadixAzureKeyVaultK8sSecretType) kube.SecretType {
	if k8sSecretType != nil && *k8sSecretType == radixv1.RadixAzureKeyVaultK8sSecretTypeTls {
		return kube.SecretTypeTls
	}
	return kube.SecretTypeOpaque
}

// GetAzureKeyVaultSecretRefSecretName Gets a secret name for Azure KeyVault RadixSecretRef
func GetAzureKeyVaultSecretRefSecretName(componentName, azKeyVaultName string, secretType kube.SecretType) string {
	radixSecretRefSecretType := string(getK8sSecretTypeRadixAzureKeyVaultK8sSecretType(secretType))
	return getSecretRefSecretName(componentName, string(radixv1.RadixSecretRefAzureKeyVault), radixSecretRefSecretType, azKeyVaultName)
}

func getSecretRefSecretName(componentName, secretRefType, secretType, secretResourceName string) string {
	return fmt.Sprintf("%s-%s-%s-%s-%s", componentName, secretRefType, secretType, secretResourceName, strings.ToLower(utils.RandString(5)))
}

func getK8sSecretTypeRadixAzureKeyVaultK8sSecretType(k8sSecretType kube.SecretType) radixv1.RadixAzureKeyVaultK8sSecretType {
	if k8sSecretType == kube.SecretTypeTls {
		return radixv1.RadixAzureKeyVaultK8sSecretTypeTls
	}
	return radixv1.RadixAzureKeyVaultK8sSecretTypeOpaque
}
