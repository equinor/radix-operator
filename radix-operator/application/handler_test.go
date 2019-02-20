package application

import (
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	registration "github.com/equinor/radix-operator/radix-operator/registration"
	"github.com/equinor/radix-operator/radix-operator/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube "k8s.io/client-go/kubernetes"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

const clusterName = "AnyClusterName"
const dnsZone = "dev.radix.equinor.com"
const containerRegistry = "any.container.registry"

func setupTest() (*test.Utils, kube.Interface) {
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	registrationHandler := registration.NewRegistrationHandler(kubeclient, radixclient)
	applicationHandler := NewApplicationHandler(kubeclient, radixclient)

	handlerTestUtils := test.NewHandlerTestUtils(kubeclient, radixclient, &registrationHandler, &applicationHandler, nil)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	return &handlerTestUtils, kubeclient
}

func TestObjectSynced_WithEnvironmentsNoLimitsSet_NamespacesAreCreatedWithNoLimits(t *testing.T) {
	handlerTestUtils, kubeclient := setupTest()

	handlerTestUtils.ApplyApplication(utils.ARadixApplication().
		WithAppName("any-app").
		WithEnvironment("dev", "master").
		WithEnvironment("prod", ""))

	t.Run("validate namespace creation", func(t *testing.T) {
		devNs, _ := kubeclient.CoreV1().Namespaces().Get("any-app-dev", metav1.GetOptions{})
		assert.NotNil(t, devNs)
		prodNs, _ := kubeclient.CoreV1().Namespaces().Get("any-app-prod", metav1.GetOptions{})
		assert.NotNil(t, prodNs)
	})

	t.Run("validate rolebindings", func(t *testing.T) {
		t.Parallel()
		rolebindings, _ := kubeclient.RbacV1().RoleBindings("any-app-dev").List(metav1.ListOptions{})
		assert.Equal(t, 1, len(rolebindings.Items), "Number of rolebindings was not expected")
		assert.Equal(t, "radix-app-admin-envs", rolebindings.Items[0].GetName(), "Expected rolebinding radix-app-admin-envs to be there by default")

		rolebindings, _ = kubeclient.RbacV1().RoleBindings("any-app-prod").List(metav1.ListOptions{})
		assert.Equal(t, 1, len(rolebindings.Items), "Number of rolebindings was not expected")
		assert.Equal(t, "radix-app-admin-envs", rolebindings.Items[0].GetName(), "Expected rolebinding radix-app-admin-envs to be there by default")
	})

	t.Run("validate limit range not set when missing on Operator", func(t *testing.T) {
		t.Parallel()
		limitRanges, _ := kubeclient.CoreV1().LimitRanges("any-app-dev").List(metav1.ListOptions{})
		assert.Equal(t, 0, len(limitRanges.Items), "Number of limit ranges was not expected")

		limitRanges, _ = kubeclient.CoreV1().LimitRanges("any-app-prod").List(metav1.ListOptions{})
		assert.Equal(t, 0, len(limitRanges.Items), "Number of limit ranges was not expected")
	})
}

func TestObjectSynced_WithEnvironmentsAndLimitsSet_NamespacesAreCreatedWithLimits(t *testing.T) {
	handlerTestUtils, kubeclient := setupTest()

	// Setup
	os.Setenv(applicationconfig.OperatorEnvLimitDefaultCPUEnvironmentVariable, "0.5")
	os.Setenv(applicationconfig.OperatorEnvLimitDefaultMemoryEnvironmentVariable, "300M")
	os.Setenv(applicationconfig.OperatorEnvLimitDefaultReqestCPUEnvironmentVariable, "0.25")
	os.Setenv(applicationconfig.OperatorEnvLimitDefaultRequestMemoryEnvironmentVariable, "256M")

	handlerTestUtils.ApplyApplication(utils.ARadixApplication().
		WithAppName("any-app").
		WithEnvironment("dev", "master").
		WithEnvironment("prod", ""))

	limitRanges, _ := kubeclient.CoreV1().LimitRanges("any-app-dev").List(metav1.ListOptions{})
	assert.Equal(t, 1, len(limitRanges.Items), "Number of limit ranges was not expected")
	assert.Equal(t, "mem-cpu-limit-range-env", limitRanges.Items[0].GetName(), "Expected limit range to be there by default")

	limitRanges, _ = kubeclient.CoreV1().LimitRanges("any-app-prod").List(metav1.ListOptions{})
	assert.Equal(t, 1, len(limitRanges.Items), "Number of limit ranges was not expected")
	assert.Equal(t, "mem-cpu-limit-range-env", limitRanges.Items[0].GetName(), "Expected limit range to be there by default")
}
