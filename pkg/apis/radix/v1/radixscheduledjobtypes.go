package v1

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixScheduledJob describes a job
type RadixScheduledJob struct {
	meta_v1.TypeMeta   `json:",inline" yaml:",inline"`
	meta_v1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec               RadixScheduledJobSpec   `json:"spec" yaml:"spec"`
	Status             RadixScheduledJobStatus `json:"status" yaml:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixScheduledJobList is a list of RadixScheduledJob
type RadixScheduledJobList struct {
	meta_v1.TypeMeta `json:",inline" yaml:",inline"`
	meta_v1.ListMeta `json:"metadata" yaml:"metadata"`
	Items            []RadixScheduledJob `json:"items" yaml:"items"`
}

type LocalObjectReference struct {
	// Name of the resource being referred to.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	Name string `json:"name"`
}

// RadixScheduledJobSpec is the spec for a RadixScheduledJob
type RadixScheduledJobSpec struct {
	JobId                 string                              `json:"jobId,omitempty"`
	Resources             *ResourceRequirements               `json:"resources,omitempty"`
	Node                  *RadixNode                          `json:"node,omitempty"`
	TimeLimitSeconds      *int64                              `json:"timeLimitSeconds,omitempty"`
	RadixDeploymentJobRef RadixDeploymentJobComponentSelector `json:"radixDeploymentJobRef"`
	PayloadSecretRef      *PayloadSecretKeySelector           `json:"payloadSecretRef,omitempty"`
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

	Job string `json:"job"`
}

// RadixScheduledJobStatus is the status for a RadixScheduledJob
type RadixScheduledJobStatus struct {
	Reconciled *meta_v1.Time `json:"reconciled" yaml:"reconciled"`
}
