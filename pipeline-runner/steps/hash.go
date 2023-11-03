package steps

import (
	"github.com/equinor/radix-operator/pipeline-runner/internal/hash"
	"github.com/equinor/radix-operator/pipeline-runner/model"
)

func getRadixApplicationHash(pipelineInfo *model.PipelineInfo) (string, error) {
	return hash.ToHashString(hash.SHA256, pipelineInfo.RadixApplication.Spec)
}

func getBuildSecretHash(pipelineInfo *model.PipelineInfo) (string, error) {
	if pipelineInfo.BuildSecret == nil {
		return "", nil
	}

	return hash.ToHashString(hash.SHA256, pipelineInfo.BuildSecret.Data)
}
