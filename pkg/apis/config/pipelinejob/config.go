package pipelinejob

import (
	"time"

	"github.com/equinor/radix-operator/pkg/apis/git"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Config for pipeline jobs
type Config struct {
	PipelineJobsHistoryLimit              int
	PipelineJobsHistoryPeriodLimit        time.Duration
	DeploymentsHistoryLimitPerEnvironment int
	AppBuilderResourcesLimitsMemory       *resource.Quantity
	AppBuilderResourcesRequestsCPU        *resource.Quantity
	AppBuilderResourcesRequestsMemory     *resource.Quantity
	// GitCloneConfig Config for the git repo cloning
	GitCloneConfig *git.CloneConfig
	// PipelineImageTag Tag for the radix-pipeline image
	PipelineImageTag string
}
