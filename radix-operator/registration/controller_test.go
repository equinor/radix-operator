package registration

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
)

const (
	clusterName       = "AnyClusterName"
	dnsZone           = "dev.radix.equinor.com"
	containerRegistry = "any.container.registry"
)

var synced chan bool

func setupTest() (kubernetes.Interface, radixclient.Interface) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()

	handlerTestUtils := test.NewTestUtils(client, radixClient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	return client, radixClient
}

func Test_Controller_Calls_Handler(t *testing.T) {
	// Setup
	client, radixClient := setupTest()

	stop := make(chan struct{})
	synced := make(chan bool)

	defer close(stop)
	defer close(synced)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, 0)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, 0)

	registrationHandler := NewHandler(
		client,
		radixClient,
		func(syncedOk bool) {
			synced <- syncedOk
		},
		kubeInformerFactory.Core().V1().Namespaces().Lister(),
	)
	go startRegistrationController(client, radixClient, radixInformerFactory, kubeInformerFactory, registrationHandler, stop)

	// Test

	// Create registration should sync
	registration, err := utils.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	if err != nil {
		log.Fatalf("Could not read configuration data: %v", err)
	}

	registeredApp, err := radixClient.RadixV1().RadixRegistrations().Create(registration)

	assert.NoError(t, err)
	assert.NotNil(t, registeredApp)

	op, ok := <-synced
	assert.True(t, ok)
	assert.True(t, op)

	// Update registration should sync
	registration.ObjectMeta.Annotations = map[string]string{
		"update": "test",
	}
	updatedApp, err := radixClient.RadixV1().RadixRegistrations().Update(registration)

	op, ok = <-synced
	assert.True(t, ok)
	assert.True(t, op)

	assert.NoError(t, err)
	assert.NotNil(t, updatedApp)
	assert.NotNil(t, updatedApp.Annotations)
	assert.Equal(t, "test", updatedApp.Annotations["update"])

	// Delete namespace should sync
	client.CoreV1().Namespaces().Delete(utils.GetAppNamespace("testapp"), &metav1.DeleteOptions{})

	op, ok = <-synced
	assert.True(t, ok)
	assert.True(t, op)
}

func startRegistrationController(
	client kubernetes.Interface,
	radixClient radixclient.Interface,
	radixInformerFactory informers.SharedInformerFactory,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	handler Handler, stop chan struct{}) {

	eventRecorder := &record.FakeRecorder{}

	controller := NewController(client, radixClient, &handler,
		radixInformerFactory.Radix().V1().RadixRegistrations(),
		kubeInformerFactory.Core().V1().Namespaces(), eventRecorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)
	controller.Run(1, stop)

}
