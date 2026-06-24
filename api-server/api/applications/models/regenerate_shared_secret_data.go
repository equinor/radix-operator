package models

// RegenerateSharedSecretData Holds regenerated shared secret
// swagger:model RegenerateSharedSecretData
type RegenerateSharedSecretData struct {
	// SharedSecret of the shared secret
	//
	// required: false
	SharedSecret string `json:"sharedSecret"`
}
