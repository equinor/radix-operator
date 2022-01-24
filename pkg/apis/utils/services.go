package utils

import "fmt"

// GetAuxiliaryComponentServiceName returns service name for auxiliary component, e.g. the oauth proxy
func GetAuxiliaryComponentServiceName(componentName string, auxSuffix string) string {
	return fmt.Sprintf("%s-%s", componentName, auxSuffix)
}
