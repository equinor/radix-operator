package promote

import "fmt"

// EmptyArgument Argument by name cannot be empty
func EmptyArgument(argumentName string) error {
	return fmt.Errorf("%s cannot be empty", argumentName)
}

// NonExistingFromEnvironment From environment does not exist
func NonExistingFromEnvironment(environment string) error {
	return fmt.Errorf("non existing from environment %s", environment)
}

// NonExistingToEnvironment To environment does not exist
func NonExistingToEnvironment(environment string) error {
	return fmt.Errorf("non existing to environment %s", environment)
}

// NonExistingDeployment Deployment wasn't found
func NonExistingDeployment(deploymentName string) error {
	return fmt.Errorf("non existing deployment %s", deploymentName)
}

// NonExistingComponentName Component by name was not found
func NonExistingComponentName(appName, componentName string) error {
	return fmt.Errorf("unable to get application component %s for app %s", componentName, appName)
}
