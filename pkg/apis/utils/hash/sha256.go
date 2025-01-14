package hash

import "crypto/sha256"

const (
	SHA256 Algorithm = "sha256"
)

func sum256(in []byte) []byte {
	hashBytes := sha256.Sum256(in)
	return hashBytes[:]
}

func init() {
	registerProvider(SHA256, sum256)
}
