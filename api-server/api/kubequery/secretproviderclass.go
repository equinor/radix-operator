package kubequery

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

// GetSecretProviderClassesForEnvironment returns all SecretProviderClasses for the specified application and environment.
func GetSecretProviderClassesForEnvironment(ctx context.Context, client secretProviderClient.Interface, appName, envName string) ([]secretsstorev1.SecretProviderClass, error) {
	ns := operatorutils.GetEnvironmentNamespace(appName, envName)
	labelSelector := labels.Set{
		kube.RadixAppLabel:           appName,
		kube.RadixSecretRefTypeLabel: string(radixv1.RadixSecretRefTypeAzureKeyVault),
	}
	spcList, err := client.SecretsstoreV1().SecretProviderClasses(ns).List(ctx, v1.ListOptions{LabelSelector: labelSelector.String()})
	if err != nil {
		return nil, err
	}
	return spcList.Items, nil
}
