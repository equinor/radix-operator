package v1

import meta "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Application",type="string",JSONPath=".spec.appName"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.condition"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:path=radixdnsaliases,scope=Cluster,shortName=rda
// +kubebuilder:subresource:status

// RadixDNSAlias is a Custom Resource Definition
type RadixDNSAlias struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata"`
	Spec            RadixDNSAliasSpec   `json:"spec"`
	Status          RadixDNSAliasStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixDNSAliasList is a list of RadixDNSAliases
type RadixDNSAliasList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata"`
	Items         []RadixDNSAlias `json:"items"`
}

// RadixDNSAliasSpec is the spec for an RadixDNSAlias
type RadixDNSAliasSpec struct {
	// Name of the application the DNS alias used in.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9]*)?[a-z0-9])?$
	AppName string `json:"appName"`

	// Name of the environment for the component.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9]*)?[a-z0-9])?$
	Environment string `json:"environment"`

	// Name of the component that shall receive the incoming requests.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9]*)?[a-z0-9])?$
	Component string `json:"component"`
}

// RadixDNSAliasCondition Holds the condition of a RadixDNSAlias
type RadixDNSAliasCondition string

// These are valid conditions of a deployment.
const (
	// RadixDNSAliasSucceeded means the RadixDNSAlias has been successfully created or updated
	RadixDNSAliasSucceeded RadixDNSAliasCondition = "Succeeded"
	// RadixDNSAliasFailed means the RadixDNSAlias create or update failed
	RadixDNSAliasFailed RadixDNSAliasCondition = "Failed"
)

// RadixDNSAliasStatus is the status for an RadixDNSAlias
type RadixDNSAliasStatus struct {
	// Condition of the RadixDNSAlias creating or updating
	// +optional
	Condition RadixDNSAliasCondition `json:"condition,omitempty"`
	// A human-readable message indicating details about the condition.
	// +optional
	Message string `json:"message,omitempty"`
	// Reconciled The timestamp when the RadixDNSAlias was reconciled
	// +optional
	Reconciled *meta.Time `json:"reconciled,omitempty"`
}
