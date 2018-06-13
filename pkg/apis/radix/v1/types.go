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
	Secrets      SecretsMap `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Repository   string     `json:"repository" yaml:"repository"`
	CloneURL     string     `json:"cloneURL" yaml:"cloneURL"`
	SharedSecret string     `json:"sharedSecret" yaml:"sharedSecret"`
	DeployKey    string     `json:"deployKey" yaml:"deployKey"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

//RadixRegistrationList is a list of Radix applications
type RadixRegistrationList struct {
	meta_v1.TypeMeta `json:",inline" yaml:",inline"`
	meta_v1.ListMeta `json:"metadata" yaml:"metadata"`
	Items            []RadixRegistration `json:"items" yaml:"items"`
}

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
	Secrets     SecretsMap       `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Environment []Environment    `json:"environment"`
	Components  []RadixComponent `json:"components"`
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

//Environment defines a Radix application environment
type Environment struct {
	Name          string              `json:"name" yaml:"name"`
	Authorization []AuthorizationSpec `json:"authorization" yaml:"authorization"`
}

//AuthorizationSpec maps Azure AD groups to roles
type AuthorizationSpec struct {
	Role   string   `json:"role" yaml:"role"`
	Groups []string `json:"groups" yaml:"groups"`
}

//RadixComponent defines a single component within a RadixApplication - maps to single deployment/service/ingress etc
type RadixComponent struct {
	Name                 string            `json:"name" yaml:"name"`
	SourceFolder         string            `json:"src" yaml:"src"`
	Ports                []int             `json:"ports" yaml:"ports"`
	Public               bool              `json:"public" yaml:"public"`
	EnvironmentVariables map[string]string `json:"env" yaml:"env"`
}
