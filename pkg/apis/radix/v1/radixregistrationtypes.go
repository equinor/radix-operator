package v1

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixRegistration describe an application
type RadixRegistration struct {
	meta_v1.TypeMeta   `json:",inline" yaml:",inline"`
	meta_v1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec               RadixRegistrationSpec   `json:"spec" yaml:"spec"`
	Status             RadixRegistrationStatus `json:"status" yaml:"status"`
}

// RadixRegistrationStatus is the status for a rr
type RadixRegistrationStatus struct {
	Reconciled meta_v1.Time `json:"reconciled" yaml:"reconciled"`
}

// RadixRegistrationSpec is the spec for an application
type RadixRegistrationSpec struct {
	CloneURL            string   `json:"cloneURL" yaml:"cloneURL"`
	SharedSecret        string   `json:"sharedSecret" yaml:"sharedSecret"`
	DeployKey           string   `json:"deployKey" yaml:"deployKey"`
	DeployKeyPublic     string   `json:"deployKeyPublic" yaml:"deployKeyPublic"`
	AdGroups            []string `json:"adGroups" yaml:"adGroups"`
	ReaderAdGroups      []string `json:"readerAdGroups" yaml:"readerAdGroups"`
	Creator             string   `json:"creator" yaml:"creator"`
	Owner               string   `json:"owner" yaml:"owner"`
	MachineUser         bool     `json:"machineUser" yaml:"machineUser"`
	WBS                 string   `json:"wbs" yaml:"wbs"`
	ConfigBranch        string   `json:"configBranch" yaml:"configBranch"`
	RadixConfigFullName string   `json:"radixConfigFullName" yaml:"radixConfigFullName"`
	// ConfigurationItem is and identifier for an entity in a configuration management solution such as a CMDB.
	// ITIL defines a CI as any component that needs to be managed in order to deliver an IT Service
	// Ref: https://en.wikipedia.org/wiki/Configuration_item
	ConfigurationItem string `json:"configurationItem" yaml:"configurationItem"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixRegistrationList is a list of Radix applications
type RadixRegistrationList struct {
	meta_v1.TypeMeta `json:",inline" yaml:",inline"`
	meta_v1.ListMeta `json:"metadata" yaml:"metadata"`
	Items            []RadixRegistration `json:"items" yaml:"items"`
}
