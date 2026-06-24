package models

// EnvVarMetadata Environment variable metadata, holding state of creating or changing of value in Radix console
// swagger:model EnvVarMetadata
type EnvVarMetadata struct {
	// Value of the environment variable in radixconfig.yaml
	//
	// required: false
	// example: value1
	RadixConfigValue string `json:"radixConfigValue"`
}
