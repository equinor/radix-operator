package hash

import (
	"encoding/hex"
	"fmt"
	"strings"
)

type Algorithm string

const (
	hashStringSeparator string = "="
)

type sumFunc func(in []byte) []byte

var sumProviders map[Algorithm]sumFunc = map[Algorithm]sumFunc{}

func registerProvider(alg Algorithm, provider sumFunc) {
	sumProviders[alg] = provider
}

func ToHashString(alg Algorithm, source any) (string, error) {
	sum, found := sumProviders[alg]
	if !found {
		return "", fmt.Errorf("hash algorithm %s not found", alg)
	}

	encode, err := getEncoder(source)
	if err != nil {
		return "", err
	}

	b, err := encode(source)
	if err != nil {
		return "", err
	}

	hashBytes := sum(b)
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
