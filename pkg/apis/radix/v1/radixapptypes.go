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
	Environments []Environment    `json:"environments"`
	Components   []RadixComponent `json:"components"`
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

// EnvVarsMap maps environment variable keys to their values
type EnvVarsMap map[string]string

// EnvVars defines environment and their environment variable values
type EnvVars struct {
	Environment string     `json:"environment" yaml:"environment"`
	Variables   EnvVarsMap `json:"variables" yaml:"variables"`
}

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

// ComponentPort defines the port number, protocol and port for a service
type ComponentPort struct {
	Name string `json:"name"`
	Port int32  `json:"port"`
}

//RadixComponent defines a single component within a RadixApplication - maps to single deployment/service/ingress etc
type RadixComponent struct {
	Name                 string          `json:"name" yaml:"name"`
	SourceFolder         string          `json:"src" yaml:"src"`
	DockerfileName       string          `json:"dockerfileName" yaml:"dockerfileName"`
	Ports                []ComponentPort `json:"ports" yaml:"ports"`
	Public               bool            `json:"public" yaml:"public"`
	Replicas             int             `json:"replicas" yaml:"replicas"`
	EnvironmentVariables []EnvVars       `json:"environmentVariables,omitempty" yaml:"environmentVariables,omitempty"`
	Secrets              []string        `json:"secrets,omitempty" yaml:"secrets,omitempty"`
}
