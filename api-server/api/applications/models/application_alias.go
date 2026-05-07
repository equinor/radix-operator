package models

// ApplicationAlias holds public alias information
// swagger:model ApplicationAlias
type ApplicationAlias struct {
	// URL the public endpoint
	//
	// required: true
	// example: https://my-app.app.radix.equinor.com
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
