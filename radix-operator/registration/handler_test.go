package registration

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"

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

	handlerTestUtils := test.NewTestUtils(kubeclient, radixclient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)

	return registrationHandler, kubeclient, radixclient
}

func Test_RadixRegistrationHandler_NoLimitsSetWhenNoLimitsDefined(t *testing.T) {
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

	t.Run("validate limit range not set when missing on Operator", func(t *testing.T) {
		t.Parallel()
		limitRanges, _ := client.CoreV1().LimitRanges(utils.GetAppNamespace(registration.Name)).List(metav1.ListOptions{})
		assert.Equal(t, 0, len(limitRanges.Items), "Number of limit ranges was not expected")
	})
}

func Test_RadixRegistrationHandler_LimitsSetWhenLimitsDefined(t *testing.T) {
	// Setup
	registrationHandler, client, radixClient := setupTest()
	eventRecorder := &record.FakeRecorder{}
	registration, _ := utils.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	radixClient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Create(registration)

	// Setup
	os.Setenv(application.OperatorLimitDefaultCPUEnvironmentVariable, "0.5")
	os.Setenv(application.OperatorLimitDefaultMemoryEnvironmentVariable, "300M")
	os.Setenv(application.OperatorLimitDefaultReqestCPUEnvironmentVariable, "0.25")
	os.Setenv(application.OperatorLimitDefaultRequestMemoryEnvironmentVariable, "256M")

	// Test
	err := registrationHandler.Sync(corev1.NamespaceDefault, registration.Name, eventRecorder)
	assert.NoError(t, err)
	ns, err := client.CoreV1().Namespaces().Get(utils.GetAppNamespace(registration.Name), metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, ns)

	t.Run("validate limit range not set when missing on Operator", func(t *testing.T) {
		t.Parallel()
		limitRanges, _ := client.CoreV1().LimitRanges(utils.GetAppNamespace(registration.Name)).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(limitRanges.Items), "Number of limit ranges was not expected")
		assert.Equal(t, "mem-cpu-limit-range-app", limitRanges.Items[0].GetName(), "Expected limit range to be there by default")
	})
}
