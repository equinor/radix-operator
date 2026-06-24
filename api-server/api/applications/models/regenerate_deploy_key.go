package models

// RegenerateDeployKeyData Holds regenerated shared secret
// swagger:model RegenerateDeployKeyData
type RegenerateDeployKeyData struct {
	// PrivateKey of the deploy key
	//
	// required: false
	PrivateKey string `json:"privateKey"`
}
