package utils

import "fmt"

// GetComponentServiceAccountName Gets unique name for component or job service account
func GetComponentServiceAccountName(componentName string) string {
	return fmt.Sprintf("%s-sa", componentName)
}
