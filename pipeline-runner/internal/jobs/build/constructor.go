package build

import (
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	batchv1 "k8s.io/api/batch/v1"
)

type Constructor interface {
	ConstructJobs() ([]batchv1.Job, error)
}

func GetConstructor(useBuildKit, useBuildCache bool, pipelineArgs model.PipelineArguments, cloneURL, gitCommitHash, gitTags string, componentImages []pipeline.BuildComponentImage, buildSecrets []string) Constructor {
	if useBuildKit {
		return &buildahConstructor{
			pipelineArgs:    pipelineArgs,
			componentImages: componentImages,
			cloneURL:        cloneURL,
			gitCommitHash:   gitCommitHash,
			gitTags:         gitTags,
			buildSecrets:    buildSecrets,
			useBuildCache:   useBuildCache,
		}
	}
	return &acrConstructor{
		pipelineArgs:    pipelineArgs,
		componentImages: componentImages,
		cloneURL:        cloneURL,
		gitCommitHash:   gitCommitHash,
		gitTags:         gitTags,
		buildSecrets:    buildSecrets,
	}
}
