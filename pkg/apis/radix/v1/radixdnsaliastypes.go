package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Application",type="string",JSONPath=".spec.appName"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.condition"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:path=radixdnsaliases,scope=Cluster,shortName=rda
// +kubebuilder:subresource:status

// RadixDNSAlias is a Custom Resource Definition
type RadixDNSAlias struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// Spec is the desired state of the RadixDNSAlias
	Spec RadixDNSAliasSpec `json:"spec"`

	// Status is the observed state of the RadixDNSAlias
	// +kubebuilder:validation:Optional
	Status RadixDNSAliasStatus `json:"status,omitzero"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixDNSAliasList is a list of RadixDNSAliases
type RadixDNSAliasList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []RadixDNSAlias `json:"items"`
}

// RadixDNSAliasSpec is the spec for an RadixDNSAlias
type RadixDNSAliasSpec struct {
	// Name of the application the DNS alias used in.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=50
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

type RadixDNSAliasReconcileStatus string

const (
	RadixDNSAliasReconcileSucceeded RadixDNSAliasReconcileStatus = "Succeeded"
	RadixDNSAliasReconcileFailed    RadixDNSAliasReconcileStatus = "Failed"
)

// RadixDNSAliasStatus is the observed state of the RadixDNSAlias
type RadixDNSAliasStatus struct {
	// Reconciled is the timestamp of the last successful reconciliation
	// +kubebuilder:validation:Optional
	Reconciled metav1.Time `json:"reconciled,omitzero"`

	// ObservedGeneration is the generation observed by the controller
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ReconcileStatus indicates whether the last reconciliation succeeded or failed
	// +kubebuilder:validation:Optional
	ReconcileStatus RadixDNSAliasReconcileStatus `json:"reconcileStatus,omitempty"`

	// Message provides additional information about the reconciliation state, typically error details when reconciliation fails
	// +kubebuilder:validation:Optional
	Message string `json:"message,omitempty"`
}
