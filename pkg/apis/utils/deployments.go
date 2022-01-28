package utils

import (
	"fmt"
	"strings"
)

// GetDeploymentName Function to get deployment name
func GetDeploymentName(appName, env, tag string) string {
	random := strings.ToLower(RandString(8))
	return fmt.Sprintf("%s-%s-%s", env, tag, random)
}

// GetAuxiliaryComponentDeploymentName returns deployment name for auxiliary component, e.g. the oauth proxy
func GetAuxiliaryComponentDeploymentName(componentName string, auxSuffix string) string {
	return fmt.Sprintf("%s-%s", componentName, auxSuffix)
}
