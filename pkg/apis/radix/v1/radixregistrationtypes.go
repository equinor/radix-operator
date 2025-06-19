package v1

import (
	"github.com/oklog/ulid/v2"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:webhook:verbs=create;update,groups=radix.equinor.com,resources=radixregistrations,versions=v1,name=radixregistrations.validation.radix.equinor.com,path=/api/v1/radixregistration/validator,webhookVersions=v1,matchPolicy=equivalent,mutating=false,failurePolicy=fail,sideEffects=None,admissionReviewVersions=v1

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
	AppID               ulid.ULID `json:"appID" yaml:"appID"`
	CloneURL            string    `json:"cloneURL" yaml:"cloneURL"`
	SharedSecret        string    `json:"sharedSecret" yaml:"sharedSecret"`
	DeployKey           string    `json:"deployKey" yaml:"deployKey"`
	DeployKeyPublic     string    `json:"deployKeyPublic" yaml:"deployKeyPublic"`
	AdGroups            []string  `json:"adGroups" yaml:"adGroups"`
	AdUsers             []string  `json:"adUsers" yaml:"adUsers"`
	ReaderAdGroups      []string  `json:"readerAdGroups" yaml:"readerAdGroups"`
	ReaderAdUsers       []string  `json:"readerAdUsers" yaml:"readerAdUsers"`
	Creator             string    `json:"creator" yaml:"creator"`
	Owner               string    `json:"owner" yaml:"owner"`
	WBS                 string    `json:"wbs" yaml:"wbs"`
	ConfigBranch        string    `json:"configBranch" yaml:"configBranch"`
	RadixConfigFullName string    `json:"radixConfigFullName" yaml:"radixConfigFullName"`
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
