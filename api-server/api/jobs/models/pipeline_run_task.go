package models

import tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

// PipelineRunTask holds general information about pipeline run task
// swagger:model PipelineRunTask
type PipelineRunTask struct {
	// Name of the task
	//
	// required: true
	// example: build
	Name string `json:"name"`

	// KubeName Name of the pipeline run in the namespace
	//
	// required: true
	// example: radix-tekton-task-dev-2022-05-09-abcde
	KubeName string `json:"kubeName"`

	// PipelineRunEnv Environment of the pipeline run
	//
	// required: true
	// example: prod
	PipelineRunEnv string `json:"pipelineRunEnv"`

	// PipelineName of the task
	//
	// required: true
	// example: build-pipeline
	PipelineName string `json:"pipelineName"`

	// Status of the task
	//
	// required: false
	Status PipelineRunReason `json:"status"`

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

// PipelineRunReason copies the fields from github.com/tektoncd/pipeline so go-swagger can map the enums
// swagger:enum PipelineRunReason
type PipelineRunReason tektonv1.PipelineRunReason

const (
	// PipelineRunReasonStarted is the reason set when the PipelineRun has just started
	PipelineRunReasonStarted PipelineRunReason = "Started"
	// PipelineRunReasonRunning is the reason set when the PipelineRun is running
	PipelineRunReasonRunning PipelineRunReason = "Running"
	// PipelineRunReasonSuccessful is the reason set when the PipelineRun completed successfully
	PipelineRunReasonSuccessful PipelineRunReason = "Succeeded"
	// PipelineRunReasonCompleted is the reason set when the PipelineRun completed successfully with one or more skipped Tasks
	PipelineRunReasonCompleted PipelineRunReason = "Completed"
	// PipelineRunReasonFailed is the reason set when the PipelineRun completed with a failure
	PipelineRunReasonFailed PipelineRunReason = "Failed"
	// PipelineRunReasonCancelled is the reason set when the PipelineRun cancelled by the user
	// This reason may be found with a corev1.ConditionFalse status, if the cancellation was processed successfully
	// This reason may be found with a corev1.ConditionUnknown status, if the cancellation is being processed or failed
	PipelineRunReasonCancelled PipelineRunReason = "Cancelled"
	// PipelineRunReasonPending is the reason set when the PipelineRun is in the pending state
	PipelineRunReasonPending PipelineRunReason = "PipelineRunPending"
	// PipelineRunReasonTimedOut is the reason set when the PipelineRun has timed out
	PipelineRunReasonTimedOut PipelineRunReason = "PipelineRunTimeout"
	// PipelineRunReasonStopping indicates that no new Tasks will be scheduled by the controller, and the
	// pipeline will stop once all running tasks complete their work
	PipelineRunReasonStopping PipelineRunReason = "PipelineRunStopping"
	// PipelineRunReasonCancelledRunningFinally indicates that pipeline has been gracefully cancelled
	// and no new Tasks will be scheduled by the controller, but final tasks are now running
	PipelineRunReasonCancelledRunningFinally PipelineRunReason = "CancelledRunningFinally"
	// PipelineRunReasonStoppedRunningFinally indicates that pipeline has been gracefully stopped
	// and no new Tasks will be scheduled by the controller, but final tasks are now running
	PipelineRunReasonStoppedRunningFinally PipelineRunReason = "StoppedRunningFinally"
	// ReasonCouldntGetPipeline indicates that the reason for the failure status is that the
	// associated Pipeline couldn't be retrieved
	PipelineRunReasonCouldntGetPipeline PipelineRunReason = "CouldntGetPipeline"
	// ReasonInvalidBindings indicates that the reason for the failure status is that the
	// PipelineResources bound in the PipelineRun didn't match those declared in the Pipeline
	PipelineRunReasonInvalidBindings PipelineRunReason = "InvalidPipelineResourceBindings"
	// ReasonInvalidWorkspaceBinding indicates that a Pipeline expects a workspace but a
	// PipelineRun has provided an invalid binding.
	PipelineRunReasonInvalidWorkspaceBinding PipelineRunReason = "InvalidWorkspaceBindings"
	// ReasonInvalidTaskRunSpec indicates that PipelineRun.Spec.TaskRunSpecs[].PipelineTaskName is defined with
	// a not exist taskName in pipelineSpec.
	PipelineRunReasonInvalidTaskRunSpec PipelineRunReason = "InvalidTaskRunSpecs"
	// ReasonParameterTypeMismatch indicates that the reason for the failure status is that
	// parameter(s) declared in the PipelineRun do not have the some declared type as the
	// parameters(s) declared in the Pipeline that they are supposed to override.
	PipelineRunReasonParameterTypeMismatch PipelineRunReason = "ParameterTypeMismatch"
	// ReasonObjectParameterMissKeys indicates that the object param value provided from PipelineRun spec
	// misses some keys required for the object param declared in Pipeline spec.
	PipelineRunReasonObjectParameterMissKeys PipelineRunReason = "ObjectParameterMissKeys"
	// ReasonParamArrayIndexingInvalid indicates that the use of param array indexing is not under correct api fields feature gate
	// or the array is out of bound.
	PipelineRunReasonParamArrayIndexingInvalid PipelineRunReason = "ParamArrayIndexingInvalid"
	// ReasonCouldntGetTask indicates that the reason for the failure status is that the
	// associated Pipeline's Tasks couldn't all be retrieved
	PipelineRunReasonCouldntGetTask PipelineRunReason = "CouldntGetTask"
	// ReasonParameterMissing indicates that the reason for the failure status is that the
	// associated PipelineRun didn't provide all the required parameters
	PipelineRunReasonParameterMissing PipelineRunReason = "ParameterMissing"
	// ReasonFailedValidation indicates that the reason for failure status is
	// that pipelinerun failed runtime validation
	PipelineRunReasonFailedValidation PipelineRunReason = "PipelineValidationFailed"
	// PipelineRunReasonCouldntGetPipelineResult indicates that the pipeline fails to retrieve the
	// referenced result. This could be due to failed TaskRuns or Runs that were supposed to produce
	// the results
	PipelineRunReasonCouldntGetPipelineResult PipelineRunReason = "CouldntGetPipelineResult"
	// ReasonInvalidGraph indicates that the reason for the failure status is that the
	// associated Pipeline is an invalid graph (a.k.a wrong order, cycle, …)
	PipelineRunReasonInvalidGraph PipelineRunReason = "PipelineInvalidGraph"
	// ReasonCouldntCancel indicates that a PipelineRun was cancelled but attempting to update
	// all of the running TaskRuns as cancelled failed.
	PipelineRunReasonCouldntCancel PipelineRunReason = "PipelineRunCouldntCancel"
	// ReasonCouldntTimeOut indicates that a PipelineRun was timed out but attempting to update
	// all of the running TaskRuns as timed out failed.
	PipelineRunReasonCouldntTimeOut PipelineRunReason = "PipelineRunCouldntTimeOut"
	// ReasonInvalidMatrixParameterTypes indicates a matrix contains invalid parameter types
	PipelineRunReasonInvalidMatrixParameterTypes PipelineRunReason = "InvalidMatrixParameterTypes"
	// ReasonInvalidTaskResultReference indicates a task result was declared
	// but was not initialized by that task
	PipelineRunReasonInvalidTaskResultReference PipelineRunReason = "InvalidTaskResultReference"
	// ReasonRequiredWorkspaceMarkedOptional indicates an optional workspace
	// has been passed to a Task that is expecting a non-optional workspace
	PipelineRunReasonRequiredWorkspaceMarkedOptional PipelineRunReason = "RequiredWorkspaceMarkedOptional"
	// ReasonResolvingPipelineRef indicates that the PipelineRun is waiting for
	// its pipelineRef to be asynchronously resolved.
	PipelineRunReasonResolvingPipelineRef PipelineRunReason = "ResolvingPipelineRef"
	// ReasonResourceVerificationFailed indicates that the pipeline fails the trusted resource verification,
	// it could be the content has changed, signature is invalid or public key is invalid
	PipelineRunReasonResourceVerificationFailed PipelineRunReason = "ResourceVerificationFailed"
	// ReasonCreateRunFailed indicates that the pipeline fails to create the taskrun or other run resources
	PipelineRunReasonCreateRunFailed PipelineRunReason = "CreateRunFailed"
	// ReasonCELEvaluationFailed indicates the pipeline fails the CEL evaluation
	PipelineRunReasonCELEvaluationFailed PipelineRunReason = "CELEvaluationFailed"
	// PipelineRunReasonInvalidParamValue indicates that the PipelineRun Param input value is not allowed.
	PipelineRunReasonInvalidParamValue PipelineRunReason = "InvalidParamValue"
)
