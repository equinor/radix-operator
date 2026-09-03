package defaults

// Environment variables that define default resources (limits and requests) for containers and environments
// See https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/memory-default-namespace/
const (
	OperatorAppBuilderResourcesLimitsMemoryEnvironmentVariable   = "RADIXOPERATOR_APP_BUILDER_RESOURCES_LIMITS_MEMORY"
	OperatorAppBuilderResourcesLimitsCPUEnvironmentVariable      = "RADIXOPERATOR_APP_BUILDER_RESOURCES_LIMITS_CPU"
	OperatorAppBuilderResourcesRequestsMemoryEnvironmentVariable = "RADIXOPERATOR_APP_BUILDER_RESOURCES_REQUESTS_MEMORY"
	OperatorAppBuilderResourcesRequestsCPUEnvironmentVariable    = "RADIXOPERATOR_APP_BUILDER_RESOURCES_REQUESTS_CPU"
)
