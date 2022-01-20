package utils

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
)

//GetRadixAzureKeyVaultObjectTypePtr Gets pointer to RadixAzureKeyVaultObjectType
func GetRadixAzureKeyVaultObjectTypePtr(objectType v1.RadixAzureKeyVaultObjectType) *v1.RadixAzureKeyVaultObjectType {
	return &objectType
}

//GetAzureKeyVaultTypeSecrets Gets secrets with kube.RadixSecretRefTypeLabel and value v1.RadixSecretRefTypeAzureKeyVault
func GetAzureKeyVaultTypeSecrets(secrets *corev1.SecretList) *corev1.SecretList {
	var azureKeyVaultSecrets []corev1.Secret
	for _, secret := range secrets.Items {
		if label, ok := secret.ObjectMeta.Labels[kube.RadixSecretRefTypeLabel]; ok && label == string(v1.RadixSecretRefTypeAzureKeyVault) {
			azureKeyVaultSecrets = append(azureKeyVaultSecrets, secret)
		}
	}
	return &corev1.SecretList{Items: azureKeyVaultSecrets}
}
