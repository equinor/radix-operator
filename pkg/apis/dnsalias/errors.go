package dnsalias

import "fmt"

// DeployComponentNotFoundByName Deploy component not found
func DeployComponentNotFoundByName(appName, envName, componentName, deploymentName string) error {
	return fmt.Errorf("component %s not found in environment %s in application %s for deployment %s", componentName, envName, appName, deploymentName)
}
