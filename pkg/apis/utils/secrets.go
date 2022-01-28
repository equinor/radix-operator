package utils

import (
	"fmt"
	"strings"
)

// GetComponentSecretName Gets unique name of the component secret
func GetComponentSecretName(componentName string) string {
	// include a hash so that users cannot get access to a secret they should not get
	// by naming component the same as secret object
	hash := strings.ToLower(RandStringStrSeed(8, componentName))
	return fmt.Sprintf("%s-%s", componentName, hash)
}

// GetComponentClientCertificateSecretName Gets name of the component secret that holds the ca.crt public key for client certificate authentication
func GetComponentClientCertificateSecretName(componentame string) string {
	return fmt.Sprintf("%s-clientcertca", componentame)
}

func GetAuxiliaryComponentSecretName(componentName string, suffix string) string {
	return GetComponentSecretName(GetAuxiliaryComponentDeploymentName(componentName, suffix))
}
