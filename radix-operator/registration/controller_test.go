package registration

import (
	"context"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func setupTest(t *testing.T) (kubernetes.Interface, *kube.Kube, radixclient.Interface) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(client, radixClient, secretproviderclient)
	handlerTestUtils := test.NewTestUtils(client, radixClient, secretproviderclient)
	err := handlerTestUtils.CreateClusterPrerequisites("AnyClusterName", "0.0.0.0", "anysubid")
	require.NoError(t, err)
	return client, kubeUtil, radixClient
}

func Test_Controller_Calls_Handler(t *testing.T) {
	// Setup
	client, kubeUtil, radixClient := setupTest(t)

	ctx, stopFn := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopFn()

	synced := make(chan bool)
	defer close(synced)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, 0)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, 0)

	// Create registration should sync
	registration, err := utils.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	require.NoError(t, err)
	registeredApp, err := radixClient.RadixV1().RadixRegistrations().Create(ctx, registration, metav1.CreateOptions{})
	require.NoError(t, err)
	require.NotNil(t, registeredApp)

	registrationHandler := NewHandler(
		client,
		kubeUtil,
		radixClient,
		func(syncedOk bool) {
			synced <- syncedOk
		},
	)
	go func() {
		err := startRegistrationController(ctx, client, radixClient, radixInformerFactory, kubeInformerFactory, registrationHandler)
		require.NoError(t, err)
	}()

	// Test
	select {
	case op, ok := <-synced:
		assert.True(t, op)
		assert.True(t, ok)
	case <-ctx.Done():
		require.NoError(t, ctx.Err())
	}

	syncedRr, _ := radixClient.RadixV1().RadixRegistrations().Get(ctx, registration.GetName(), metav1.GetOptions{})
	lastReconciled := syncedRr.Status.Reconciled
	assert.Truef(t, !lastReconciled.Time.IsZero(), "Reconciled on status should have been set")

	// Update registration should sync
	registration.ObjectMeta.Annotations = map[string]string{
		"update": "test",
	}
	updatedApp, err := radixClient.RadixV1().RadixRegistrations().Update(ctx, registration, metav1.UpdateOptions{})
	require.NoError(t, err)
	select {
	case op, ok := <-synced:
		assert.True(t, op)
		assert.True(t, ok)
	case <-ctx.Done():
		require.NoError(t, ctx.Err())
	}

	assert.NotNil(t, updatedApp)
	assert.NotNil(t, updatedApp.Annotations)
	assert.Equal(t, "test", updatedApp.Annotations["update"])

	// Delete namespace should sync
	err = client.CoreV1().Namespaces().Delete(ctx, utils.GetAppNamespace("testapp"), metav1.DeleteOptions{})
	require.NoError(t, err)

	// Delete private key secret should sync
	err = client.CoreV1().Secrets(utils.GetAppNamespace("testapp")).Delete(ctx, defaults.GitPrivateKeySecretName, metav1.DeleteOptions{})
	require.NoError(t, err)
	select {
	case op, ok := <-synced:
		assert.True(t, op)
		assert.True(t, ok)
	case <-ctx.Done():
		require.NoError(t, ctx.Err())
	}

	// Update private key secret should sync
	existingSecret, err := client.CoreV1().Secrets(utils.GetAppNamespace("testapp")).Get(ctx, defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	require.NoError(t, err)
	deployKey, err := utils.GenerateDeployKey()
	require.NoError(t, err)
	existingSecret.Data[defaults.GitPrivateKeySecretKey] = []byte(deployKey.PrivateKey)
	newSecret := existingSecret.DeepCopy()
	newSecret.ResourceVersion = "1"
	_, err = client.CoreV1().Secrets(utils.GetAppNamespace("testapp")).Update(ctx, newSecret, metav1.UpdateOptions{})
	require.NoError(t, err)
	select {
	case op, ok := <-synced:
		assert.True(t, op)
		assert.True(t, ok)
	case <-ctx.Done():
		require.NoError(t, ctx.Err())
	}
}

func startRegistrationController(ctx context.Context, client kubernetes.Interface, radixClient radixclient.Interface, radixInformerFactory informers.SharedInformerFactory, kubeInformerFactory kubeinformers.SharedInformerFactory, handler Handler) error {

	eventRecorder := &record.FakeRecorder{}

	const waitForChildrenToSync = false
	controller := NewController(ctx, client, radixClient, &handler, kubeInformerFactory, radixInformerFactory, waitForChildrenToSync, eventRecorder)

	kubeInformerFactory.Start(ctx.Done())
	radixInformerFactory.Start(ctx.Done())
	return controller.Run(ctx, 5)
}
