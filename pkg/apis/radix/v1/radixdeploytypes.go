package v1

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixDeployment describe a deployment
type RadixDeployment struct {
	meta_v1.TypeMeta   `json:",inline" yaml:",inline"`
	meta_v1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec               RadixDeploymentSpec `json:"spec" yaml:"spec"`
}

//RadixDeploymentSpec is the spec for a deployment
type RadixDeploymentSpec struct {
	Image string `json:"image" yaml:"image"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

//RadixDeploymentList is a list of Radix deployments
type RadixDeploymentList struct {
	meta_v1.TypeMeta `json:",inline" yaml:",inline"`
	meta_v1.ListMeta `json:"metadata" yaml:"metadata"`
	Items            []RadixDeployment `json:"items" yaml:"items"`
}
