package config

import (
	"slices"
	"time"

	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
)

const (
	minPipelineJobsHistoryLimit       = 3
	minPipelineJobsHistoryPeriodLimit = time.Hour * 24
	minDeploymentsHistoryLimit        = 3
)

// Config for pipeline josb
type PipelineJobConfig struct {
	PipelineJobsHistoryLimit              int               `envconfig:"RADIX_PIPELINE_JOBS_HISTORY_LIMIT" required:"true" default:"3"`
	PipelineJobsHistoryPeriodLimit        time.Duration     `envconfig:"RADIX_PIPELINE_JOBS_HISTORY_PERIOD_LIMIT" required:"true" default:"24h"`
	DeploymentsHistoryLimitPerEnvironment int               `envconfig:"RADIX_DEPLOYMENTS_PER_ENVIRONMENT_HISTORY_LIMIT" required:"true" default:"3"`
	GitCloneImage                         string            `envconfig:"RADIX_PIPELINE_GIT_CLONE_GIT_IMAGE" required:"true"`
	PipelineImage                         string            `envconfig:"RADIXOPERATOR_PIPELINE_IMAGE" required:"true"`
	PipelineImagePullPolicy               corev1.PullPolicy `envconfig:"RADIXOPERATOR_PIPELINE_IMAGE_PULL_POLICY" default:"Always"`
}

func (pjc *PipelineJobConfig) MustValidate() {
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
	if !slices.Contains([]corev1.PullPolicy{corev1.PullAlways, corev1.PullIfNotPresent, corev1.PullNever}, pjc.PipelineImagePullPolicy) {
		log.Warn().Msgf("RADIXOPERATOR_PIPELINE_IMAGE_PULL_POLICY has invalid value %q. Set to %s", pjc.PipelineImagePullPolicy, corev1.PullAlways)
		pjc.PipelineImagePullPolicy = corev1.PullAlways
	}
}
