package application

import (
	"fmt"
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
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

func TestOnSync_RegistrationCreated_AppNamespaceWithResourcesCreated(t *testing.T) {
	// Setup
	tu, client, radixClient := setupTest()

	// Test
	applyRegistrationWithSync(tu, client, radixClient, utils.ARadixRegistration().
		WithName("any-app"))

	ns, err := client.CoreV1().Namespaces().Get(utils.GetAppNamespace("any-app"), metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, ns)

	rolebindings, _ := client.RbacV1().RoleBindings("any-app-app").List(metav1.ListOptions{})
	assert.Equal(t, 2, len(rolebindings.Items))
	assert.Equal(t, "radix-pipeline", rolebindings.Items[0].Name)
	assert.Equal(t, "radix-app-admin", rolebindings.Items[1].Name)

	secrets, _ := client.CoreV1().Secrets("any-app-app").List(metav1.ListOptions{})
	assert.Equal(t, 1, len(secrets.Items))
	assert.Equal(t, "git-ssh-keys", secrets.Items[0].Name)

	serviceAccounts, _ := client.CoreV1().ServiceAccounts("any-app-app").List(metav1.ListOptions{})
	assert.Equal(t, 1, len(serviceAccounts.Items))
	assert.Equal(t, "radix-pipeline", serviceAccounts.Items[0].Name)
}

func TestOnSync_RegistrationCreated_AppNamespaceReconciled(t *testing.T) {
	// Setup
	tu, client, radixClient := setupTest()

	// Create namespaces manually
	client.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "any-app-app",
		},
	})

	label := fmt.Sprintf("%s=%s", kube.RadixAppLabel, "any-app")

	// Test
	applyRegistrationWithSync(tu, client, radixClient, utils.ARadixRegistration().
		WithName("any-app"))

	namespaces, _ := client.CoreV1().Namespaces().List(metav1.ListOptions{
		LabelSelector: label,
	})
	assert.Equal(t, 1, len(namespaces.Items))
}

func TestOnSync_NoUserGroupDefined_DefaultUserGroupSet(t *testing.T) {
	// Setup
	tu, client, radixClient := setupTest()
	os.Setenv(OperatorDefaultUserGroupEnvironmentVariable, "9876-54321-09876")

	// Test
	applyRegistrationWithSync(tu, client, radixClient, utils.ARadixRegistration().
		WithName("any-app").
		WithAdGroups([]string{}))

	rolebindings, _ := client.RbacV1().RoleBindings("any-app-app").List(metav1.ListOptions{})
	assert.Equal(t, 2, len(rolebindings.Items))
	assert.Equal(t, "radix-app-admin", rolebindings.Items[1].Name)
	assert.Equal(t, "9876-54321-09876", rolebindings.Items[1].Subjects[0].Name)

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

	limitRanges, _ := client.CoreV1().LimitRanges(utils.GetAppNamespace("any-app")).List(metav1.ListOptions{})
	assert.Equal(t, 0, len(limitRanges.Items), "Number of limit ranges was not expected")

}

func applyRegistrationWithSync(tu test.Utils, client kubernetes.Interface,
	radixclient radixclient.Interface, registrationBuilder utils.RegistrationBuilder) (*v1.RadixRegistration, error) {
	err := tu.ApplyRegistration(registrationBuilder)

	rr := registrationBuilder.BuildRR()
	application, _ := NewApplication(client, radixclient, rr)
	err = application.OnSync()
	if err != nil {
		return nil, err
	}

	return rr, nil
}

func updateRegistrationWithSync(tu test.Utils, client kubernetes.Interface,
	radixclient radixclient.Interface, rr *v1.RadixRegistration) error {
	_, err := radixclient.RadixV1().RadixRegistrations().Update(rr)
	if err != nil {
		return err
	}

	application, _ := NewApplication(client, radixclient, rr)
	err = application.OnSync()
	if err != nil {
		return err
	}

	return nil
}
