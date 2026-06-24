package models

import (
	"fmt"

	radixhttp "github.com/equinor/radix-common/net/http"
)

// NonExistingApplication No application found by name
func NonExistingApplication(underlyingError error, appName string) error {
	return radixhttp.TypeMissingError(fmt.Sprintf("Unable to get application for app %s", appName), underlyingError)
}

// IllegalEmptyEnvironment From environment does not exist
func IllegalEmptyEnvironment() error {
	return radixhttp.ValidationError("Radix Deployment", "Environment cannot be empty")
}

// NoActiveDeploymentFoundInEnvironment Deployment wasn't found
func NoActiveDeploymentFoundInEnvironment(appName, envName string) error {
	return radixhttp.TypeMissingError(fmt.Sprintf("No active deployment for %s was found in %s", appName, envName), nil)
}

// NonExistingDeployment Deployment wasn't found
func NonExistingDeployment(underlyingError error, deploymentName string) error {
	return radixhttp.TypeMissingError(fmt.Sprintf("Non existing deployment %s", deploymentName), underlyingError)
}

// NonExistingPod Pod by name was not found
func NonExistingPod(appName, podName string) error {
	return radixhttp.TypeMissingError(fmt.Sprintf("Unable to get pod %s for app %s", podName, appName), nil)
}
