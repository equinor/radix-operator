package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=radixenvironments,scope=Cluster,shortName=re
// +kubebuilder:subresource:status

// RadixEnvironment is a Custom Resource Definition
type RadixEnvironment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// Spec is the desired state of the RadixEnvironment
	Spec RadixEnvironmentSpec `json:"spec"`

	// Status is the observed state of the RadixEnvironment
	// +kubebuilder:validation:Optional
	Status RadixEnvironmentStatus `json:"status,omitzero"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixEnvironmentList is a list of REs
type RadixEnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []RadixEnvironment `json:"items"`
}

// RadixEnvironmentSpec is the spec for an RE
type RadixEnvironmentSpec struct {
	AppName string `json:"appName"`
	EnvName string `json:"envName"`

	// Egress defines egress rules
	// +kubebuilder:validation:Optional
	Egress EgressConfig `json:"egress,omitzero"`
}

type RadixEnvironmentReconcileStatus string

const (
	RadixEnvironmentReconcileSucceeded RadixEnvironmentReconcileStatus = "Succeeded"
	RadixEnvironmentReconcileFailed    RadixEnvironmentReconcileStatus = "Failed"
)

// RadixEnvironmentStatus is the observed state of the RadixEnvironment
type RadixEnvironmentStatus struct {
	// Orphaned is true when this environment was removed from the RadixApplication
	Orphaned bool `json:"orphaned"`

	// OrphanedTimestamp is a timestamp representing the server time when this RadixEnvironment was removed from the RadixApplication
	OrphanedTimestamp *metav1.Time `json:"orphanedTimestamp,omitempty"`

	// Reconciled is the timestamp of the last successful reconciliation
	// +kubebuilder:validation:Optional
	Reconciled metav1.Time `json:"reconciled,omitzero"`

	// ObservedGeneration is the generation observed by the controller
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ReconcileStatus indicates whether the last reconciliation succeeded or failed
	// +kubebuilder:validation:Optional
	ReconcileStatus RadixAlertReconcileStatus `json:"reconcileStatus,omitempty"`

	// Message provides additional information about the reconciliation state, typically error details when reconciliation fails
	// +kubebuilder:validation:Optional
	Message string `json:"message,omitempty"`
}
