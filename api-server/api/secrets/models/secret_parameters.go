package models

// SecretParameters describes a component secret
// swagger:model SecretParameters
type SecretParameters struct {
	// Name the unique name of the Radix application deployment
	//
	// required: true
	// example: p4$sW0rDz
	SecretValue string `json:"secretValue"`
}
