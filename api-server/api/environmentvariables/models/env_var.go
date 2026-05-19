package models

// EnvVar environment variable with metadata
// swagger:model EnvVar
type EnvVar struct {
	// Name of the environment variable
	//
	// required: true
	// example: VAR1
	Name string `json:"name"`

	// Value of the environment variable
	//
	// required: false
	// example: value1
	Value string `json:"value"`

	// Metadata for the environment variable
	//
	// required: false
	Metadata *EnvVarMetadata `json:"metadata,omitempty"`
}
