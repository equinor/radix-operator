package models

// DNSAlias holds public DNS alias information
// swagger:model DNSAlias
type DNSAlias struct {
	// URL the public endpoint
	//
	// required: true
	// example: https://my-app.radix.equinor.com
	URL string `json:"url"`

	// ComponentName the component exposing the endpoint
	//
	// required: true
	// example: frontend
	ComponentName string `json:"componentName"`

	// EnvironmentName the environment hosting the endpoint
	//
	// required: true
	// example: prod
	EnvironmentName string `json:"environmentName"`

	Status DNSAliasStatus `json:"status,omitempty"`
}

// DNSAliasStatus Status of the DNSAlias
// swagger:model DNSAliasStatus
type DNSAliasStatus struct {
	Condition string `json:"condition,omitempty"`
	Message   string `json:"message,omitempty"`
}
