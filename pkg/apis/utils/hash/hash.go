package hash

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	v2 "k8s.io/api/core/v1"
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

// Constants used to generate hash for RadixApplication and BuildSecret if they are nil. Do not change.
const (
	magicValueForNilRadixApplication = "0nXSg9l6EUepshGFmolpgV3elB0m8Mv7"
	magicValueForNilBuildSecretData  = "34Wd68DsJRUzrHp2f63o3U5hUD6zl8Tj"
	magicValueForNilVolumeMountData  = "50gfDfg6h65fGfdVfs4r5dhnGFF4D35f"
)

func CreateRadixApplicationHash(ra *v1.RadixApplication) (string, error) {
	return ToHashString(SHA256, getRadixApplicationOrMagicValue(ra))
}

func CompareRadixApplicationHash(targetHash string, ra *v1.RadixApplication) (bool, error) {
	return CompareWithHashString(getRadixApplicationOrMagicValue(ra), targetHash)
}

func CreateBuildSecretHash(secret *v2.Secret) (string, error) {
	return ToHashString(SHA256, getBuildSecretOrMagicValue(secret))
}

func CompareBuildSecretHash(targetHash string, secret *v2.Secret) (bool, error) {
	return CompareWithHashString(getBuildSecretOrMagicValue(secret), targetHash)
}

func getRadixApplicationOrMagicValue(ra *v1.RadixApplication) any {
	if ra == nil {
		return magicValueForNilRadixApplication
	}
	return ra.Spec
}

func getBuildSecretOrMagicValue(secret *v2.Secret) any {
	if secret == nil || len(secret.Data) == 0 {
		return magicValueForNilBuildSecretData
	}
	return secret.Data
}
