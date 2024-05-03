package utils

import (
	"strings"
)

// IsRadixEnvVar Indicates if environment-variable is created by Radix
func IsRadixEnvVar(envVarName string) bool {
	return strings.HasPrefix(envVarName, "RADIX_") || strings.HasPrefix(envVarName, "RADIXOPERATOR_")
}
