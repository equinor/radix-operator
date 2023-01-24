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

// RadixBatch describes a job
type RadixBatch struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`

	Spec   RadixBatchSpec   `json:"spec"`
	Status RadixBatchStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixBatchList is a list of RadixBatch
type RadixBatchList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`

	Items []RadixBatch `json:"items"`
}

type RadixBatchSpec struct {
	RadixDeploymentJobRef RadixDeploymentJobComponentSelector `json:"radixDeploymentJobRef"`

	// +listType:=map
	// +listMapKey:=name
	// +kubebuilder:validation:MinItems:=1
	// +kubebuilder:validation:MaxItems:=500
	Jobs []RadixBatchJob `json:"jobs"`
}

// RadixBatchJob defines the spec for a single job in the batch
type RadixBatchJob struct {
	// +kubebuilder:validation:MaxLength:=63
	Name string `json:"name"`

	// +optional
	JobId string `json:"jobId,omitempty"`

	// +optional
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// +optional
	Node *RadixNode `json:"node,omitempty"`

	// +optional
	// +kubebuilder:validation:Minimum:=1
	TimeLimitSeconds *int64 `json:"timeLimitSeconds,omitempty"`

	// +optional
	// +kubebuilder:validation:Minimum:=0
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`

	// +optional
	PayloadSecretRef *PayloadSecretKeySelector `json:"payloadSecretRef,omitempty"`

	// +optional
	Stop *bool `json:"stop,omitempty"`
}

// PayloadSecretKeySelector selects a Secret to get job payload from
type PayloadSecretKeySelector struct {
	// The name of the Secret resource being referred to.
	LocalObjectReference `json:",inline"`

	// The key of the entry in the Secret resource's `data` field to be used.
	Key string `json:"key"`
}

// A reference to a specific job within a RadixDeployment resource.
type RadixDeploymentJobComponentSelector struct {
	// The name of the RadixDeployment resource being referred to
	LocalObjectReference `json:",inline"`

	// The name of the job in the RadixDeployment's `jobs` list to be used
	Job string `json:"job"`
}

// RadixBatchJobPhase represents the phase of the job
// +kubebuilder:validation:Enum=Waiting;Active;Succeeded;Failed;Stopped
type RadixBatchJobPhase string

const (
	// BatchJobPhaseWaiting means that the the job is waiting to start
	// Details about the reason for this state is found in the `reason` field
	BatchJobPhaseWaiting RadixBatchJobPhase = "Waiting"

	// BatchJobPhaseActive means that the job is active
	BatchJobPhaseActive RadixBatchJobPhase = "Active"

	// BatchJobPhaseSucceeded means that the job has completed without errors
	BatchJobPhaseSucceeded RadixBatchJobPhase = "Succeeded"

	// BatchJobPhaseFailed means that the job has failed
	BatchJobPhaseFailed RadixBatchJobPhase = "Failed"

	// BatchJobPhaseStopped means that the job has been stopped
	BatchJobPhaseStopped RadixBatchJobPhase = "Stopped"
)

// +kubebuilder:validation:Enum=Waiting;Active;Completed
type RadixBatchConditionType string

const (
	BatchConditionTypeWaiting   RadixBatchConditionType = "Waiting"
	BatchConditionTypeRunning   RadixBatchConditionType = "Active"
	BatchConditionTypeCompleted RadixBatchConditionType = "Completed"
)

type RadixBatchCondition struct {
	Type RadixBatchConditionType `json:"type"`

	// +optional
	Reason string `json:"reason,omitempty"`

	// +optional
	Message string `json:"message,omitempty"`

	// +optional
	ActiveTime *meta_v1.Time `json:"activeTime,omitempty"`

	// +optional
	CompletedTime *meta_v1.Time `json:"completedTime,omitempty"`
}

// RadixBatchStatus is the status for a RadixBatch
type RadixBatchStatus struct {
	// Status for each job defined in spec.jobs
	// +optional
	JobStatuses []RadixBatchJobStatus `json:"jobStatuses,omitempty"`

	// The batch is completed when all jobs are in a completed phase (Succeeded, Failed or Stopped)
	// +optional
	Condition RadixBatchCondition `json:"condition,omitempty"`
}

// RadixBatchJobStatus is the status for a specific item in spec.items
type RadixBatchJobStatus struct {
	// +kubebuilder:validation:MaxLength:=63
	Name string `json:"name"`

	Phase RadixBatchJobPhase `json:"phase"`

	// A brief CamelCase message indicating details about why the job is in this phase
	// +optional
	Reason string `json:"reason,omitempty"`

	// A human readable message indicating details about why the job is in this phase
	// +optional
	Message string `json:"message,omitempty"`

	// +optional
	CreatedTime *meta_v1.Time `json:"createdTime,omitempty"`

	// +optional
	StartedTime *meta_v1.Time `json:"startedTime,omitempty"`

	// +optional
	EndedTime *meta_v1.Time `json:"endedTime,omitempty"`
}

type LocalObjectReference struct {
	// Name of the resource being referred to.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	// +kubebuilder:validation:MaxLength:=253
	Name string `json:"name"`
}
