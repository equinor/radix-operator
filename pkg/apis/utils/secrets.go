package utils

import (
	"fmt"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"strings"
)

// GetComponentSecretName Gets unique name of the component secret
func GetComponentSecretName(componentName string) string {
	// include a hash so that users cannot get access to a secret they should not ,
	// by naming component the same as secret object
	hash := strings.ToLower(RandStringStrSeed(8, componentName))
	return fmt.Sprintf("%s-%s", componentName, hash)
}

// GetComponentClientCertificateSecretName Gets name of the component secret that holds the ca.crt public key for clientcertificate authentication
func GetComponentClientCertificateSecretName(componentame string) string {
	return fmt.Sprintf("%s-clientcertca", componentame)
}

// CreateComponentAzureKeyVaultCredentialsSecretName Gets unique name of the component key-vault secret
func CreateComponentAzureKeyVaultCredentialsSecretName(componentName string, radixSecretRefType v1.RadixSecretRefType, secretRefName string) string {
	// include a hash so that users cannot get access to a key-vault secret they should not ,
	// by naming component the same as key-vault secret object
	hash := strings.ToLower(RandStringStrSeed(8, componentName))
	return fmt.Sprintf("%s-%s-%s-creds-%s", componentName, radixSecretRefType, secretRefName, hash)
}

// GetComponentSecretProviderClassName Gets unique name of the component secret storage class
func GetComponentSecretProviderClassName(componentName, secretRefType, secretRefName string) string {
	// include a hash so that users cannot get access to a secret-ref they should not ,
	// by naming component the same as secret-ref object
	hash := strings.ToLower(RandStringStrSeed(8, componentName))
	return fmt.Sprintf("%s-%s-%s-%s", componentName, secretRefType, secretRefName, hash)
}
