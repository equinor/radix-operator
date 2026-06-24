package models

import "time"

// Step holds general information about job step
// swagger:model Step
type Step struct {
	// Name of the step
	//
	// required: false
	// example: build
	Name string `json:"name"`

	// Status of the step
	//
	// required: false
	// enum: Queued,Waiting,Running,Succeeded,Failed,Stopped,StoppedNoChanges
	// example: Waiting
	Status string `json:"status"`

	// Started timestamp
	//
	// required: false
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	Started *time.Time `json:"started"`

	// Ended timestamp
	//
	// required: false
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	Ended *time.Time `json:"ended"`

	// Pod name
	//
	// required: false
	PodName string `json:"-"`

	// Components associated components
	//
	// required: false
	Components []string `json:"components,omitempty"`

	// SubPipelineTaskStep sub pipeline task step
	//
	// required: false
	SubPipelineTaskStep *SubPipelineTaskStep `json:"subPipelineTaskStep,omitempty"`
}

// SubPipelineTaskStep holds general information about subpipeline task step
// swagger:model SubPipelineTaskStep
type SubPipelineTaskStep struct {
	// Name of the step
	//
	// required: true
	// example: step-abc
	Name string `json:"name"`

	// PipelineName of the task
	//
	// required: true
	PipelineName string `json:"pipelineName"`

	// Environment of the pipeline run
	//
	// required: true
	Environment string `json:"environment"`

	// PipelineRunName of the task
	//
	// required: true
	PipelineRunName string `json:"pipelineRunName"`

	// TaskName of the task
	//
	// required: true
	// example: task-abc
	TaskName string `json:"taskName"`

	// KubeName Name of the pipeline run in the namespace
	//
	// required: true
	// example: radix-tekton-task-dev-2022-05-09-abcde
	KubeName string `json:"kubeName"`

	// Status of the step
	//
	// required: false
	// enum: Starting,Started,Running,Succeeded,Failed,Waiting,ToBeRetried,TaskRunCancelled,TaskRunTimeout,ResolvingTaskRef,ResolvingStepActionRef,TaskRunImagePullFailed,TaskRunResultLargerThanAllowedLimit,TaskRunStopSidecarFailed,InvalidParamValue,TaskRunResolutionFailed,TaskRunValidationFailedTaskValidationFailed,ResourceVerificationFailed,FailureIgnored,Error
	// example: Waiting
	Status string `json:"status"`
}
