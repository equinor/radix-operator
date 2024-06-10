package internal

import (
	"github.com/equinor/radix-operator/pipeline-runner/internal/hash"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
)

// Constants used to generate hash for RadixApplication and BuildSecret if they are nil. Do not change.
const (
	magicValueForNilRadixApplication = "0nXSg9l6EUepshGFmolpgV3elB0m8Mv7"
	magicValueForNilBuildSecretData  = "34Wd68DsJRUzrHp2f63o3U5hUD6zl8Tj"
)

func CreateRadixApplicationHash(ra *radixv1.RadixApplication) (string, error) {
	return hash.ToHashString(hash.SHA256, getRadixApplicationOrMagicValue(ra))
}

func CompareRadixApplicationHash(targetHash string, ra *radixv1.RadixApplication) (bool, error) {
	return hash.CompareWithHashString(getRadixApplicationOrMagicValue(ra), targetHash)
}

func CreateBuildSecretHash(secret *corev1.Secret) (string, error) {
	return hash.ToHashString(hash.SHA256, getBuildSecretOrMagicValue(secret))
}

func CompareBuildSecretHash(targetHash string, secret *corev1.Secret) (bool, error) {
	return hash.CompareWithHashString(getBuildSecretOrMagicValue(secret), targetHash)
}

func getRadixApplicationOrMagicValue(ra *radixv1.RadixApplication) any {
	if ra == nil {
		return magicValueForNilRadixApplication
	}
	return ra.Spec
}

func getBuildSecretOrMagicValue(secret *corev1.Secret) any {
	if secret == nil || len(secret.Data) == 0 {
		return magicValueForNilBuildSecretData
	}
	return secret.Data
}
