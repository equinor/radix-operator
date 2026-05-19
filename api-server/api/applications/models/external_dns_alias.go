package models

// DNSExternalAlias holds public external DNS alias information
// swagger:model DNSExternalAlias
type DNSExternalAlias struct {
	// URL the public endpoint
	//
	// required: true
	// example: https://my-app.equinor.com
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
}
