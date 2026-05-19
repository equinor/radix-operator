package models

// EnvVarParameter describes an environment variable
// swagger:model EnvVarParameter
type EnvVarParameter struct {
	// Name of the environment variable
	//
	// required: true
	// example: VAR1
	Name string `json:"name"`

	// Value a new value of the environment variable
	//
	// required: true
	// example: value1
	Value string `json:"value"`
}
