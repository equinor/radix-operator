package application

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
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
	anyAppName := "test-app"

	// Setup
	tu, client, radixClient := setupTest()

	client.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.GetAppNamespace(anyAppName),
			Labels: map[string]string{
				kube.RadixAppLabel: anyAppName,
				kube.RadixEnvLabel: "app",
			},
		},
	})

	stop := make(chan struct{})
	synced := make(chan bool)

	defer close(stop)
	defer close(synced)

	applicationHandler := NewHandler(
		client,
		radixClient,
		func(syncedOk bool) {
			synced <- syncedOk
		},
	)
	go startApplicationController(client, radixClient, applicationHandler, stop)

	// Test

	// Create registration should sync
	tu.ApplyApplication(
		utils.ARadixApplication().
			WithAppName(anyAppName).
			WithEnvironment("dev", "master"))

	op, ok := <-synced
	assert.True(t, ok)
	assert.True(t, op)
}

func startApplicationController(client kubernetes.Interface, radixClient radixclient.Interface, handler Handler, stop chan struct{}) {

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, 0)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, 0)
	eventRecorder := &record.FakeRecorder{}

	controller := NewController(client, radixClient, &handler,
		radixInformerFactory.Radix().V1().RadixApplications(),
		radixInformerFactory.Radix().V1().RadixRegistrations(), eventRecorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)
	controller.Run(1, stop)

}
