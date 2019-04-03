package steps

import (
	"github.com/equinor/radix-operator/pipeline-runner/model"
)

type Step interface {
	ErrorMsg(pipelineInfo model.PipelineInfo, err error) string
	SucceededMsg(pipelineInfo model.PipelineInfo) string
	Run(pipelineInfo model.PipelineInfo) error
}
