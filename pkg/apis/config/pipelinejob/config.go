package pipelinejob

import (
	"time"

	"github.com/rs/zerolog/log"
)

const (
	minPipelineJobsHistoryLimit       = 3
	minPipelineJobsHistoryPeriodLimit = time.Hour * 24
	minDeploymentsHistoryLimit        = 3
)

// Config for pipeline jobs
type Config struct {
	PipelineJobsHistoryLimit              int           `envconfig:"RADIX_PIPELINE_JOBS_HISTORY_LIMIT" required:"true" default:"3"`
	PipelineJobsHistoryPeriodLimit        time.Duration `envconfig:"RADIX_PIPELINE_JOBS_HISTORY_PERIOD_LIMIT" required:"true" default:"24h"`
	DeploymentsHistoryLimitPerEnvironment int           `envconfig:"RADIX_DEPLOYMENTS_PER_ENVIRONMENT_HISTORY_LIMIT" required:"true" default:"3"`
	AppBuilderResourcesLimitsCPU          *Quantity     `envconfig:"RADIXOPERATOR_APP_BUILDER_RESOURCES_LIMITS_CPU" required:"true"`
	AppBuilderResourcesLimitsMemory       *Quantity     `envconfig:"RADIXOPERATOR_APP_BUILDER_RESOURCES_LIMITS_MEMORY" required:"true"`
	AppBuilderResourcesRequestsCPU        *Quantity     `envconfig:"RADIXOPERATOR_APP_BUILDER_RESOURCES_REQUESTS_CPU" required:"true"`
	AppBuilderResourcesRequestsMemory     *Quantity     `envconfig:"RADIXOPERATOR_APP_BUILDER_RESOURCES_REQUESTS_MEMORY" required:"true"`
	GitCloneImage                         string        `envconfig:"RADIX_PIPELINE_GIT_CLONE_GIT_IMAGE" required:"true"`
	PipelineImage                         string        `envconfig:"RADIXOPERATOR_PIPELINE_IMAGE" required:"true"`
}

func (pjc *Config) MustValidate() {
	if pjc.PipelineJobsHistoryLimit < minPipelineJobsHistoryLimit {
		log.Warn().Msgf("RADIX_PIPELINE_JOBS_HISTORY_LIMIT should be at least %d. Set to minimum value", minPipelineJobsHistoryLimit)
		pjc.PipelineJobsHistoryLimit = minPipelineJobsHistoryLimit
	}
	if pjc.PipelineJobsHistoryPeriodLimit < minPipelineJobsHistoryPeriodLimit {
		log.Warn().Msgf("RADIX_PIPELINE_JOBS_HISTORY_PERIOD_LIMIT must be at least %s. Set to minimum value", minPipelineJobsHistoryPeriodLimit)
		pjc.PipelineJobsHistoryPeriodLimit = minPipelineJobsHistoryPeriodLimit
	}
	if pjc.DeploymentsHistoryLimitPerEnvironment < minDeploymentsHistoryLimit {
		log.Warn().Msgf("RADIX_DEPLOYMENTS_PER_ENVIRONMENT_HISTORY_LIMIT must be at least %d. Set to minimum value", minDeploymentsHistoryLimit)
		pjc.DeploymentsHistoryLimitPerEnvironment = minDeploymentsHistoryLimit
	}
	if pjc.AppBuilderResourcesRequestsCPU.Cmp(pjc.AppBuilderResourcesLimitsCPU.Quantity) > 0 {
		log.Fatal().Msg("RADIXOPERATOR_APP_BUILDER_RESOURCES_REQUESTS_CPU must be greater than RADIXOPERATOR_APP_BUILDER_RESOURCES_LIMITS_CPU")
	}
	if pjc.AppBuilderResourcesRequestsMemory.Cmp(pjc.AppBuilderResourcesLimitsMemory.Quantity) > 0 {
		log.Fatal().Msg("RADIXOPERATOR_APP_BUILDER_RESOURCES_REQUESTS_MEMORY must be greate than RADIXOPERATOR_APP_BUILDER_RESOURCES_LIMITS_MEMORY")
	}
}
