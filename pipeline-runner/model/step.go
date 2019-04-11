package model

type Step interface {
	ErrorMsg(pipelineInfo PipelineInfo, err error) string
	SucceededMsg(pipelineInfo PipelineInfo) string
	Run(pipelineInfo PipelineInfo) error
}

type NullStep struct {
	ErrorMessage   string
	SuccessMessage string
	Error          error
}

func (step NullStep) ErrorMsg(pipelineInfo PipelineInfo, err error) string {
	return step.ErrorMessage
}
func (step NullStep) SucceededMsg(pipelineInfo PipelineInfo) string {
	return step.SuccessMessage
}
func (step NullStep) Run(pipelineInfo PipelineInfo) error {
	return step.Error
}
