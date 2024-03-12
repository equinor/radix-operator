package v1

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Job",type="string",JSONPath=".spec.radixDeploymentJobRef.job"
// +kubebuilder:printcolumn:name="Condition",type="string",JSONPath=".status.condition.type"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:path=radixbatches,shortName=rb
// +kubebuilder:subresource:status

// RadixBatch enables batch execution of Radix job components.
type RadixBatch struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`

	Spec   RadixBatchSpec   `json:"spec"`
	Status RadixBatchStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixBatchList is a collection of RadixBatches.
type RadixBatchList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`

	Items []RadixBatch `json:"items"`
}

// RadixBatchSpec is the specification of batch jobs.
type RadixBatchSpec struct {
	// Reference to the RadixDeployment containing the job component spec.
	RadixDeploymentJobRef RadixDeploymentJobComponentSelector `json:"radixDeploymentJobRef"`

	// List of batch jobs to run.
	// +listType:=map
	// +listMapKey:=name
	// +kubebuilder:validation:MinItems:=1
	// +kubebuilder:validation:MaxItems:=500
	Jobs []RadixBatchJob `json:"jobs"`
}

// RadixBatchJob Spec for a batch job
type RadixBatchJob struct {
	// Defines the unique name of the job in a RadixBatch.
	// +kubebuilder:validation:MaxLength:=63
	Name string `json:"name"`

	// Defines a user defined ID of the job.
	// +optional
	JobId string `json:"jobId,omitempty"`

	// Specifies compute resource requirements.
	// Overrides resource configuration defined for job component in RadixDeployment.
	// +optional
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// Specifies node attributes, where container should be scheduled.
	// Overrides node configuration defined for job component in RadixDeployment.
	// +optional
	Node *RadixNode `json:"node,omitempty"`

	// Specifies maximum job run time.
	// Overrides timeLimitSeconds defined for job component in RadixDeployment.
	// +optional
	// +kubebuilder:validation:Minimum:=1
	TimeLimitSeconds *int64 `json:"timeLimitSeconds,omitempty"`

	// Specifies the number of retries before marking this job failed.
	// +optional
	// +kubebuilder:validation:Minimum:=0
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`

	// Specifies the Secret name and data key containing the payload for the job
	// +optional
	PayloadSecretRef *PayloadSecretKeySelector `json:"payloadSecretRef,omitempty"`

	// Controls if a job should be stopped.
	// If Stop is set to true, the underlying Kubernetes Job is deleted.
	// A job that is stopped cannot be started again by setting Stop to false.
	// +optional
	Stop *bool `json:"stop,omitempty"`

	// Controls if a job should be restarted.
	// If Restart is set to new timestamp, and
	// - the job is stopped - the job is started again.
	// - the job is running - the job is stopped and started again.
	// This timestamp set to the job's status.restart.
	// +optional
	Restart string `json:"restart,omitempty"`

	// ImageTagName defines the image tag name to use for the job image
	//
	// required: false
	ImageTagName string `json:"imageTagName,omitempty"`
}

// PayloadSecretKeySelector selects a key of a Secret.
type PayloadSecretKeySelector struct {
	// The name of the secret in the RadixBatch namespace to select from.
	LocalObjectReference `json:",inline"`

	// The key of the secret to select from.
	Key string `json:"key"`
}

// RadixDeploymentJobComponentSelector selects a job component of a RadixDeployment.
type RadixDeploymentJobComponentSelector struct {
	// The name of the RadixDeployment in the RadixBatch namespace to select from.
	LocalObjectReference `json:",inline"`

	// The job name of the RadixDeployment to select.
	Job string `json:"job"`
}

// RadixBatchJobPhase represents the phase of the job
// +kubebuilder:validation:Enum=Waiting;Active;Succeeded;Failed;Stopped
type RadixBatchJobPhase string

const (
	// Waiting means that the the job is waiting to start,
	// either because the Kubernetes job has not yet been created,
	// or the Kubernetes job controller has not processed the Job.
	BatchJobPhaseWaiting RadixBatchJobPhase = "Waiting"

	// Active means that the job is active.
	// The Kubernetes job is created, and the Kubernetes job
	// controller has started the job.
	BatchJobPhaseActive RadixBatchJobPhase = "Active"

	// Succeeded means that the job has completed without errors.
	BatchJobPhaseSucceeded RadixBatchJobPhase = "Succeeded"

	// Failed means that the job has failed.
	BatchJobPhaseFailed RadixBatchJobPhase = "Failed"

	// Stopped means that the job has been stopped.
	BatchJobPhaseStopped RadixBatchJobPhase = "Stopped"
)

// +kubebuilder:validation:Enum=Waiting;Active;Completed
type RadixBatchConditionType string

const (
	// Waiting means that all jobs are in phase Waiting.
	BatchConditionTypeWaiting RadixBatchConditionType = "Waiting"

	// Active means that one or more jobs are in phase Active.
	BatchConditionTypeActive RadixBatchConditionType = "Active"

	// Completed means that all jobs are in Succeeded, Failed or Stopped phase.
	BatchConditionTypeCompleted RadixBatchConditionType = "Completed"
)

// RadixBatchCondition describes the state of the RadixBatch
type RadixBatchCondition struct {
	// Type of RadixBatch condition.
	Type RadixBatchConditionType `json:"type"`

	// The reason for the condition.
	// +optional
	Reason string `json:"reason,omitempty"`

	// A human readable message indicating details about the condition.
	// +optional
	Message string `json:"message,omitempty"`

	// The time the condition entered Active state.
	// +optional
	ActiveTime *meta_v1.Time `json:"activeTime,omitempty"`

	// The time the condition entered Completed state.
	// +optional
	CompletionTime *meta_v1.Time `json:"completionTime,omitempty"`
}

// RadixBatchStatus represents the current state of a RadixBatch
type RadixBatchStatus struct {
	// Status for each job defined in spec.jobs
	// +optional
	JobStatuses []RadixBatchJobStatus `json:"jobStatuses,omitempty"`

	// The batch is completed when all jobs are in a completed phase (Succeeded, Failed or Stopped)
	// +optional
	Condition RadixBatchCondition `json:"condition,omitempty"`
}

// RadixBatchJobStatus contains details for the current status of the job.
type RadixBatchJobStatus struct {
	// +kubebuilder:validation:MaxLength:=63
	Name string `json:"name"`

	// The jRadixBatchJob is a simple, high-level summary of where the RadixBatchJob is in its lifecycle.
	Phase RadixBatchJobPhase `json:"phase"`

	// A brief CamelCase message indicating details about why the job is in this phase
	// +optional
	Reason string `json:"reason,omitempty"`

	// A human readable message indicating details about why the job is in this phase
	// +optional
	Message string `json:"message,omitempty"`

	// The time at which the Kubernetes job was created.
	// +optional
	CreationTime *meta_v1.Time `json:"creationTime,omitempty"`

	// The time at which the Kubernetes job was started.
	// +optional
	StartTime *meta_v1.Time `json:"startTime,omitempty"`

	// The time at which the batch job ended.
	// The value is set when phase is either Succeeded, Failed or Stopped.
	// - Succeeded: Value from CompletionTime of the Kubernetes job.
	// - Failed: Value from LastTransitionTime of the Failed condition of the Kubernetes job.
	// - Stopped: The timestamp a job with Stop=true was reonciled.
	// +optional
	EndTime *meta_v1.Time `json:"endTime,omitempty"`

	// The number of times the container for the job has failed.
	// +optional
	Failed int32 `json:"failed,omitempty"`

	// Timestamp of the job restart, if applied.
	// +optional
	Restart string `json:"restart,omitempty"`

	// Status for each pod of the job
	// +optional
	RadixBatchJobPodStatuses []RadixBatchJobPodStatus `json:"podStatuses,omitempty"`
}

// RadixBatchJobPodStatus contains details for the current status of the job's pods.
type RadixBatchJobPodStatus struct {
	// +kubebuilder:validation:MaxLength:=63
	Name string `json:"name"`

	// The phase of a Pod is a simple, high-level summary of where the Pod is in its lifecycle.
	Phase RadixBatchJobPodPhase `json:"phase"`

	// A brief CamelCase message indicating details about why the job is in this phase
	// +optional
	Reason string `json:"reason,omitempty"`

	// A human-readable message indicating details about why the job is in this phase
	// +optional
	Message string `json:"message,omitempty"`

	// Exit status from the last termination of the container
	ExitCode int32 `json:"exitCode"`

	// The time at which the Kubernetes job's pod was created.
	// +optional
	CreationTime *meta_v1.Time `json:"creationTime,omitempty"`

	// The time at which the batch job's pod startedAt
	// +optional
	StartTime *meta_v1.Time `json:"startTime,omitempty"`

	// The time at which the batch job's pod finishedAt.
	// +optional
	EndTime *meta_v1.Time `json:"endTime,omitempty"`
}

// RadixBatchJobPodPhase is a label for the condition of a pod at the current time.
// +enum
type RadixBatchJobPodPhase string

// These are the valid statuses of job's pods.
const (
	// PodPending means the pod has been accepted by the system, but one or more of the containers
	// has not been started. This includes time before being bound to a node, as well as time spent
	// pulling images onto the host.
	PodPending RadixBatchJobPodPhase = "Pending"
	// PodRunning means the pod has been bound to a node and all the containers have been started.
	// At least one container is still running or is in the process of being restarted.
	PodRunning RadixBatchJobPodPhase = "Running"
	// PodSucceeded means that all containers in the pod have voluntarily terminated
	// with a container exit code of 0, and the system is not going to restart any of these containers.
	PodSucceeded RadixBatchJobPodPhase = "Succeeded"
	// PodFailed means that all containers in the pod have terminated, and at least one container has
	// terminated in a failure (exited with a non-zero exit code or was stopped by the system).
	PodFailed RadixBatchJobPodPhase = "Failed"
)

// LocalObjectReference contains enough information to let you locate the
// referenced object inside the same namespace.
type LocalObjectReference struct {
	// Name of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	// +kubebuilder:validation:MaxLength:=253
	Name string `json:"name"`
}
