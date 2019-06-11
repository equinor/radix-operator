package deployment

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

	deploymentHandler := NewDeployHandler(
		client,
		radixClient,
		nil,
		func(syncedOk bool) {
			synced <- syncedOk
		},
	)
	go startDeploymentController(client, radixClient, deploymentHandler, stop)

	// Test

	// Create deployment should sync
	rd, _ := tu.ApplyDeployment(utils.
		ARadixDeployment().
		WithAppName("test-app"))

	op, ok := <-synced
	assert.True(t, ok)
	assert.True(t, op)

	// Update deployment should sync
	radixClient.RadixV1().RadixDeployments(rd.ObjectMeta.Namespace).Update(rd)

	op, ok = <-synced
	assert.True(t, ok)
	assert.True(t, op)

	// Delete service should sync
	services, _ := client.CoreV1().Services(rd.ObjectMeta.Namespace).List(metav1.ListOptions{
		LabelSelector: "radix-app=test-app",
	})

	for _, aservice := range services.Items {
		client.CoreV1().Services(rd.ObjectMeta.Namespace).Delete(aservice.Name, &metav1.DeleteOptions{})

		op, ok = <-synced
		assert.True(t, ok)
		assert.True(t, op)
	}
}

func startDeploymentController(client kubernetes.Interface, radixClient radixclient.Interface, handler RadixDeployHandler, stop chan struct{}) {

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, 0)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, 0)
	eventRecorder := &record.FakeRecorder{}

	controller := NewDeployController(
		client, radixClient, &handler,
		radixInformerFactory.Radix().V1().RadixDeployments(),
		kubeInformerFactory.Core().V1().Services(),
		eventRecorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)
	controller.Run(1, stop)

}
