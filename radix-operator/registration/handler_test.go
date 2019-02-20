package registration

import (
	"io/ioutil"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/radix-operator/test"

	log "github.com/sirupsen/logrus"

	"github.com/stretchr/testify/assert"

	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube "k8s.io/client-go/kubernetes"
	kubernetes "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
)

const (
	clusterName       = "AnyClusterName"
	dnsZone           = "dev.radix.equinor.com"
	containerRegistry = "any.container.registry"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

func setupTest() (RadixRegistrationHandler, kube.Interface, radixclient.Interface) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	registrationHandler := NewRegistrationHandler(kubeclient, radixclient)

	handlerTestUtils := test.NewHandlerTestUtils(kubeclient, radixclient, &registrationHandler, nil, nil)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)

	return registrationHandler, kubeclient, radixclient
}
func Test_RadixRegistrationHandler(t *testing.T) {
	// Setup
	registrationHandler, client, radixClient := setupTest()
	eventRecorder := &record.FakeRecorder{}
	registration, _ := utils.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	radixClient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Create(registration)

	// Test
	err := registrationHandler.Sync(corev1.NamespaceDefault, registration.Name, eventRecorder)
	assert.NoError(t, err)
	ns, err := client.CoreV1().Namespaces().Get(utils.GetAppNamespace(registration.Name), metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, ns)
}
