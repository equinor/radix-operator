package application

import (
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	clusterName       = "AnyClusterName"
	dnsZone           = "dev.radix.equinor.com"
	containerRegistry = "any.container.registry"
)

func setupTest() (test.Utils, kubernetes.Interface, radixclient.Interface) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()

	handlerTestUtils := test.NewTestUtils(client, radixClient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	return handlerTestUtils, client, radixClient
}

func TestOnSync_LimitsDefined_LimitsSet(t *testing.T) {
	// Setup
	tu, client, radixClient := setupTest()
	os.Setenv(OperatorLimitDefaultCPUEnvironmentVariable, "0.5")
	os.Setenv(OperatorLimitDefaultMemoryEnvironmentVariable, "300M")
	os.Setenv(OperatorLimitDefaultReqestCPUEnvironmentVariable, "0.25")
	os.Setenv(OperatorLimitDefaultRequestMemoryEnvironmentVariable, "256M")

	// Test
	applyRegistrationWithSync(tu, client, radixClient, utils.ARadixRegistration().
		WithName("any-app"))

	ns, err := client.CoreV1().Namespaces().Get(utils.GetAppNamespace("any-app"), metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, ns)

	limitRanges, _ := client.CoreV1().LimitRanges(utils.GetAppNamespace("any-app")).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(limitRanges.Items), "Number of limit ranges was not expected")
	assert.Equal(t, "mem-cpu-limit-range-app", limitRanges.Items[0].GetName(), "Expected limit range to be there by default")

}

func TestOnSync_NoLimitsDefined_NoLimitsSet(t *testing.T) {
	// Setup
	tu, client, radixClient := setupTest()
	os.Setenv(OperatorLimitDefaultCPUEnvironmentVariable, "")
	os.Setenv(OperatorLimitDefaultMemoryEnvironmentVariable, "")
	os.Setenv(OperatorLimitDefaultReqestCPUEnvironmentVariable, "")
	os.Setenv(OperatorLimitDefaultRequestMemoryEnvironmentVariable, "")

	// Test
	applyRegistrationWithSync(tu, client, radixClient, utils.ARadixRegistration().
		WithName("any-app"))

	ns, _ := client.CoreV1().Namespaces().Get(utils.GetAppNamespace("any-app"), metav1.GetOptions{})
	assert.NotNil(t, ns)

	limitRanges, _ := client.CoreV1().LimitRanges(utils.GetAppNamespace("any-app")).List(metav1.ListOptions{})
	assert.Equal(t, 0, len(limitRanges.Items), "Number of limit ranges was not expected")

}

func applyRegistrationWithSync(tu test.Utils, client kubernetes.Interface,
	radixclient radixclient.Interface, registrationBuilder utils.RegistrationBuilder) error {
	err := tu.ApplyRegistration(registrationBuilder)

	rr := registrationBuilder.BuildRR()
	application, _ := NewApplication(client, radixclient, rr)
	err = application.OnSync()
	if err != nil {
		return err
	}

	return nil
}
