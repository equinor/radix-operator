package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"golang.org/x/crypto/ssh"
)

// DeployKey Representation of a deploy key pair
type DeployKey struct {
	PrivateKey string
	PublicKey  string
}

// GenerateDeployKey Generates a deploy key
func GenerateDeployKey() (*DeployKey, error) {
	bitSize := 2048

	privateKey, err := generatePrivateKey(bitSize)
	if err != nil {
		return nil, err
	}

	publicKeyBytes, err := generatePublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}

	privateKeyBytes := encodePrivateKeyToPEM(privateKey)

	return &DeployKey{
		string(privateKeyBytes),
		string(publicKeyBytes),
	}, nil

}

func generatePrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	// Private Key generation
	privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, err
	}

	// Validate Private Key
	err = privateKey.Validate()
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

// encodePrivateKeyToPEM encodes Private Key from RSA to PEM format
func encodePrivateKeyToPEM(privateKey *rsa.PrivateKey) []byte {
	// Get ASN.1 DER format
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)

	// pem.Block
	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}

	// Private key in PEM format
	privatePEM := pem.EncodeToMemory(&privBlock)
	return privatePEM
}

// generatePublicKey takes a rsa.PublicKey and return bytes suitable for writing to .pub file
// returns to the format "ssh-rsa ..."
func generatePublicKey(publicKey *rsa.PublicKey) ([]byte, error) {
	publicRsaKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		return nil, err
	}

	pubKeyBytes := ssh.MarshalAuthorizedKey(publicRsaKey)
	return pubKeyBytes, nil
}

func DeriveDeployKeyFromPrivateKey(privateKey string) (*DeployKey, error) {
	privateKeyBytes := []byte(privateKey)

	sign, err := ssh.ParsePrivateKey(privateKeyBytes)

	if err != nil {
		return nil, err
	}

	publicKeyBytes := ssh.MarshalAuthorizedKey(sign.PublicKey())

	return &DeployKey{
		PrivateKey: privateKey,
		PublicKey:  string(publicKeyBytes),
	}, nil
}
