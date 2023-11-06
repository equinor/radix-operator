package steps

import (
	"github.com/equinor/radix-operator/pipeline-runner/internal/hash"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
)

func createRadixApplicationHash(ra *radixv1.RadixApplication) (string, error) {
	if ra == nil {
		return "", nil
	}
	return hash.ToHashString(hash.SHA256, ra.Spec)
}

func compareRadixApplicationHash(targetHash string, ra *radixv1.RadixApplication) (bool, error) {
	return hash.CompareWithHashString(ra.Spec, targetHash)
}

func createBuildSecretHash(secret *corev1.Secret) (string, error) {
	if secret == nil {
		return "", nil
	}

	return hash.ToHashString(hash.SHA256, secret.Data)
}

func compareBuildSecretHash(targetHash string, secret corev1.Secret) (bool, error) {
	return hash.CompareWithHashString(secret.Data, targetHash)
}
