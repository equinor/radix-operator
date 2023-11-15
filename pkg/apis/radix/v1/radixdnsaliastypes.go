package v1

import meta "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=radixdnsaliases,scope=Cluster,shortName=rda

// RadixDNSAlias is a Custom Resource Definition
type RadixDNSAlias struct {
	meta.TypeMeta   `json:",inline" yaml:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec            RadixDNSAliasSpec   `json:"spec" yaml:"spec"`
	Status          RadixDNSAliasStatus `json:"status" yaml:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixDNSAliasList is a list of RadixDNSAliases
type RadixDNSAliasList struct {
	meta.TypeMeta `json:",inline" yaml:",inline"`
	meta.ListMeta `json:"metadata" yaml:"metadata"`
	Items         []RadixDNSAlias `json:"items" yaml:"items"`
}

// RadixDNSAliasSpec is the spec for an RadixDNSAlias
type RadixDNSAliasSpec struct {
	// Name of the application the DNS alias used in.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9]*)?[a-z0-9])?$
	AppName string `json:"appName" yaml:"appName"`

	// Name of the environment for the component.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9]*)?[a-z0-9])?$
	Environment string `json:"environment" yaml:"environment"`

	// Name of the component that shall receive the incoming requests.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9]*)?[a-z0-9])?$
	Component string `json:"component" yaml:"component"`

	// Port number.
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`
}

// RadixDNSAliasStatus is the status for an RadixDNSAlias
type RadixDNSAliasStatus struct {
	// Reconciled *meta.Time `json:"reconciled" yaml:"reconciled"`
}
