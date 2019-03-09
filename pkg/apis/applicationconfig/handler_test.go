package applicationconfig

import (
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube "k8s.io/client-go/kubernetes"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

const clusterName = "AnyClusterName"
const containerRegistry = "any.container.registry"

func setupTest() (*test.Utils, kube.Interface, radixclient.Interface) {
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	handlerTestUtils := test.NewTestUtils(kubeclient, radixclient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	return &handlerTestUtils, kubeclient, radixclient
}

func TestObjectSynced_WithEnvironmentsNoLimitsSet_NamespacesAreCreatedWithNoLimits(t *testing.T) {
	tu, client, radixclient := setupTest()

	applyApplicationWithSync(tu, client, radixclient, utils.ARadixApplication().
		WithAppName("any-app").
		WithEnvironment("dev", "master").
		WithEnvironment("prod", ""))

	t.Run("validate namespace creation", func(t *testing.T) {
		devNs, _ := client.CoreV1().Namespaces().Get("any-app-dev", metav1.GetOptions{})
		assert.NotNil(t, devNs)
		prodNs, _ := client.CoreV1().Namespaces().Get("any-app-prod", metav1.GetOptions{})
		assert.NotNil(t, prodNs)
	})

	t.Run("validate rolebindings", func(t *testing.T) {
		t.Parallel()
		rolebindings, _ := client.RbacV1().RoleBindings("any-app-dev").List(metav1.ListOptions{})
		assert.Equal(t, 1, len(rolebindings.Items), "Number of rolebindings was not expected")
		assert.Equal(t, "radix-app-admin-envs", rolebindings.Items[0].GetName(), "Expected rolebinding radix-app-admin-envs to be there by default")

		rolebindings, _ = client.RbacV1().RoleBindings("any-app-prod").List(metav1.ListOptions{})
		assert.Equal(t, 1, len(rolebindings.Items), "Number of rolebindings was not expected")
		assert.Equal(t, "radix-app-admin-envs", rolebindings.Items[0].GetName(), "Expected rolebinding radix-app-admin-envs to be there by default")
	})

	t.Run("validate limit range not set when missing on Operator", func(t *testing.T) {
		t.Parallel()
		limitRanges, _ := client.CoreV1().LimitRanges("any-app-dev").List(metav1.ListOptions{})
		assert.Equal(t, 0, len(limitRanges.Items), "Number of limit ranges was not expected")

		limitRanges, _ = client.CoreV1().LimitRanges("any-app-prod").List(metav1.ListOptions{})
		assert.Equal(t, 0, len(limitRanges.Items), "Number of limit ranges was not expected")
	})
}

func TestObjectSynced_WithEnvironmentsAndLimitsSet_NamespacesAreCreatedWithLimits(t *testing.T) {
	tu, client, radixclient := setupTest()

	// Setup
	os.Setenv(OperatorEnvLimitDefaultCPUEnvironmentVariable, "0.5")
	os.Setenv(OperatorEnvLimitDefaultMemoryEnvironmentVariable, "300M")
	os.Setenv(OperatorEnvLimitDefaultReqestCPUEnvironmentVariable, "0.25")
	os.Setenv(OperatorEnvLimitDefaultRequestMemoryEnvironmentVariable, "256M")

	applyApplicationWithSync(tu, client, radixclient, utils.ARadixApplication().
		WithAppName("any-app").
		WithEnvironment("dev", "master").
		WithEnvironment("prod", ""))

	limitRanges, _ := client.CoreV1().LimitRanges("any-app-dev").List(metav1.ListOptions{})
	assert.Equal(t, 1, len(limitRanges.Items), "Number of limit ranges was not expected")
	assert.Equal(t, "mem-cpu-limit-range-env", limitRanges.Items[0].GetName(), "Expected limit range to be there by default")

	limitRanges, _ = client.CoreV1().LimitRanges("any-app-prod").List(metav1.ListOptions{})
	assert.Equal(t, 1, len(limitRanges.Items), "Number of limit ranges was not expected")
	assert.Equal(t, "mem-cpu-limit-range-env", limitRanges.Items[0].GetName(), "Expected limit range to be there by default")
}

func applyApplicationWithSync(tu *test.Utils, client kube.Interface,
	radixclient radixclient.Interface, applicationBuilder utils.ApplicationBuilder) error {

	err := tu.ApplyApplication(applicationBuilder)
	if err != nil {
		return err
	}

	ra := applicationBuilder.BuildRA()
	radixRegistration, err := radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Get(ra.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	applicationconfig, err := NewApplicationConfig(client, radixclient, radixRegistration, ra)

	err = applicationconfig.OnSync()
	if err != nil {
		return err
	}

	return nil
}
