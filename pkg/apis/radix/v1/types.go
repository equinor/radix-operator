package v1

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixApplication describe an application
type RadixApplication struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata,omitempty"`
	Spec               RadixApplicationSpec `json:"spec"`
}

//RadixApplicationSpec is the spec for an application
type RadixApplicationSpec struct {
	Secrets       SecretsMap `json:"secrets,omitempty"`
	Project       string     `json:"project"`
	Repository    string     `json:"repository"`
	CloneURL      string     `json:"cloneURL"`
	SharedSecret  string     `json:"sharedSecret"`
	SshKey        string     `json:"sshKey"`
	DefaultScript string     `json:"defaultScript"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

//RadixApplicationList is a list of Radix applications
type RadixApplicationList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []RadixApplication `json:"items"`
}

type SecretsMap map[string]string
