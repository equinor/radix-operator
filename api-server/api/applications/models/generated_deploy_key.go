package models

// DeployKeyAndSecret Holds generated public deploy key and shared secret
// swagger:model DeployKeyAndSecret
type DeployKeyAndSecret struct {
	// PublicDeployKey the public value of the deploy key
	//
	// required: true
	PublicDeployKey string `json:"publicDeployKey"`

	// SharedSecret the shared secret
	//
	// required: true
	SharedSecret string `json:"sharedSecret"`
}
