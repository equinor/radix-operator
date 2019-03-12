package application

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
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

func setupTest() (*test.Utils, kubernetes.Interface, radixclient.Interface) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()

	handlerTestUtils := test.NewTestUtils(client, radixClient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	return &handlerTestUtils, client, radixClient
}

func Test_Controller_Calls_Handler(t *testing.T) {
	// Setup
	tu, client, radixClient := setupTest()

	stop := make(chan struct{})
	synced := make(chan bool)

	defer close(stop)
	defer close(synced)

	applicationHandler := NewApplicationHandler(
		client,
		radixClient,
		func(syncedOk bool) {
			synced <- syncedOk
		},
	)
	go startApplicationController(client, radixClient, applicationHandler, stop)

	// Test

	// Create registration should sync
	tu.ApplyApplication(utils.
		ARadixApplication().
		WithAppName("test-app").
		WithEnvironment("dev", "master"))

	op, ok := <-synced
	assert.True(t, ok)
	assert.True(t, op)

	// Delete namespace should sync
	client.CoreV1().Namespaces().Delete(utils.GetEnvironmentNamespace("test-app", "dev"), &metav1.DeleteOptions{})

	op, ok = <-synced
	assert.True(t, ok)
	assert.True(t, op)
}

func startApplicationController(client kubernetes.Interface, radixClient radixclient.Interface, handler RadixApplicationHandler, stop chan struct{}) {

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, 0)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, 0)
	eventRecorder := &record.FakeRecorder{}

	controller := NewApplicationController(client, radixClient, &handler,
		radixInformerFactory.Radix().V1().RadixApplications(),
		kubeInformerFactory.Core().V1().Namespaces(), eventRecorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)
	controller.Run(1, stop)

}
