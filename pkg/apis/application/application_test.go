package application

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func setupTest(t *testing.T) (test.Utils, kubernetes.Interface, *kube.Kube, radixclient.Interface, *kedafake.Clientset) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(client, radixClient, kedaClient, secretproviderclient)
	handlerTestUtils := test.NewTestUtils(client, radixClient, kedaClient, secretproviderclient)
	err := handlerTestUtils.CreateClusterPrerequisites("AnyClusterName", "0.0.0.0", "anysubid")
	require.NoError(t, err)
	return handlerTestUtils, client, kubeUtil, radixClient, kedaClient
}

func TestOnSync_CorrectRRScopedClusterRoles_CorrectClusterRoleBindings(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient, _ := setupTest(t)
	defer os.Clearenv()

	// Test
	appName := "any-app"
	rr, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, utils.ARadixRegistration().
		WithName(appName))
	assert.NoError(t, err)

	clusterRoleBindings, _ := client.RbacV1().ClusterRoleBindings().List(context.Background(), metav1.ListOptions{})
	for _, clusterRoleBindingName := range []string{
		"radix-platform-user-rr-any-app", "radix-pipeline-rr-any-app", "radix-platform-user-rr-reader-any-app",
	} {
		assert.True(t, clusterRoleBindingByNameExists(clusterRoleBindingName, clusterRoleBindings), fmt.Sprintf("ClusterRoleBinding %s does not exist", clusterRoleBindingName))
	}

	assert.Equal(t, getClusterRoleBindingByName("radix-pipeline-rr-any-app", clusterRoleBindings).Subjects[0].Name, defaults.PipelineServiceAccountName)
	assert.Equal(t, getClusterRoleBindingByName("radix-platform-user-rr-any-app", clusterRoleBindings).Subjects[0].Name, rr.Spec.AdGroups[0])
	assert.Len(t, getClusterRoleBindingByName("radix-platform-user-rr-any-app", clusterRoleBindings).Subjects, 1)
	assert.Equal(t, getClusterRoleBindingByName("radix-platform-user-rr-reader-any-app", clusterRoleBindings).Subjects[0].Name, rr.Spec.ReaderAdGroups[0])

	clusterRoles, _ := client.RbacV1().ClusterRoles().List(context.Background(), metav1.ListOptions{})
	for _, clusterRoleName := range []string{
		"radix-platform-user-rr-any-app", "radix-pipeline-rr-any-app", "radix-platform-user-rr-reader-any-app",
	} {
		assert.True(t, clusterRoleByNameExists(clusterRoleName, clusterRoles), fmt.Sprintf("ClusterRole %s does not exist", clusterRoleName))
	}
}

func TestOnSync_CorrectRoleBindings_AppNamespace(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient, _ := setupTest(t)
	defer os.Clearenv()

	// Test
	appName := "any-app"
	rr, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, utils.ARadixRegistration().
		WithName(appName))
	require.NoError(t, err)

	roleBindings, _ := client.RbacV1().RoleBindings(utils.GetAppNamespace(appName)).List(context.Background(), metav1.ListOptions{})
	assert.ElementsMatch(t,
		[]string{defaults.PipelineAppRoleName, defaults.AppAdminRoleName, defaults.AppReaderRoleName, "git-ssh-keys"},
		getRoleBindingNames(roleBindings),
	)

	require.Len(t, getRoleBindingByName(defaults.AppAdminRoleName, roleBindings).Subjects, 1)
	assert.Equal(t, getRoleBindingByName(defaults.PipelineAppRoleName, roleBindings).Subjects[0].Name, defaults.PipelineServiceAccountName)
	assert.Equal(t, getRoleBindingByName(defaults.AppAdminRoleName, roleBindings).Subjects[0].Name, rr.Spec.AdGroups[0])
	assert.Equal(t, getRoleBindingByName(defaults.AppReaderRoleName, roleBindings).Subjects[0].Name, rr.Spec.ReaderAdGroups[0])
	assert.Equal(t, getRoleBindingByName("git-ssh-keys", roleBindings).Subjects[0].Name, rr.Spec.AdGroups[0])
}

func TestOnSync_RegistrationCreated_AppNamespaceWithResourcesCreated(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient, _ := setupTest(t)
	defer os.Clearenv()

	// Test
	appName := "any-app"
	_, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, utils.ARadixRegistration().
		WithName(appName))
	require.NoError(t, err)

	ns, err := client.CoreV1().Namespaces().Get(context.Background(), utils.GetAppNamespace(appName), metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, ns)
	expected := map[string]string{
		kube.RadixAppLabel:          appName,
		kube.RadixEnvLabel:          utils.AppNamespaceEnvName,
		"snyk-service-account-sync": "radix-snyk-service-account",
	}
	assert.Equal(t, expected, ns.GetLabels())

	roleBindings, _ := client.RbacV1().RoleBindings("any-app-app").List(context.Background(), metav1.ListOptions{})
	assert.ElementsMatch(t,
		[]string{defaults.PipelineAppRoleName, defaults.AppAdminRoleName, "git-ssh-keys", defaults.AppReaderRoleName},
		getRoleBindingNames(roleBindings))
	appAdminRoleBinding := getRoleBindingByName(defaults.AppAdminRoleName, roleBindings)
	assert.Equal(t, 1, len(appAdminRoleBinding.Subjects))

	secrets, _ := client.CoreV1().Secrets("any-app-app").List(context.Background(), metav1.ListOptions{})
	secretNames := slice.Map(secrets.Items, func(s corev1.Secret) string { return s.Name })
	expectedSecrets := []string{defaults.GitPrivateKeySecretName, defaults.AzureACRServicePrincipleBuildahSecretName, defaults.AzureACRServicePrincipleSecretName, defaults.AzureACRTokenPasswordAppRegistrySecretName}
	assert.ElementsMatch(t, expectedSecrets, secretNames)

	serviceAccounts, _ := client.CoreV1().ServiceAccounts("any-app-app").List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 1, len(serviceAccounts.Items))
	assert.True(t, serviceAccountByNameExists(defaults.PipelineServiceAccountName, serviceAccounts))
}

func TestOnSync_PodSecurityStandardLabelsSetOnNamespace(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient, _ := setupTest(t)
	defer os.Clearenv()
	os.Setenv(defaults.PodSecurityStandardAppNamespaceEnforceLevelEnvironmentVariable, "enforceAppNsLvl")
	os.Setenv(defaults.PodSecurityStandardEnforceLevelEnvironmentVariable, "enforceLvl")
	os.Setenv(defaults.PodSecurityStandardEnforceVersionEnvironmentVariable, "enforceVer")
	os.Setenv(defaults.PodSecurityStandardAuditLevelEnvironmentVariable, "auditLvl")
	os.Setenv(defaults.PodSecurityStandardAuditVersionEnvironmentVariable, "auditVer")
	os.Setenv(defaults.PodSecurityStandardWarnLevelEnvironmentVariable, "warnLvl")
	os.Setenv(defaults.PodSecurityStandardWarnVersionEnvironmentVariable, "warnVer")

	// Test
	appName := "any-app"
	_, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, utils.ARadixRegistration().
		WithName(appName))
	require.NoError(t, err)

	ns, err := client.CoreV1().Namespaces().Get(context.Background(), utils.GetAppNamespace(appName), metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, ns)
	expected := map[string]string{
		kube.RadixAppLabel:                           appName,
		kube.RadixEnvLabel:                           utils.AppNamespaceEnvName,
		"snyk-service-account-sync":                  "radix-snyk-service-account",
		"pod-security.kubernetes.io/enforce":         "enforceAppNsLvl",
		"pod-security.kubernetes.io/enforce-version": "enforceVer",
		"pod-security.kubernetes.io/audit":           "auditLvl",
		"pod-security.kubernetes.io/audit-version":   "auditVer",
		"pod-security.kubernetes.io/warn":            "warnLvl",
		"pod-security.kubernetes.io/warn-version":    "warnVer",
	}
	assert.Equal(t, expected, ns.GetLabels())
}

func TestOnSync_RegistrationCreated_AppNamespaceReconciled(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient, _ := setupTest(t)
	defer os.Clearenv()

	// Create namespaces manually
	_, err := client.CoreV1().Namespaces().Create(
		context.Background(),
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "any-app-app",
			},
		},
		metav1.CreateOptions{})
	require.NoError(t, err)

	label := fmt.Sprintf("%s=%s", kube.RadixAppLabel, "any-app")

	// Test
	_, err = applyRegistrationWithSync(tu, client, kubeUtil, radixClient, utils.ARadixRegistration().
		WithName("any-app"))
	require.NoError(t, err)

	namespaces, _ := client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{
		LabelSelector: label,
	})
	assert.Equal(t, 1, len(namespaces.Items))
}

func TestOnSync_NoUserGroupDefined_DefaultUserGroupSet(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient, _ := setupTest(t)
	defaultRole := "9876-54321-09876"
	defer os.Clearenv()
	os.Setenv(defaults.OperatorDefaultUserGroupEnvironmentVariable, defaultRole)

	// Test
	_, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, utils.ARadixRegistration().WithName("any-app").WithAdGroups([]string{}).WithReaderAdGroups([]string{}))
	require.NoError(t, err)

	rolebindings, _ := client.RbacV1().RoleBindings("any-app-app").List(context.Background(), metav1.ListOptions{})
	assert.ElementsMatch(t,
		[]string{defaults.PipelineAppRoleName, defaults.AppAdminRoleName, "git-ssh-keys", defaults.AppReaderRoleName},
		getRoleBindingNames(rolebindings))
	assert.Equal(t, defaultRole, getRoleBindingByName(defaults.AppAdminRoleName, rolebindings).Subjects[0].Name)
	assert.Equal(t, defaultRole, getRoleBindingByName("git-ssh-keys", rolebindings).Subjects[0].Name)

	clusterRoleBindings, _ := client.RbacV1().ClusterRoleBindings().List(context.Background(), metav1.ListOptions{})
	require.Len(t, getClusterRoleBindingByName("radix-platform-user-rr-any-app", clusterRoleBindings).Subjects, 1)
	assert.Equal(t, defaultRole, getClusterRoleBindingByName("radix-platform-user-rr-any-app", clusterRoleBindings).Subjects[0].Name)
	assert.Len(t, getClusterRoleBindingByName("radix-platform-user-rr-reader-any-app", clusterRoleBindings).Subjects, 0)
}

func TestOnSync_LimitsDefined_LimitsSet(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient, _ := setupTest(t)
	defer os.Clearenv()
	os.Setenv(defaults.OperatorAppLimitDefaultMemoryEnvironmentVariable, "300M")
	os.Setenv(defaults.OperatorAppLimitDefaultRequestCPUEnvironmentVariable, "0.25")
	os.Setenv(defaults.OperatorAppLimitDefaultRequestMemoryEnvironmentVariable, "256M")

	// Test
	_, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, utils.ARadixRegistration().
		WithName("any-app"))
	require.NoError(t, err)

	limitRanges, _ := client.CoreV1().LimitRanges(utils.GetAppNamespace("any-app")).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 1, len(limitRanges.Items), "Number of limit ranges was not expected")
	assert.Equal(t, "mem-cpu-limit-range-app", limitRanges.Items[0].GetName(), "Expected limit range to be there by default")

}

func TestOnSync_NoLimitsDefined_NoLimitsSet(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient, _ := setupTest(t)
	defer os.Clearenv()
	os.Setenv(defaults.OperatorAppLimitDefaultMemoryEnvironmentVariable, "")
	os.Setenv(defaults.OperatorAppLimitDefaultRequestCPUEnvironmentVariable, "")
	os.Setenv(defaults.OperatorAppLimitDefaultRequestMemoryEnvironmentVariable, "")

	// Test
	_, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, utils.ARadixRegistration().
		WithName("any-app"))
	require.NoError(t, err)

	limitRanges, _ := client.CoreV1().LimitRanges(utils.GetAppNamespace("any-app")).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 0, len(limitRanges.Items), "Number of limit ranges was not expected")

}

func applyRegistrationWithSync(tu test.Utils, client kubernetes.Interface, kubeUtil *kube.Kube,
	radixclient radixclient.Interface, registrationBuilder utils.RegistrationBuilder) (*v1.RadixRegistration, error) {
	rr, err := tu.ApplyRegistration(registrationBuilder)
	if err != nil {
		return nil, err
	}

	application, _ := NewApplication(client, kubeUtil, radixclient, rr)
	err = application.OnSync(context.Background())
	if err != nil {
		return nil, err
	}

	return rr, nil
}

func getRoleBindingByName(name string, roleBindings *rbacv1.RoleBindingList) *rbacv1.RoleBinding {
	for _, roleBinding := range roleBindings.Items {
		if roleBinding.Name == name {
			return &roleBinding
		}
	}

	return nil
}

func getRoleBindingNames(roleBindings *rbacv1.RoleBindingList) []string {
	if roleBindings == nil {
		return nil
	}
	return slice.Map(roleBindings.Items, func(r rbacv1.RoleBinding) string { return r.Name })
}

// func roleBindingByNameExists(name string, roleBindings *rbacv1.RoleBindingList) bool {
// 	return getRoleBindingByName(name, roleBindings) != nil
// }

func getClusterRoleBindingByName(name string, clusterRoleBindings *rbacv1.ClusterRoleBindingList) *rbacv1.ClusterRoleBinding {
	for _, clusterRoleBinding := range clusterRoleBindings.Items {
		if clusterRoleBinding.Name == name {
			return &clusterRoleBinding
		}
	}

	return nil
}

func getClusterRoleByName(name string, clusterRoles *rbacv1.ClusterRoleList) *rbacv1.ClusterRole {
	for _, clusterRole := range clusterRoles.Items {
		if clusterRole.Name == name {
			return &clusterRole
		}
	}

	return nil
}

func clusterRoleBindingByNameExists(name string, clusterRoleBindings *rbacv1.ClusterRoleBindingList) bool {
	return getClusterRoleBindingByName(name, clusterRoleBindings) != nil
}

func clusterRoleByNameExists(name string, clusterRoles *rbacv1.ClusterRoleList) bool {
	return getClusterRoleByName(name, clusterRoles) != nil
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
	return getServiceAccountByName(name, serviceAccounts) != nil
}
