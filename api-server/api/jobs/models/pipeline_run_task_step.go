package models

import tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

// PipelineRunTaskStep holds general information about pipeline run task steps
// swagger:model PipelineRunTaskStep
type PipelineRunTaskStep struct {
	// Name of the step
	//
	// required: true
	// example: build
	Name string `json:"name"`

	// Status of the task
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

// TaskRunReason copies the fields from github.com/tektoncd/pipeline so go-swagger can map the enums
// swagger:enum TaskRunReason
type TaskRunReason tektonv1.TaskRunReason

const (
	// TaskRunReasonStarted is the reason set when the TaskRun has just started
	TaskRunReasonStarted TaskRunReason = "Started"
	// TaskRunReasonRunning is the reason set when the TaskRun is running
	TaskRunReasonRunning TaskRunReason = "Running"
	// TaskRunReasonSuccessful is the reason set when the TaskRun completed successfully
	TaskRunReasonSuccessful TaskRunReason = "Succeeded"
	// TaskRunReasonFailed is the reason set when the TaskRun completed with a failure
	TaskRunReasonFailed TaskRunReason = "Failed"
	// TaskRunReasonToBeRetried is the reason set when the last TaskRun execution failed, and will be retried
	TaskRunReasonToBeRetried TaskRunReason = "ToBeRetried"
	// TaskRunReasonCancelled is the reason set when the TaskRun is cancelled by the user
	TaskRunReasonCancelled TaskRunReason = "TaskRunCancelled"
	// TaskRunReasonTimedOut is the reason set when one TaskRun execution has timed out
	TaskRunReasonTimedOut TaskRunReason = "TaskRunTimeout"
	// TaskRunReasonResolvingTaskRef indicates that the TaskRun is waiting for
	// its taskRef to be asynchronously resolved.
	TaskRunReasonResolvingTaskRef = "ResolvingTaskRef"
	// TaskRunReasonResolvingStepActionRef indicates that the TaskRun is waiting for
	// its StepAction's Ref to be asynchronously resolved.
	TaskRunReasonResolvingStepActionRef = "ResolvingStepActionRef"
	// TaskRunReasonImagePullFailed is the reason set when the step of a task fails due to image not being pulled
	TaskRunReasonImagePullFailed TaskRunReason = "TaskRunImagePullFailed"
	// TaskRunReasonResultLargerThanAllowedLimit is the reason set when one of the results exceeds its maximum allowed limit of 1 KB
	TaskRunReasonResultLargerThanAllowedLimit TaskRunReason = "TaskRunResultLargerThanAllowedLimit"
	// TaskRunReasonStopSidecarFailed indicates that the sidecar is not properly stopped.
	TaskRunReasonStopSidecarFailed TaskRunReason = "TaskRunStopSidecarFailed"
	// TaskRunReasonInvalidParamValue indicates that the TaskRun Param input value is not allowed.
	TaskRunReasonInvalidParamValue TaskRunReason = "InvalidParamValue"
	// TaskRunReasonFailedResolution indicated that the reason for failure status is
	// that references within the TaskRun could not be resolved
	TaskRunReasonFailedResolution TaskRunReason = "TaskRunResolutionFailed"
	// TaskRunReasonFailedValidation indicated that the reason for failure status is
	// that taskrun failed runtime validation
	TaskRunReasonFailedValidation TaskRunReason = "TaskRunValidationFailed"
	// TaskRunReasonTaskFailedValidation indicated that the reason for failure status is
	// that task failed runtime validation
	TaskRunReasonTaskFailedValidation TaskRunReason = "TaskValidationFailed"
	// TaskRunReasonResourceVerificationFailed indicates that the task fails the trusted resource verification,
	// it could be the content has changed, signature is invalid or public key is invalid
	TaskRunReasonResourceVerificationFailed TaskRunReason = "ResourceVerificationFailed"
	// TaskRunReasonFailureIgnored is the reason set when the Taskrun has failed due to pod execution error and the failure is ignored for the owning PipelineRun.
	// TaskRuns failed due to reconciler/validation error should not use this reason.
	TaskRunReasonFailureIgnored TaskRunReason = "FailureIgnored"
)
