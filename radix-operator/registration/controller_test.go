package registration

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func setupTest() (kubernetes.Interface, *kube.Kube, radixclient.Interface) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(client, radixClient, secretproviderclient)
	handlerTestUtils := test.NewTestUtils(client, radixClient, secretproviderclient)
	handlerTestUtils.CreateClusterPrerequisites("AnyClusterName", "0.0.0.0", "anysubid")
	return client, kubeUtil, radixClient
}

func Test_Controller_Calls_Handler(t *testing.T) {
	// Setup
	client, kubeUtil, radixClient := setupTest()

	stop := make(chan struct{})
	synced := make(chan bool)

	defer close(stop)
	defer close(synced)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, 0)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, 0)

	registrationHandler := NewHandler(
		client,
		kubeUtil,
		radixClient,
		func(syncedOk bool) {
			synced <- syncedOk
		},
	)
	go func() {
		err := startRegistrationController(client, radixClient, radixInformerFactory, kubeInformerFactory, registrationHandler, stop)
		require.NoError(t, err)
	}()

	// Test

	// Create registration should sync
	registration, err := utils.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	if err != nil {
		log.Fatalf("Could not read configuration data: %v", err)
	}

	registeredApp, err := radixClient.RadixV1().RadixRegistrations().Create(context.TODO(), registration, metav1.CreateOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, registeredApp)

	op, ok := <-synced
	assert.True(t, ok)
	assert.True(t, op)

	syncedRr, _ := radixClient.RadixV1().RadixRegistrations().Get(context.TODO(), registration.GetName(), metav1.GetOptions{})
	lastReconciled := syncedRr.Status.Reconciled
	assert.Truef(t, !lastReconciled.Time.IsZero(), "Reconciled on status should have been set")

	// Update registration should sync
	registration.ObjectMeta.Annotations = map[string]string{
		"update": "test",
	}
	updatedApp, err := radixClient.RadixV1().RadixRegistrations().Update(context.TODO(), registration, metav1.UpdateOptions{})

	op, ok = <-synced
	assert.True(t, ok)
	assert.True(t, op)

	assert.NoError(t, err)
	assert.NotNil(t, updatedApp)
	assert.NotNil(t, updatedApp.Annotations)
	assert.Equal(t, "test", updatedApp.Annotations["update"])

	// Delete namespace should sync
	err = client.CoreV1().Namespaces().Delete(context.TODO(), utils.GetAppNamespace("testapp"), metav1.DeleteOptions{})
	assert.NoError(t, err)

	op, ok = <-synced
	assert.True(t, ok)
	assert.True(t, op)

	// Delete private key secret should sync
	err = client.CoreV1().Secrets(utils.GetAppNamespace("testapp")).Delete(context.TODO(), defaults.GitPrivateKeySecretName, metav1.DeleteOptions{})
	assert.NoError(t, err)

	op, ok = <-synced
	assert.True(t, ok)
	assert.True(t, op)

	// Update private key secret should sync
	existingSecret, err := client.CoreV1().Secrets(utils.GetAppNamespace("testapp")).Get(context.TODO(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	deployKey, err := utils.GenerateDeployKey()
	assert.NoError(t, err)
	existingSecret.Data[defaults.GitPrivateKeySecretKey] = []byte(deployKey.PrivateKey)
	newSecret := existingSecret.DeepCopy()
	newSecret.ResourceVersion = "1"
	_, err = client.CoreV1().Secrets(utils.GetAppNamespace("testapp")).Update(context.TODO(), newSecret, metav1.UpdateOptions{})
	assert.NoError(t, err)

	op, ok = <-synced
	assert.True(t, ok)
	assert.True(t, op)
}

func startRegistrationController(client kubernetes.Interface, radixClient radixclient.Interface, radixInformerFactory informers.SharedInformerFactory, kubeInformerFactory kubeinformers.SharedInformerFactory, handler Handler, stop chan struct{}) error {

	eventRecorder := &record.FakeRecorder{}

	const waitForChildrenToSync = false
	controller := NewController(client, radixClient, &handler,
		kubeInformerFactory, radixInformerFactory, waitForChildrenToSync, eventRecorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)
	return controller.Run(5, stop)
}
