package environment

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func Test_Sync_DeletedEnvironment_DeletesPersistentVolumesOfDeletedNamespace(t *testing.T) {
	const envNamespace = "anyapp-anyenv"
	kubeClient := kubefake.NewSimpleClientset()
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck
	kubeUtil, err := kube.New(kubeClient, radixClient, kedafake.NewSimpleClientset(), secretproviderfake.NewSimpleClientset())
	require.NoError(t, err)
	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{
		Name:   "pv-radixvolumemount-any",
		Labels: map[string]string{kube.RadixAppLabel: "anyapp", kube.RadixNamespace: envNamespace, kube.RadixComponentLabel: "web", kube.RadixVolumeMountNameLabel: "blob"},
	}}
	_, err = kubeClient.CoreV1().PersistentVolumes().Create(context.Background(), pv, metav1.CreateOptions{})
	require.NoError(t, err)

	sut := NewHandler(kubeClient, kubeUtil, radixClient, config.Config{}, record.NewFakeRecorder(10))
	require.NoError(t, sut.Sync(context.Background(), "", envNamespace))

	_, err = kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), pv.Name, metav1.GetOptions{})
	assert.Error(t, err, "the PersistentVolume of the deleted environment namespace should be deleted")
}

func Test_getDeletedEnvironmentNamespace(t *testing.T) {
	envNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "anyapp-anyenv", Labels: map[string]string{kube.RadixAppLabel: "anyapp", kube.RadixEnvLabel: "anyenv"}}}
	appNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "anyapp-app", Labels: map[string]string{kube.RadixAppLabel: "anyapp", kube.RadixEnvLabel: "app"}}}
	otherNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other"}}

	tests := map[string]struct {
		obj      any
		expected bool
	}{
		"environment namespace":           {obj: envNamespace, expected: true},
		"environment namespace tombstone": {obj: cache.DeletedFinalStateUnknown{Key: envNamespace.Name, Obj: envNamespace}, expected: true},
		"app namespace":                   {obj: appNamespace},
		"other namespace":                 {obj: otherNamespace},
		"other object":                    {obj: &corev1.Pod{}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			namespace, ok := getDeletedEnvironmentNamespace(test.obj)
			assert.Equal(t, test.expected, ok)
			if test.expected {
				assert.Equal(t, envNamespace.Name, namespace.Name)
			}
		})
	}
}
