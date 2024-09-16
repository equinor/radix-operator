package build

import (
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	batchv1 "k8s.io/api/batch/v1"
)

// JobsBuilder defines interface for creating pipeline build jobs
type JobsBuilder interface {
	// BuildJobs returns a slice of Kubernetes jobs to be used for building container images for Radix components and jobs
	BuildJobs(useBuildCache bool, pipelineArgs model.PipelineArguments, cloneURL, gitCommitHash, gitTags string, componentImages []pipeline.BuildComponentImage, buildSecrets []string) []batchv1.Job
}
