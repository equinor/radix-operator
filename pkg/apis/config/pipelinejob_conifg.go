package config

import (
	"slices"

	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
)

// Config for pipeline josb
type PipelineJobConfig struct {
	GitCloneImage           string            `envconfig:"RADIX_PIPELINE_GIT_CLONE_GIT_IMAGE" required:"true"`
	PipelineImage           string            `envconfig:"RADIXOPERATOR_PIPELINE_IMAGE" required:"true"`
	PipelineImagePullPolicy corev1.PullPolicy `envconfig:"RADIXOPERATOR_PIPELINE_IMAGE_PULL_POLICY" default:"Always"`
}

func (pjc *PipelineJobConfig) MustValidate() {
	if !slices.Contains([]corev1.PullPolicy{corev1.PullAlways, corev1.PullIfNotPresent, corev1.PullNever}, pjc.PipelineImagePullPolicy) {
		log.Warn().Msgf("RADIXOPERATOR_PIPELINE_IMAGE_PULL_POLICY has invalid value %q. Set to %s", pjc.PipelineImagePullPolicy, corev1.PullAlways)
		pjc.PipelineImagePullPolicy = corev1.PullAlways
	}
}
