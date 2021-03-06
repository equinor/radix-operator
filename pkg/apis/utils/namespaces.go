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

// GetComponentSecretName Gets unique name of the component secret
func GetComponentSecretName(componentame string) string {
	// include a hash so that users cannot get access to a secret they should not ,
	// by naming component the same as secret object
	hash := strings.ToLower(RandStringStrSeed(8, componentame))
	return fmt.Sprintf("%s-%s", componentame, hash)
}

// GetAppAndTagPairFromName Reverse engineer deployment name
func GetAppAndTagPairFromName(name string) (string, string) {
	runes := []rune(name)
	lastIndex := strings.LastIndex(name, "-")
	return string(runes[0:lastIndex]), string(runes[(lastIndex + 1):len(runes)])
}
