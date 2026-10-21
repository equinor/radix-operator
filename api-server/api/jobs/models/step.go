package models

import "time"

// StepType Enumeration of the types of job steps
// swagger:enum StepType
type StepType string

const (
	StepTypeCloneConfig     StepType = "CloneConfig"
	StepTypePipelineRunner  StepType = "PipelineRunner"
	StepTypeClone           StepType = "Clone"
	StepTypeBuild           StepType = "Build"
	StepTypeSubPipelineStep StepType = "SubPipelineStep"
	StepTypeUnknown         StepType = "Unknown"
)

// Step holds general information about job step
// swagger:model Step
type Step struct {
	// Name of the step
	//
	// required: false
	// example: build
	Name string `json:"name"`

	// Type of the step
	//
	// example: Build
	Type StepType `json:"type"`

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

	// Pod name for the step
	//
	// required: false
	PodName string `json:"podName"`

	// ContainerName for the step's container
	//
	// required: false
	ContainerName string `json:"containerName"`

	// Components lists components associated with this step
	//
	// required: false
	Components []string `json:"components,omitempty"`

	// Environments lists environments associated with this step
	//
	// required: false
	Environments []string `json:"environments,omitempty"`

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
