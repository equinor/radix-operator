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
	defaultCPULimitSetting := os.Getenv(OperatorEnvLimitDefaultCPUEnvironmentVariable)
	if defaultCPULimitSetting == "" {
		return nil
	}

	defaultCPULimit := resource.MustParse(defaultCPULimitSetting)
	return &defaultCPULimit
}

// GetDefaultMemoryLimit Gets the default container memory limit defined as an environment variable
func GetDefaultMemoryLimit() *resource.Quantity {
	defaultMemoryLimitSetting := os.Getenv(OperatorEnvLimitDefaultMemoryEnvironmentVariable)
	if defaultMemoryLimitSetting == "" {
		return nil
	}

	defaultMemoryLimit := resource.MustParse(defaultMemoryLimitSetting)
	return &defaultMemoryLimit
}

// GetDefaultCPURequest Gets the default container CPU request defined as an environment variable
func GetDefaultCPURequest() *resource.Quantity {
	defaultCPURequestSetting := os.Getenv(OperatorEnvLimitDefaultReqestCPUEnvironmentVariable)
	if defaultCPURequestSetting == "" {
		return nil
	}

	defaultCPURequest := resource.MustParse(defaultCPURequestSetting)
	return &defaultCPURequest
}

// GetDefaultMemoryRequest Gets the default container memory request defined as an environment variable
func GetDefaultMemoryRequest() *resource.Quantity {
	defaultMemoryRequestSetting := os.Getenv(OperatorEnvLimitDefaultRequestMemoryEnvironmentVariable)
	if defaultMemoryRequestSetting == "" {
		return nil
	}

	defaultMemoryRequest := resource.MustParse(defaultMemoryRequestSetting)
	return &defaultMemoryRequest
}
