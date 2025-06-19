package v1

import (
	"github.com/oklog/ulid/v2"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="CloneURL",type="string",JSONPath=".spec.cloneURL"
// +kubebuilder:printcolumn:name="ConfigBranch",type="string",JSONPath=".spec.configBranch"
// +kubebuilder:printcolumn:name="RadixConfig",type="string",JSONPath=".spec.radixConfigFullName"
// +kubebuilder:printcolumn:name="Reconciled",type="date",JSONPath=".status.reconciled"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:path=radixregistrations,scope=Cluster,shortName=rr
// +kubebuilder:subresource:status

// RadixRegistration describe an application
type RadixRegistration struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata,omitempty"`
	Spec               RadixRegistrationSpec   `json:"spec"`
	Status             RadixRegistrationStatus `json:"status,omitempty"`
}

// RadixRegistrationStatus is the status for a rr
type RadixRegistrationStatus struct {
	Reconciled meta_v1.Time `json:"reconciled"`
}

// RadixRegistrationSpec is the spec for an application
type RadixRegistrationSpec struct {
	AppID               ULID     `json:"appID,omitempty"`
	CloneURL            string   `json:"cloneURL"`
	SharedSecret        string   `json:"sharedSecret,omitempty"`
	DeployKey           string   `json:"deployKey,omitempty"`
	DeployKeyPublic     string   `json:"deployKeyPublic,omitempty"`
	AdGroups            []string `json:"adGroups,omitempty"`
	AdUsers             []string `json:"adUsers,omitempty"`
	ReaderAdGroups      []string `json:"readerAdGroups,omitempty"`
	ReaderAdUsers       []string `json:"readerAdUsers,omitempty"`
	Creator             string   `json:"creator,omitempty"`
	Owner               string   `json:"owner,omitempty"`
	WBS                 string   `json:"wbs,omitempty"`
	ConfigBranch        string   `json:"configBranch"`
	RadixConfigFullName string   `json:"radixConfigFullName,omitempty"`
	// ConfigurationItem is and identifier for an entity in a configuration management solution such as a CMDB.
	// ITIL defines a CI as any component that needs to be managed in order to deliver an IT Service
	// Ref: https://en.wikipedia.org/wiki/Configuration_item
	ConfigurationItem string `json:"configurationItem,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixRegistrationList is a list of Radix applications
type RadixRegistrationList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []RadixRegistration `json:"items"`
}
