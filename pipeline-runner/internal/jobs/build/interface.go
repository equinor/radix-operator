package build

import (
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	batchv1 "k8s.io/api/batch/v1"
)

// JobsBuilder defines interface for creating pipeline build jobs
type JobsBuilder interface {
	// BuildJobs returns a slice of Kubernetes jobs to be used for building container images for Radix components and jobs
	BuildJobs(useBuildCache, refreshBuildCache bool, pipelineArgs model.PipelineArguments, cloneURL, gitCommitHash, gitTags string, componentImages []pipeline.BuildComponentImage, buildSecrets []string, appID radixv1.ULID, imagePullSecret string) []batchv1.Job
}
