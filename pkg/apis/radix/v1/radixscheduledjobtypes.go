package v1

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:printcolumn:name="Job",type="string",JSONPath=".spec.radixDeploymentJobRef.job"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state"
// +kubebuilder:resource:path=radixscheduledjobs
// +kubebuilder:subresource:status

// RadixScheduledJob describes a job
type RadixScheduledJob struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`

	Spec   RadixScheduledJobSpec   `json:"spec"`
	Status RadixScheduledJobStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixScheduledJobList is a list of RadixScheduledJob
type RadixScheduledJobList struct {
	meta_v1.TypeMeta `json:",inline" yaml:",inline"`
	meta_v1.ListMeta `json:"metadata" yaml:"metadata"`

	Items []RadixScheduledJob `json:"items" yaml:"items"`
}

// RadixScheduledJobSpec is the spec for a RadixScheduledJob
type RadixScheduledJobSpec struct {

	// +optional
	JobId string `json:"jobId,omitempty"`

	// +optional
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// +optional
	Node *RadixNode `json:"node,omitempty"`

	// +optional
	TimeLimitSeconds *int64 `json:"timeLimitSeconds,omitempty"`

	RadixDeploymentJobRef RadixDeploymentJobComponentSelector `json:"radixDeploymentJobRef"`

	// +optional
	PayloadSecretRef *PayloadSecretKeySelector `json:"payloadSecretRef,omitempty"`

	// +optional
	Stop *bool `json:"bool,omitempty"`
}

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

// RadixScheduledJobState represents the state of the job
// +kubebuilder:validation:Enum=pending;running;completed;failed;stopped
type RadixScheduledJobState string

const (
	// ScheduledJobStatePending means that the underlying Kubernetes job is created but not yet running
	ScheduledJobStatePending = "pending"
	// ScheduledJobStateRunning means that the job is running
	ScheduledJobStateRunning = "running"
	// ScheduledJobStateCompleted means that the job has completed without errors
	ScheduledJobStateCompleted = "completed"
	// ScheduledJobStateFailed means that the job has failed
	ScheduledJobStateFailed = "failed"
	// ScheduledJobStateStopped means that the job has been stopped
	ScheduledJobStateStopped = "stopped"
)

// RadixScheduledJobStatus is the status for a RadixScheduledJob
type RadixScheduledJobStatus struct {
	// +optional
	State RadixScheduledJobState `json:"state,omitempty"`
	// +optional
	Created *meta_v1.Time `json:"created,omitempty"`
	// +optional
	Started *meta_v1.Time `json:"started,omitempty"`
	// +optional
	Ended *meta_v1.Time `json:"ended,omitempty"`
}

type LocalObjectReference struct {
	// Name of the resource being referred to.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	Name string `json:"name"`
}
