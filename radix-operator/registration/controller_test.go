package registration

import (
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	log "github.com/sirupsen/logrus"
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

func setupTest() (RadixRegistrationHandler, kubernetes.Interface, radixclient.Interface) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()

	handlerTestUtils := test.NewTestUtils(client, radixClient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)

	handler := NewRegistrationHandler(
		client,
		radixClient,
		make(chan bool))
	return handler, client, radixClient
}

func Test_Controller_Calls_Handler(t *testing.T) {
	// Setup
	registrationHandler, client, radixClient := setupTest()

	stop := make(chan struct{})
	defer close(stop)
	go startRegistrationController(client, radixClient, registrationHandler, stop)

	// Test
	registration, err := utils.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	if err != nil {
		log.Fatalf("Could not read configuration data: %v", err)
	}

	registeredApp, err := radixClient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Create(registration)

	assert.NoError(t, err)
	assert.NotNil(t, registeredApp)

	op, ok := <-registrationHandler.SynchedOk
	assert.True(t, ok)
	assert.True(t, op)

	registration.ObjectMeta.Annotations = map[string]string{
		"update": "test",
	}
	updatedApp, err := radixClient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Update(registration)

	op, ok = <-registrationHandler.SynchedOk
	assert.True(t, ok)
	assert.True(t, op)

	assert.NoError(t, err)
	assert.NotNil(t, updatedApp)
	assert.NotNil(t, updatedApp.Annotations)
	assert.Equal(t, "test", updatedApp.Annotations["update"])
}

func startRegistrationController(client kubernetes.Interface, radixClient radixclient.Interface, handler RadixRegistrationHandler, stop chan struct{}) {

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, 0)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, 0)
	eventRecorder := &record.FakeRecorder{}

	controller := NewRegistrationController(client, radixClient, &handler,
		radixInformerFactory.Radix().V1().RadixRegistrations(),
		kubeInformerFactory.Core().V1().Namespaces(), eventRecorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)
	controller.Run(1, stop)

}

func Test_RadixRegistrationHandler_NoLimitsSetWhenNoLimitsDefined(t *testing.T) {
	// Setup
	registrationHandler, client, radixClient := setupTest()

	stop := make(chan struct{})
	defer close(stop)
	go startRegistrationController(client, radixClient, registrationHandler, stop)

	// Test
	registration, _ := utils.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	radixClient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Create(registration)
	op, _ := <-registrationHandler.SynchedOk
	assert.True(t, op)

	ns, _ := client.CoreV1().Namespaces().Get(utils.GetAppNamespace(registration.Name), metav1.GetOptions{})
	assert.NotNil(t, ns)

	limitRanges, _ := client.CoreV1().LimitRanges(utils.GetAppNamespace(registration.Name)).List(metav1.ListOptions{})
	assert.Equal(t, 0, len(limitRanges.Items), "Number of limit ranges was not expected")
}

func Test_RadixRegistrationHandler_LimitsSetWhenLimitsDefined(t *testing.T) {
	// Setup
	registrationHandler, client, radixClient := setupTest()

	stop := make(chan struct{})
	defer close(stop)
	go startRegistrationController(client, radixClient, registrationHandler, stop)

	// Setup
	os.Setenv(application.OperatorLimitDefaultCPUEnvironmentVariable, "0.5")
	os.Setenv(application.OperatorLimitDefaultMemoryEnvironmentVariable, "300M")
	os.Setenv(application.OperatorLimitDefaultReqestCPUEnvironmentVariable, "0.25")
	os.Setenv(application.OperatorLimitDefaultRequestMemoryEnvironmentVariable, "256M")

	// Test
	registration, _ := utils.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	radixClient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Create(registration)
	op, _ := <-registrationHandler.SynchedOk
	assert.True(t, op)

	ns, err := client.CoreV1().Namespaces().Get(utils.GetAppNamespace(registration.Name), metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, ns)

	limitRanges, _ := client.CoreV1().LimitRanges(utils.GetAppNamespace(registration.Name)).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(limitRanges.Items), "Number of limit ranges was not expected")
	assert.Equal(t, "mem-cpu-limit-range-app", limitRanges.Items[0].GetName(), "Expected limit range to be there by default")

}
