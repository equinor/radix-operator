package pipelinejob

import "k8s.io/apimachinery/pkg/api/resource"

// Config for pipeline jobs
type Config struct {
	PipelineJobsHistoryLimit              int
	DeploymentsHistoryLimitPerEnvironment int
	AppBuilderResourcesLimitsMemory       *resource.Quantity
	AppBuilderResourcesRequestsCPU        *resource.Quantity
	AppBuilderResourcesRequestsMemory     *resource.Quantity
}
