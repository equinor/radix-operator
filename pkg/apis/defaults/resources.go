package defaults

import (
	"os"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Environment variables that define default resources (limits and requests) for containers and environments
// See https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/memory-default-namespace/
const (
	OperatorEnvLimitDefaultMemoryEnvironmentVariable        = "RADIXOPERATOR_APP_ENV_LIMITS_DEFAULT_MEMORY"
	OperatorEnvLimitDefaultCPUEnvironmentVariable           = "RADIXOPERATOR_APP_ENV_LIMITS_DEFAULT_CPU"
	OperatorEnvLimitDefaultRequestMemoryEnvironmentVariable = "RADIXOPERATOR_APP_ENV_LIMITS_DEFAULT_REQUEST_MEMORY"
	OperatorEnvLimitDefaultReqestCPUEnvironmentVariable     = "RADIXOPERATOR_APP_ENV_LIMITS_DEFAULT_REQUEST_CPU"
)

// GetDefaultCPULimit Gets the default container CPU limit defined as an environment variable
func GetDefaultCPULimit() *resource.Quantity {
	return getQuantityFromEnvironmentVariable(OperatorEnvLimitDefaultCPUEnvironmentVariable)
}

// GetDefaultMemoryLimit Gets the default container memory limit defined as an environment variable
func GetDefaultMemoryLimit() *resource.Quantity {
	return getQuantityFromEnvironmentVariable(OperatorEnvLimitDefaultMemoryEnvironmentVariable)
}

// GetDefaultCPURequest Gets the default container CPU request defined as an environment variable
func GetDefaultCPURequest() *resource.Quantity {
	return getQuantityFromEnvironmentVariable(OperatorEnvLimitDefaultReqestCPUEnvironmentVariable)
}

// GetDefaultMemoryRequest Gets the default container memory request defined as an environment variable
func GetDefaultMemoryRequest() *resource.Quantity {
	return getQuantityFromEnvironmentVariable(OperatorEnvLimitDefaultRequestMemoryEnvironmentVariable)
}

func getQuantityFromEnvironmentVariable(envName string) *resource.Quantity {
	quantityAsString := os.Getenv(OperatorEnvLimitDefaultCPUEnvironmentVariable)
	if quantityAsString == "" {
		return nil
	}

	quantity := resource.MustParse(quantityAsString)
	return &quantity
}
