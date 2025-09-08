package pipelinejob

import (
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Config for pipeline jobs
type Config struct {
	PipelineJobsHistoryLimit              int
	PipelineJobsHistoryPeriodLimit        time.Duration
	DeploymentsHistoryLimitPerEnvironment int
	AppBuilderResourcesLimitsCPU          *resource.Quantity
	AppBuilderResourcesLimitsMemory       *resource.Quantity
	AppBuilderResourcesRequestsCPU        *resource.Quantity
	AppBuilderResourcesRequestsMemory     *resource.Quantity
	GitCloneImage                         string
	PipelineImage                         string
}
