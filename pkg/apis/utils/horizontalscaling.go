package utils

import (
	"fmt"
)

// GetTriggerAuthenticationName Function to get trigger authentication name
func GetTriggerAuthenticationName(componentName, triggerName string) string {
	return fmt.Sprintf("%s-%s", componentName, triggerName)
}
