package application

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	clusterName       = "AnyClusterName"
	dnsZone           = "dev.radix.equinor.com"
	containerRegistry = "any.container.registry"
)

func setupTest() (test.Utils, kubernetes.Interface, *kube.Kube, radixclient.Interface) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()
	kubeUtil, _ := kube.New(client, radixClient)

	handlerTestUtils := test.NewTestUtils(client, radixClient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	return handlerTestUtils, client, kubeUtil, radixClient
}

func TestOnSync_RegistrationCreated_AppNamespaceWithResourcesCreated(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient := setupTest()

	// Test
	applyRegistrationWithSync(tu, client, kubeUtil, radixClient, utils.ARadixRegistration().
		WithName("any-app").
		WithMachineUser(true))

	clusterRolebindings, _ := client.RbacV1().ClusterRoleBindings().List(context.TODO(), metav1.ListOptions{})
	assert.True(t, clusterRoleBindingByNameExists("any-app-machine-user", clusterRolebindings))

	ns, err := client.CoreV1().Namespaces().Get(context.TODO(), utils.GetAppNamespace("any-app"), metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, ns)

	rolebindings, _ := client.RbacV1().RoleBindings("any-app-app").List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 3, len(rolebindings.Items))
	assert.True(t, roleBindingByNameExists(defaults.ConfigToMapRunnerRoleName, rolebindings))
	assert.True(t, roleBindingByNameExists(defaults.PipelineRoleName, rolebindings))
	assert.True(t, roleBindingByNameExists(defaults.AppAdminRoleName, rolebindings))

	appAdminRoleBinding := getRoleBindingByName(defaults.AppAdminRoleName, rolebindings)
	assert.Equal(t, 2, len(appAdminRoleBinding.Subjects))
	assert.Equal(t, "any-app-machine-user", appAdminRoleBinding.Subjects[1].Name)

	secrets, _ := client.CoreV1().Secrets("any-app-app").List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 1, len(secrets.Items))
	assert.Equal(t, "git-ssh-keys", secrets.Items[0].Name)

	serviceAccounts, _ := client.CoreV1().ServiceAccounts("any-app-app").List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 3, len(serviceAccounts.Items))
	assert.True(t, serviceAccountByNameExists(defaults.ConfigToMapRunnerRoleName, serviceAccounts))
	assert.True(t, serviceAccountByNameExists(defaults.PipelineRoleName, serviceAccounts))
	assert.True(t, serviceAccountByNameExists("any-app-machine-user", serviceAccounts))
}

func TestOnSync_RegistrationCreated_AppNamespaceReconciled(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient := setupTest()

	// Create namespaces manually
	client.CoreV1().Namespaces().Create(
		context.TODO(),
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "any-app-app",
			},
		},
		metav1.CreateOptions{})

	label := fmt.Sprintf("%s=%s", kube.RadixAppLabel, "any-app")

	// Test
	applyRegistrationWithSync(tu, client, kubeUtil, radixClient, utils.ARadixRegistration().
		WithName("any-app"))

	namespaces, _ := client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{
		LabelSelector: label,
	})
	assert.Equal(t, 1, len(namespaces.Items))
}

func TestOnSync_NoUserGroupDefined_DefaultUserGroupSet(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient := setupTest()
	os.Setenv(OperatorDefaultUserGroupEnvironmentVariable, "9876-54321-09876")

	// Test
	applyRegistrationWithSync(tu, client, kubeUtil, radixClient, utils.ARadixRegistration().
		WithName("any-app").
		WithAdGroups([]string{}))

	rolebindings, _ := client.RbacV1().RoleBindings("any-app-app").List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 3, len(rolebindings.Items))
	assert.True(t, roleBindingByNameExists(defaults.AppAdminRoleName, rolebindings))
	assert.Equal(t, "9876-54321-09876", getRoleBindingByName(defaults.AppAdminRoleName, rolebindings).Subjects[0].Name)

}

func TestOnSync_LimitsDefined_LimitsSet(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient := setupTest()

	os.Setenv(defaults.OperatorAppLimitDefaultCPUEnvironmentVariable, "0.5")
	os.Setenv(defaults.OperatorAppLimitDefaultMemoryEnvironmentVariable, "300M")
	os.Setenv(defaults.OperatorAppLimitDefaultReqestCPUEnvironmentVariable, "0.25")
	os.Setenv(defaults.OperatorAppLimitDefaultRequestMemoryEnvironmentVariable, "256M")

	// Test
	applyRegistrationWithSync(tu, client, kubeUtil, radixClient, utils.ARadixRegistration().
		WithName("any-app"))

	limitRanges, _ := client.CoreV1().LimitRanges(utils.GetAppNamespace("any-app")).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 1, len(limitRanges.Items), "Number of limit ranges was not expected")
	assert.Equal(t, "mem-cpu-limit-range-app", limitRanges.Items[0].GetName(), "Expected limit range to be there by default")

}

func TestOnSync_NoLimitsDefined_NoLimitsSet(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient := setupTest()
	os.Setenv(defaults.OperatorAppLimitDefaultCPUEnvironmentVariable, "")
	os.Setenv(defaults.OperatorAppLimitDefaultMemoryEnvironmentVariable, "")
	os.Setenv(defaults.OperatorAppLimitDefaultReqestCPUEnvironmentVariable, "")
	os.Setenv(defaults.OperatorAppLimitDefaultRequestMemoryEnvironmentVariable, "")

	// Test
	applyRegistrationWithSync(tu, client, kubeUtil, radixClient, utils.ARadixRegistration().
		WithName("any-app"))

	limitRanges, _ := client.CoreV1().LimitRanges(utils.GetAppNamespace("any-app")).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 0, len(limitRanges.Items), "Number of limit ranges was not expected")

}

func applyRegistrationWithSync(tu test.Utils, client kubernetes.Interface, kubeUtil *kube.Kube,
	radixclient radixclient.Interface, registrationBuilder utils.RegistrationBuilder) (*v1.RadixRegistration, error) {
	rr, err := tu.ApplyRegistration(registrationBuilder)

	application, _ := NewApplication(client, kubeUtil, radixclient, rr)
	err = application.OnSyncWithGranterToMachineUserToken(mockedGranter)
	if err != nil {
		return nil, err
	}

	return rr, nil
}

func updateRegistrationWithSync(tu test.Utils, client kubernetes.Interface, kubeUtil *kube.Kube,
	radixclient radixclient.Interface, rr *v1.RadixRegistration) error {
	_, err := radixclient.RadixV1().RadixRegistrations().Update(context.TODO(), rr, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	application, _ := NewApplication(client, kubeUtil, radixclient, rr)
	err = application.OnSyncWithGranterToMachineUserToken(mockedGranter)
	if err != nil {
		return err
	}

	return nil
}

// This is created because the granting to token functionality doesn't work in this context
func mockedGranter(kubeutil *kube.Kube, app *v1.RadixRegistration, namespace string, serviceAccount *corev1.ServiceAccount) error {
	return nil
}

func getRoleBindingByName(name string, roleBindings *rbacv1.RoleBindingList) *rbacv1.RoleBinding {
	for _, roleBinding := range roleBindings.Items {
		if roleBinding.Name == name {
			return &roleBinding
		}
	}

	return nil
}

func roleBindingByNameExists(name string, roleBindings *rbacv1.RoleBindingList) bool {
	roleBinding := getRoleBindingByName(name, roleBindings)
	if roleBinding != nil {
		return true
	}

	return false
}

func getClusterRoleBindingByName(name string, clusterRoleBindings *rbacv1.ClusterRoleBindingList) *rbacv1.ClusterRoleBinding {
	for _, clusterRoleBinding := range clusterRoleBindings.Items {
		if clusterRoleBinding.Name == name {
			return &clusterRoleBinding
		}
	}

	return nil
}

func clusterRoleBindingByNameExists(name string, clusterRoleBindings *rbacv1.ClusterRoleBindingList) bool {
	clusterRoleBinding := getClusterRoleBindingByName(name, clusterRoleBindings)
	if clusterRoleBinding != nil {
		return true
	}

	return false
}

func getServiceAccountByName(name string, serviceAccounts *corev1.ServiceAccountList) *corev1.ServiceAccount {
	for _, serviceAccount := range serviceAccounts.Items {
		if serviceAccount.Name == name {
			return &serviceAccount
		}
	}

	return nil
}

func serviceAccountByNameExists(name string, serviceAccounts *corev1.ServiceAccountList) bool {
	serviceAccount := getServiceAccountByName(name, serviceAccounts)
	if serviceAccount != nil {
		return true
	}

	return false
}
