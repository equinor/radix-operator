package v1

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixApplication describe an application
type RadixApplication struct {
	meta_v1.TypeMeta   `json:",inline" yaml:",inline"`
	meta_v1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec               RadixApplicationSpec `json:"spec" yaml:"spec"`
}

//RadixApplicationSpec is the spec for an application
type RadixApplicationSpec struct {
	Secrets       SecretsMap    `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Project       string        `json:"project" yaml:"project"`
	Repository    string        `json:"repository" yaml:"repository"`
	CloneURL      string        `json:"cloneURL" yaml:"cloneURL"`
	SharedSecret  string        `json:"sharedSecret" yaml:"sharedSecret"`
	SSHKey        string        `json:"sshKey" yaml:"sshKey"`
	DefaultScript string        `json:"defaultScript" yaml:"defaultScript"`
	Environment   []Environment `json:"environment"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

//RadixApplicationList is a list of Radix applications
type RadixApplicationList struct {
	meta_v1.TypeMeta `json:",inline" yaml:",inline"`
	meta_v1.ListMeta `json:"metadata" yaml:"metadata"`
	Items            []RadixApplication `json:"items" yaml:"items"`
}

//SecretsMap is a map of secrets (weird)
type SecretsMap map[string]string

type Environment struct {
	Name          string              `json:"name"`
	Authorization []AuthorizationSpec `json:"authorization"`
}

type AuthorizationSpec struct {
	Role   string   `json:"role"`
	Groups []string `json:"groups"`
}
