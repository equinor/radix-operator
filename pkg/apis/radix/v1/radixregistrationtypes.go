package v1

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixRegistration describe an application
type RadixRegistration struct {
	meta_v1.TypeMeta   `json:",inline" yaml:",inline"`
	meta_v1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec               RadixRegistrationSpec `json:"spec" yaml:"spec"`
}

//RadixRegistrationSpec is the spec for an application
type RadixRegistrationSpec struct {
	Secrets           SecretsMap `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Repository        string     `json:"repository" yaml:"repository"`
	CloneURL          string     `json:"cloneURL" yaml:"cloneURL"`
	SharedSecret      string     `json:"sharedSecret" yaml:"sharedSecret"`
	DeployKey         string     `json:"deployKey" yaml:"deployKey"`
	DefaultScriptName string     `json:"defaultScriptName" yaml:"defaultScriptName"`
	DefaultScript     string     `json:"defaultScript" yaml:"defaultScript"`
	AdGroups          []string   `json:"adGroups" yaml:"adGroups"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

//RadixRegistrationList is a list of Radix applications
type RadixRegistrationList struct {
	meta_v1.TypeMeta `json:",inline" yaml:",inline"`
	meta_v1.ListMeta `json:"metadata" yaml:"metadata"`
	Items            []RadixRegistration `json:"items" yaml:"items"`
}
