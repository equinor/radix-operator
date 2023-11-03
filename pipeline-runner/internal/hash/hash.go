package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"

	yamlk8s "sigs.k8s.io/yaml"
)

type Algorithm string

const (
	SHA256              Algorithm = "sha256"
	hashStringSeparator string    = "="
)

var hashers map[Algorithm]hash.Hash = map[Algorithm]hash.Hash{}

func register(alg Algorithm, hasher hash.Hash) {
	hashers[alg] = hasher
}

func ToHashString(alg Algorithm, source any) (string, error) {
	hasher, found := hashers[alg]
	if !found {
		return "", fmt.Errorf("hash algorithm %s not found", alg)
	}

	b, err := yamlk8s.Marshal(source)
	if err != nil {
		return "", nil
	}

	hashBytes := hasher.Sum(b)
	return joinToHashString(alg, hex.EncodeToString(hashBytes)), nil
}

func CompareWithHashString(source any, targetHashString string) (bool, error) {
	alg := extractAlgorithmFromHashString(targetHashString)
	sourceHashString, err := ToHashString(alg, source)
	if err != nil {
		return false, err
	}
	return targetHashString == sourceHashString, nil
}

func joinToHashString(alg Algorithm, hash string) string {
	return fmt.Sprintf("%s%s%s", alg, hashStringSeparator, hash)
}

func extractAlgorithmFromHashString(hashString string) Algorithm {
	parts := strings.Split(hashString, hashStringSeparator)
	if len(parts) != 2 {
		return ""
	}
	return Algorithm(parts[0])
}

func init() {
	register(SHA256, sha256.New())
}
