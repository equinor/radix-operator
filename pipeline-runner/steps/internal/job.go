package internal

import (
	"strings"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/utils"
)

// GetJobNameHash returns a hash of the job name
func GetJobNameHash(pipelineInfo *model.PipelineInfo) string {
	return strings.ToLower(utils.RandStringStrSeed(5, pipelineInfo.PipelineArguments.JobName))
}
