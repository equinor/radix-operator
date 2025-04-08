package test

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/hash"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateBuildSecret Create a build secret
func CreateBuildSecret(kubeClient kubernetes.Interface, appName string, data map[string][]byte) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: defaults.BuildSecretsName},
		Data:       data,
	}

	_, err := kubeClient.CoreV1().Secrets(utils.GetAppNamespace(appName)).Create(context.Background(), secret, metav1.CreateOptions{})
	return err
}

// GetRadixApplicationHash Get the hash of the radix application
func GetRadixApplicationHash(ra *radixv1.RadixApplication) string {
	if ra == nil {
		hash, _ := hash.ToHashString(hash.SHA256, "0nXSg9l6EUepshGFmolpgV3elB0m8Mv7")
		return hash
	}
	hash, _ := hash.ToHashString(hash.SHA256, ra.Spec)
	return hash
}

// GetBuildSecretHash Get the build secret hash
func GetBuildSecretHash(secret *corev1.Secret) string {
	if secret == nil {
		hash, _ := hash.ToHashString(hash.SHA256, "34Wd68DsJRUzrHp2f63o3U5hUD6zl8Tj")
		return hash
	}
	hash, _ := hash.ToHashString(hash.SHA256, secret.Data)
	return hash
}
