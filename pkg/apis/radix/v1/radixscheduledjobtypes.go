package v1

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Job",type="string",JSONPath=".spec.radixDeploymentJobRef.job"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state"
// +kubebuilder:resource:path=radixscheduledjobs,shortName=rsj
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

// RadixScheduledJobPhase represents the phase of the job
// +kubebuilder:validation:Enum=Pending;Waiting;Running;Succeeded;Failed;Stopped
type RadixScheduledJobPhase string

const (
	// ScheduledJobPhasePending means that the the Kubernetes job has not been created
	// Details about the reason for this state is found in the `reason` field
	ScheduledJobPhasePending RadixScheduledJobPhase = "Pending"

	// ScheduledJobPhaseWaiting means that the underlying Kubernetes job is created but has not yet started
	ScheduledJobPhaseWaiting RadixScheduledJobPhase = "Waiting"

	// ScheduledJobPhaseRunning means that the job is running
	ScheduledJobPhaseRunning RadixScheduledJobPhase = "Running"

	// ScheduledJobPhaseSucceeded means that the job has completed without errors
	ScheduledJobPhaseSucceeded RadixScheduledJobPhase = "Succeeded"

	// ScheduledJobPhaseFailed means that the job has failed
	ScheduledJobPhaseFailed RadixScheduledJobPhase = "Failed"

	// ScheduledJobPhaseStopped means that the job has been stopped
	ScheduledJobPhaseStopped RadixScheduledJobPhase = "Stopped"
)

// RadixScheduledJobStatus is the status for a RadixScheduledJob
type RadixScheduledJobStatus struct {
	// +optional
	Phase RadixScheduledJobPhase `json:"phase,omitempty"`

	// The reason why the job is in its current phase
	// +optional
	Reason string `json:"reason,omitempty"`

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
