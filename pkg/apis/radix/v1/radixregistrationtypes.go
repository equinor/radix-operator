package v1

import (
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

	// AppID is the unique identifier for the Radix application. Not to be confused by Configuration Item.
	// +optional
	// +kubebuilder:validation:XValidation:rule=`self == oldSelf`,message="Value is immutable"
	AppID ULID `json:"appID"` // omitempty was configured, but aparently does not work with nested structs

	// CloneURL is the URL of the GitHub repository where the Radix configuration file is located.
	// +required
	// +kubebuilder:validation:Pattern=`^git@github.com:[\w-]+/[\w-]+.git$`
	CloneURL string `json:"cloneURL"`

	// deprecated: SharedSecret is the shared secret for the git repository.
	// +optional
	SharedSecret string `json:"sharedSecret,omitempty"`

	// +optional
	// +kubebuilder:validation:items:Format=uuid
	AdGroups []string `json:"adGroups,omitempty"`

	// +optional
	// +kubebuilder:validation:items:Format=uuid
	AdUsers []string `json:"adUsers,omitempty"`

	// +optional
	// +kubebuilder:validation:items:Format=uuid
	ReaderAdGroups []string `json:"readerAdGroups,omitempty"`

	// +optional
	// +kubebuilder:validation:items:Format=uuid
	ReaderAdUsers []string `json:"readerAdUsers,omitempty"`

	// +optional
	// +kubebuilder:validation:XValidation:rule=`self == oldSelf`,message="Value is immutable"
	Creator string `json:"creator,omitempty"`

	// +optional
	Owner string `json:"owner,omitempty"`

	// ConfigBranch is the branch in the git repository where the Radix configuration file is located.
	// See https://git-scm.com/docs/git-check-ref-format#_description for more details.
	// +required
	// +kubebuilder:validation:XValidation:rule=`!(  self == '@' ||  self == '' )`
	// +kubebuilder:validation:XValidation:rule=`!(  self.startsWith('/') )`
	// +kubebuilder:validation:XValidation:rule=`!(  self.endsWith('.lock') ||  self.endsWith('.')  )`
	// +kubebuilder:validation:XValidation:rule=`!(  self.contains('/.') || self.contains('.lock/') )`
	// +kubebuilder:validation:XValidation:rule=`!(  self.contains('..') ||  self.contains('@{') ||  self.contains('\\')  ||  self.contains('//')  )`
	// +kubebuilder:validation:XValidation:rule=`!(  self.matches('.*[\\x00-\\x1F\\x7F ~^:\\?\\*\\[].*') )`
	ConfigBranch string `json:"configBranch"`

	// RadixConfigFullName is the full name of the Radix configuration file in the git repository.
	// +optional
	// +kubebuilder:validation:Pattern=`^(\/*[a-zA-Z0-9_\.\-]+)+((\.yaml)|(\.yml))$`
	RadixConfigFullName string `json:"radixConfigFullName,omitempty"`

	// ConfigurationItem is and identifier for an entity in a configuration management solution such as a CMDB.
	// TODO: Should be ServiceNow AppID, is currently a mix of name, and CMDB SysID.
	// ITIL defines a CI as any component that needs to be managed in order to deliver an IT Service
	// Ref: https://en.wikipedia.org/wiki/Configuration_item
	// +kubebuilder:validation:MaxLength=100
	ConfigurationItem string `json:"configurationItem,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixRegistrationList is a list of Radix applications
type RadixRegistrationList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []RadixRegistration `json:"items"`
}
