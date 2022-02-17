package v1

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixEnvironment is a Custom Resource Definition
type RadixEnvironment struct {
	meta.TypeMeta   `json:",inline" yaml:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec            RadixEnvironmentSpec   `json:"spec" yaml:"spec"`
	Status          RadixEnvironmentStatus `json:"status" yaml:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixEnvironmentList is a list of REs
type RadixEnvironmentList struct {
	meta.TypeMeta `json:",inline" yaml:",inline"`
	meta.ListMeta `json:"metadata" yaml:"metadata"`
	Items         []RadixEnvironment `json:"items" yaml:"items"`
}

//RadixEnvironmentSpec is the spec for an RE
type RadixEnvironmentSpec struct {
	AppName     string       `json:"appName" yaml:"appName"`
	EnvName     string       `json:"envName" yaml:"envName"`
	EgressRules []EgressRule `json:"egressRules,omitempty" yaml:"egressRules,omitempty"`
}

// RadixEnvironmentStatus is the status for an RE
type RadixEnvironmentStatus struct {
	Reconciled meta.Time `json:"reconciled" yaml:"reconciled"`
	Orphaned   bool      `json:"orphaned" yaml:"orphaned"`
}
