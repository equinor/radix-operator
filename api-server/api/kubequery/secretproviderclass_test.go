package kubequery

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	secretstorefake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func Test_GetSecretProviderClassesForEnvironment(t *testing.T) {
	matched1 := secretsstorev1.SecretProviderClass{ObjectMeta: metav1.ObjectMeta{Name: "matched1", Namespace: "app1-env1", Labels: map[string]string{kube.RadixAppLabel: "app1", kube.RadixSecretRefTypeLabel: string(radixv1.RadixSecretRefTypeAzureKeyVault)}}}
	matched2 := secretsstorev1.SecretProviderClass{ObjectMeta: metav1.ObjectMeta{Name: "matched2", Namespace: "app1-env1", Labels: map[string]string{kube.RadixAppLabel: "app1", kube.RadixSecretRefTypeLabel: string(radixv1.RadixSecretRefTypeAzureKeyVault)}}}
	unmatched1 := secretsstorev1.SecretProviderClass{ObjectMeta: metav1.ObjectMeta{Name: "unmatched1", Namespace: "app1-env1", Labels: map[string]string{kube.RadixAppLabel: "app2", kube.RadixSecretRefTypeLabel: string(radixv1.RadixSecretRefTypeAzureKeyVault)}}}
	unmatched2 := secretsstorev1.SecretProviderClass{ObjectMeta: metav1.ObjectMeta{Name: "unmatched2", Namespace: "app1-env2", Labels: map[string]string{kube.RadixAppLabel: "app1", kube.RadixSecretRefTypeLabel: string(radixv1.RadixSecretRefTypeAzureKeyVault)}}}
	unmatched3 := secretsstorev1.SecretProviderClass{ObjectMeta: metav1.ObjectMeta{Name: "unmatched3", Namespace: "app1-env1", Labels: map[string]string{kube.RadixAppLabel: "app1", kube.RadixSecretRefTypeLabel: "invalid-type"}}}
	unmatched4 := secretsstorev1.SecretProviderClass{ObjectMeta: metav1.ObjectMeta{Name: "unmatched4", Namespace: "app1-env1", Labels: map[string]string{kube.RadixAppLabel: "app1"}}}
	unmatched5 := secretsstorev1.SecretProviderClass{ObjectMeta: metav1.ObjectMeta{Name: "unmatched5", Namespace: "app1-env1"}}
	client := secretstorefake.NewSimpleClientset(&matched1, &matched2, &unmatched1, &unmatched2, &unmatched3, &unmatched4, &unmatched5)
	expected := []secretsstorev1.SecretProviderClass{matched1, matched2}
	actual, err := GetSecretProviderClassesForEnvironment(context.Background(), client, "app1", "env1")
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, actual)
}
