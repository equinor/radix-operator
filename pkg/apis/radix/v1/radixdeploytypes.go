package v1

import (
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixDeployment describe a deployment
type RadixDeployment struct {
	meta_v1.TypeMeta   `json:",inline" yaml:",inline"`
	meta_v1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec               RadixDeploymentSpec `json:"spec" yaml:"spec"`
	Status             RadixDeployStatus   `json:"status" yaml:"status"`
}

//RadixDeployStatus is the status for a rd
type RadixDeployStatus struct {
	ActiveFrom meta_v1.Time         `json:"activeFrom" yaml:"activeFrom"`
	ActiveTo   meta_v1.Time         `json:"activeTo" yaml:"activeTo"`
	Condition  RadixDeployCondition `json:"condition" yaml:"condition"`
	Reconciled meta_v1.Time         `json:"reconciled" yaml:"reconciled"`
}

// RadixDeployCondition Holds the condition of a component
type RadixDeployCondition string

// These are valid conditions of a deployment.
const (
	// Active means the radix deployment is active and should be consolidated
	DeploymentActive RadixDeployCondition = "Active"
	// Inactive means radix deployment is inactive and should not be consolidated
	DeploymentInactive RadixDeployCondition = "Inactive"
)

//RadixDeploymentSpec is the spec for a deployment
type RadixDeploymentSpec struct {
	AppName          string                         `json:"appname" yaml:"appname"`
	Components       []RadixDeployComponent         `json:"components"`
	Environment      string                         `json:"environment" yaml:"environment"`
	ImagePullSecrets []core_v1.LocalObjectReference `json:"imagePullSecrets" yaml:"imagePullSecrets"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

//RadixDeploymentList is a list of Radix deployments
type RadixDeploymentList struct {
	meta_v1.TypeMeta `json:",inline" yaml:",inline"`
	meta_v1.ListMeta `json:"metadata" yaml:"metadata"`
	Items            []RadixDeployment `json:"items" yaml:"items"`
}

//RadixDeployComponent defines a single component within a RadixDeployment - maps to single deployment/service/ingress etc
type RadixDeployComponent struct {
	Name     string          `json:"name" yaml:"name"`
	Image    string          `json:"image" yaml:"image"`
	Ports    []ComponentPort `json:"ports" yaml:"ports"`
	Replicas *int            `json:"replicas" yaml:"replicas"`
	// Deprecated: For backwards comptibility Public is still supported, new code should use PublicPort instead
	Public               bool                    `json:"public" yaml:"public"`
	PublicPort           string                  `json:"publicPort,omitempty" yaml:"publicPort,omitempty"`
	EnvironmentVariables EnvVarsMap              `json:"environmentVariables,omitempty" yaml:"environmentVariables,omitempty"`
	Secrets              []string                `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	IngressConfiguration []string                `json:"ingressConfiguration,omitempty" yaml:"ingressConfiguration,omitempty"`
	DNSAppAlias          bool                    `json:"dnsAppAlias,omitempty" yaml:"dnsAppAlias,omitempty"`
	DNSExternalAlias     []string                `json:"dnsExternalAlias,omitempty" yaml:"dnsExternalAlias,omitempty"`
	Monitoring           bool                    `json:"monitoring" yaml:"monitoring"`
	Resources            ResourceRequirements    `json:"resources,omitempty" yaml:"resources,omitempty"`
	HorizontalScaling    *RadixHorizontalScaling `json:"horizontalScaling,omitempty" yaml:"horizontalScaling,omitempty"`
}
