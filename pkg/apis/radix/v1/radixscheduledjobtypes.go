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

// RadixScheduledJobSpec is the spec for a RadixScheduledJob
type RadixScheduledJobSpec struct {
}

// RadixScheduledJobStatus is the status for a RadixScheduledJob
type RadixScheduledJobStatus struct {
	Reconciled *meta_v1.Time `json:"reconciled" yaml:"reconciled"`
}
