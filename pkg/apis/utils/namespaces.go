package utils

import (
	"fmt"
	"strings"
)

// AppNamespaceEnvName Name of environment for app namespace
const AppNamespaceEnvName = "app"

// GetAppNamespace Function to get namespace from app name
func GetAppNamespace(appName string) string {
	return fmt.Sprintf("%s-app", appName)
}

// GetEnvironmentNamespace Function to get namespace from app name and environment
func GetEnvironmentNamespace(appName, environment string) string {
	return fmt.Sprintf("%s-%s", appName, environment)
}

// GetDeploymentName Function to get deployment name
func GetDeploymentName(appName, env, tag string) string {
	random := strings.ToLower(RandString(8))
	return fmt.Sprintf("%s-%s-%s", env, tag, random)
}

// GetAuxiliaryComponentDeploymentName returns deployment name for auxiliary component, e.g. the oauth proxy
func GetAuxiliaryComponentDeploymentName(componentName string, auxSuffix string) string {
	return fmt.Sprintf("%s-%s", componentName, auxSuffix)
}

func GetAuxiliaryComponentSecretName(componentName string, suffix string) string {
	return GetComponentSecretName(GetAuxiliaryComponentDeploymentName(componentName, suffix))
}

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

// GetAppAndTagPairFromName Reverse engineer deployment name
func GetAppAndTagPairFromName(name string) (string, string) {
	runes := []rune(name)
	lastIndex := strings.LastIndex(name, "-")
	return string(runes[0:lastIndex]), string(runes[(lastIndex + 1):len(runes)])
}
