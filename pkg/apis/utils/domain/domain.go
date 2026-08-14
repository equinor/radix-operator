package domain

import "fmt"

// GetComponentHostname returns the domain label for a component in a namespace.
func GetComponentHostname(componentName, namespace string) string {
	return fmt.Sprintf("%s-%s", componentName, namespace)
}
