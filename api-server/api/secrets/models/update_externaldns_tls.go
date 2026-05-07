package models

// UpdateExternalDNSTLSRequest describes request body for setting private key and certificate for external DNS TLS
// swagger:model UpdateExternalDnsTlsRequest
type UpdateExternalDNSTLSRequest struct {
	// Private key in PEM format
	//
	// required: true
	PrivateKey string `json:"privateKey"`

	// X509 certificate in PEM format
	//
	// required: true
	Certificate string `json:"certificate"`

	// Skip validation of certificate and private key
	//
	// required: false
	SkipValidation bool `json:"skipValidation"`
}
