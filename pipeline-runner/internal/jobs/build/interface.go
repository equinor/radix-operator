package build

import (
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/oklog/ulid/v2"
	batchv1 "k8s.io/api/batch/v1"
)

// JobsBuilder defines interface for creating pipeline build jobs
type JobsBuilder interface {
	// BuildJobs returns a slice of Kubernetes jobs to be used for building container images for Radix components and jobs
	BuildJobs(useBuildCache, refreshBuildCache bool, pipelineArgs model.PipelineArguments, cloneURL, gitCommitHash, gitTags string, componentImages []pipeline.BuildComponentImage, buildSecrets []string, appID ulid.ULID) []batchv1.Job
}
