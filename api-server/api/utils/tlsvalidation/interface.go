package tlsvalidation

// Validator defines methods to validate certificate and private key for TLS
type Validator interface {
	// ValidateX509Certificate validates the certificate, dnsName and private key
	// certBytes and keyBytes must be in PEM format
	// Returns false if validation fails, along with a list of validation error messages
	ValidateX509Certificate(certBytes, keyBytes []byte, dnsName string) (bool, []string)
}
