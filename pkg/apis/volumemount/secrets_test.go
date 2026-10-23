package volumemount

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func Test_GarbageCollectVolumeMountsSecretsNoLongerInSpecForComponent(t *testing.T) {
	const namespace = "anyapp-anyenv"
	kubeClient := fake.NewSimpleClientset()
	kubeUtil, err := kube.New(kubeClient, radixfake.NewSimpleClientset(), kedafake.NewSimpleClientset(), secretproviderfake.NewSimpleClientset()) //nolint:staticcheck
	require.NoError(t, err)
	secret := func(name, component, mountType string) *corev1.Secret {
		return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: map[string]string{
			kube.RadixComponentLabel: component, kube.RadixMountTypeLabel: mountType,
		}}}
	}
	secrets := []*corev1.Secret{
		secret("legacy-azure-blob-creds", "web", "azure-blob"),
		secret("removed-blobfuse2-creds", "web", string(radixv1.MountTypeBlobFuse2Fuse2CsiAzure)),
		secret("current-blobfuse2-creds", "web", string(radixv1.MountTypeBlobFuse2Fuse2CsiAzure)),
		secret("other-component-creds", "other", "azure-blob"),
	}
	for _, s := range secrets {
		_, err := kubeClient.CoreV1().Secrets(namespace).Create(context.Background(), s, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	err = GarbageCollectVolumeMountsSecretsNoLongerInSpecForComponent(context.Background(), kubeUtil, namespace, &radixv1.RadixDeployComponent{Name: "web"}, []string{"current-blobfuse2-creds"})
	require.NoError(t, err)

	exists := func(name string) bool {
		_, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
		return err == nil
	}
	assert.False(t, exists("legacy-azure-blob-creds"), "credentials of a removed legacy azure-blob volume mount should be deleted")
	assert.False(t, exists("removed-blobfuse2-creds"))
	assert.True(t, exists("current-blobfuse2-creds"))
	assert.True(t, exists("other-component-creds"))
}
