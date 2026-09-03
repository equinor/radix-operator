package defaults

import (
	"os"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Environment variables that define default resources (limits and requests) for containers and environments
// See https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/memory-default-namespace/
const (
	OperatorEnvLimitDefaultMemoryEnvironmentVariable        = "RADIXOPERATOR_APP_ENV_LIMITS_DEFAULT_MEMORY"
	OperatorEnvLimitDefaultRequestMemoryEnvironmentVariable = "RADIXOPERATOR_APP_ENV_LIMITS_DEFAULT_REQUEST_MEMORY"
	OperatorEnvLimitDefaultRequestCPUEnvironmentVariable    = "RADIXOPERATOR_APP_ENV_LIMITS_DEFAULT_REQUEST_CPU"

	OperatorAppBuilderResourcesLimitsMemoryEnvironmentVariable   = "RADIXOPERATOR_APP_BUILDER_RESOURCES_LIMITS_MEMORY"
	OperatorAppBuilderResourcesLimitsCPUEnvironmentVariable      = "RADIXOPERATOR_APP_BUILDER_RESOURCES_LIMITS_CPU"
	OperatorAppBuilderResourcesRequestsMemoryEnvironmentVariable = "RADIXOPERATOR_APP_BUILDER_RESOURCES_REQUESTS_MEMORY"
	OperatorAppBuilderResourcesRequestsCPUEnvironmentVariable    = "RADIXOPERATOR_APP_BUILDER_RESOURCES_REQUESTS_CPU"
)

// GetDefaultMemoryLimit Gets the default container memory limit defined as an environment variable
func GetDefaultMemoryLimit() *resource.Quantity {
	return getQuantityFromEnvironmentVariable(OperatorEnvLimitDefaultMemoryEnvironmentVariable)
}

// GetDefaultCPURequest Gets the default container CPU request defined as an environment variable
func GetDefaultCPURequest() *resource.Quantity {
	return getQuantityFromEnvironmentVariable(OperatorEnvLimitDefaultRequestCPUEnvironmentVariable)
}

// GetDefaultMemoryRequest Gets the default container memory request defined as an environment variable
func GetDefaultMemoryRequest() *resource.Quantity {
	return getQuantityFromEnvironmentVariable(OperatorEnvLimitDefaultRequestMemoryEnvironmentVariable)
}

// GetResourcesLimitsMemoryForAppBuilderNamespace Gets the default container memory limit for builder job in app namespaces defined as an environment variable
func GetResourcesLimitsMemoryForAppBuilderNamespace() *resource.Quantity {
	return getQuantityFromEnvironmentVariable(OperatorAppBuilderResourcesLimitsMemoryEnvironmentVariable)
}

// GetResourcesLimitsCPUForAppBuilderNamespace Gets the default container CPU limit for builder job in app namespaces defined as an environment variable
func GetResourcesLimitsCPUForAppBuilderNamespace() *resource.Quantity {
	return getQuantityFromEnvironmentVariable(OperatorAppBuilderResourcesLimitsCPUEnvironmentVariable)
}

// GetResourcesRequestsCPUForAppBuilderNamespace Gets the default container CPU request for builder job in app namespaces defined as an environment variable
func GetResourcesRequestsCPUForAppBuilderNamespace() *resource.Quantity {
	return getQuantityFromEnvironmentVariable(OperatorAppBuilderResourcesRequestsCPUEnvironmentVariable)
}

// GetResourcesRequestsMemoryForAppBuilderNamespace Gets the default container memory request for builder job in app namespaces defined as an environment variable
func GetResourcesRequestsMemoryForAppBuilderNamespace() *resource.Quantity {
	return getQuantityFromEnvironmentVariable(OperatorAppBuilderResourcesRequestsMemoryEnvironmentVariable)
}

func getQuantityFromEnvironmentVariable(envName string) *resource.Quantity {
	quantityAsString := os.Getenv(envName)
	if quantityAsString == "" {
		return nil
	}

	quantity := resource.MustParse(quantityAsString)
	return &quantity
}
