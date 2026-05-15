package models

// PipelineRun holds general information about pipeline run
// swagger:model PipelineRun
type PipelineRun struct {
	// Name Original name of the pipeline run
	//
	// required: true
	// example: build-pipeline
	Name string `json:"name"`

	// Env Environment of the pipeline run
	//
	// required: true
	// example: prod
	Env string `json:"env"`

	// KubeName Name of the pipeline run in the namespace
	//
	// required: true
	// example: radix-tekton-pipelinerun-dev-2022-05-09-abcde
	KubeName string `json:"kubeName"`

	// Status of the step
	//
	// required: false
	Status TaskRunReason `json:"status"`

	// StatusMessage of the task
	//
	// required: false
	StatusMessage string `json:"statusMessage"`

	// Started timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Started string `json:"started"`

	// Ended timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Ended string `json:"ended"`
}
