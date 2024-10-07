package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixEnvironment is a Custom Resource Definition
type RadixEnvironment struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec              RadixEnvironmentSpec   `json:"spec" yaml:"spec"`
	Status            RadixEnvironmentStatus `json:"status" yaml:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixEnvironmentList is a list of REs
type RadixEnvironmentList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata" yaml:"metadata"`
	Items           []RadixEnvironment `json:"items" yaml:"items"`
}

// RadixEnvironmentSpec is the spec for an RE
type RadixEnvironmentSpec struct {
	AppName string       `json:"appName" yaml:"appName"`
	EnvName string       `json:"envName" yaml:"envName"`
	Egress  EgressConfig `json:"egress,omitempty" yaml:"egress,omitempty"`
}

// RadixEnvironmentStatus is the status for an RE
type RadixEnvironmentStatus struct {
	Reconciled metav1.Time `json:"reconciled" yaml:"reconciled"`
	Orphaned   bool        `json:"orphaned" yaml:"orphaned"`
	// OrphanedTimestamp is a timestamp representing the server time when this RadixEnvironment was removed from the RadixApplication
	OrphanedTimestamp *metav1.Time `json:"orphanedTimestamp" yaml:"orphanedTimestamp"`
}
