package utils

import (
	"fmt"
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
