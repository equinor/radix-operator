package applicationconfig_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/equinor/radix-common/pkg/docker"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	sampleRegistration = "./testdata/sampleregistration.yaml"
	sampleApp          = "./testdata/radixconfig.yaml"
	clusterName        = "AnyClusterName"
)

func setupTest(t *testing.T) (*test.Utils, *kubefake.Clientset, *kube.Kube, *radixfake.Clientset) {
	kubeClient := kubefake.NewSimpleClientset()
	radixClient := radixfake.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(kubeClient, radixClient, kedaClient, secretproviderclient)
	handlerTestUtils := test.NewTestUtils(kubeClient, radixClient, kedaClient, secretproviderclient)
	err := handlerTestUtils.CreateClusterPrerequisites(clusterName, "anysubid")
	require.NoError(t, err)
	return &handlerTestUtils, kubeClient, kubeUtil, radixClient
}

func Test_Status(t *testing.T) {
	_, client, kubeUtil, radixClient := setupTest(t)

	rr := &radixv1.RadixRegistration{}
	ra := &radixv1.RadixApplication{ObjectMeta: metav1.ObjectMeta{Name: "any-name", Generation: 42}}
	ra, err := radixClient.RadixV1().RadixApplications("any-ns").Create(context.Background(), ra, metav1.CreateOptions{})
	require.NoError(t, err)

	// First sync sets status
	expectedGen := ra.Generation
	sut := applicationconfig.NewApplicationConfig(client, kubeUtil, radixClient, rr, ra, nil)
	err = sut.OnSync(context.Background())
	require.NoError(t, err)
	ra, err = radixClient.RadixV1().RadixApplications(ra.Namespace).Get(context.Background(), ra.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, radixv1.RadixApplicationReconcileSucceeded, ra.Status.ReconcileStatus)
	assert.Empty(t, ra.Status.Message)
	assert.Equal(t, expectedGen, ra.Status.ObservedGeneration)
	assert.False(t, ra.Status.Reconciled.IsZero())

	// Second sync with updated generation
	ra.Generation++
	expectedGen = ra.Generation
	sut = applicationconfig.NewApplicationConfig(client, kubeUtil, radixClient, rr, ra, nil)
	err = sut.OnSync(context.Background())
	require.NoError(t, err)
	ra, err = radixClient.RadixV1().RadixApplications(ra.Namespace).Get(context.Background(), ra.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, radixv1.RadixApplicationReconcileSucceeded, ra.Status.ReconcileStatus)
	assert.Empty(t, ra.Status.Message)
	assert.Equal(t, expectedGen, ra.Status.ObservedGeneration)
	assert.False(t, ra.Status.Reconciled.IsZero())

	// Sync with error
	errorMsg := "any sync error"
	client.PrependReactor("*", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New(errorMsg)
	})
	ra.Generation++
	expectedGen = ra.Generation
	sut = applicationconfig.NewApplicationConfig(client, kubeUtil, radixClient, rr, ra, nil)
	err = sut.OnSync(context.Background())
	require.ErrorContains(t, err, errorMsg)
	ra, err = radixClient.RadixV1().RadixApplications(ra.Namespace).Get(context.Background(), ra.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, radixv1.RadixApplicationReconcileFailed, ra.Status.ReconcileStatus)
	assert.Contains(t, ra.Status.Message, errorMsg)
	assert.Equal(t, expectedGen, ra.Status.ObservedGeneration)
	assert.False(t, ra.Status.Reconciled.IsZero())
}

func Test_Create_Radix_Environments(t *testing.T) {
	_, client, kubeUtil, radixClient := setupTest(t)

	radixRegistration, _ := utils.GetRadixRegistrationFromFile(sampleRegistration)
	radixApp, _ := utils.GetRadixApplicationFromFile(sampleApp)
	app := applicationconfig.NewApplicationConfig(client, kubeUtil, radixClient, radixRegistration, radixApp, nil)
	_, err := radixClient.RadixV1().RadixApplications(radixApp.Namespace).Create(context.Background(), radixApp, metav1.CreateOptions{})
	require.NoError(t, err)
	label := fmt.Sprintf("%s=%s", kube.RadixAppLabel, radixRegistration.Name)
	t.Run("It can create environments", func(t *testing.T) {
		err := app.OnSync(context.Background())
		assert.NoError(t, err)
		environments, _ := radixClient.RadixV1().RadixEnvironments().List(
			context.Background(),
			metav1.ListOptions{
				LabelSelector: label,
			})
		assert.Len(t, environments.Items, 2)
	})

	t.Run("It doesn't fail when re-running creation", func(t *testing.T) {
		err := app.OnSync(context.Background())
		assert.NoError(t, err)
		environments, _ := radixClient.RadixV1().RadixEnvironments().List(
			context.Background(),
			metav1.ListOptions{
				LabelSelector: label,
			})
		assert.Len(t, environments.Items, 2)
	})
}

func Test_Reconciles_Radix_Environments(t *testing.T) {
	// Setup
	_, client, kubeUtil, radixClient := setupTest(t)

	// Create environments manually
	_, err := radixClient.RadixV1().RadixEnvironments().Create(
		context.Background(),
		&radixv1.RadixEnvironment{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "any-app-qa",
				Labels: labels.Set{kube.RadixAppLabel: "any-app"},
			},
		},
		metav1.CreateOptions{})
	assert.NoError(t, err)

	_, err = radixClient.RadixV1().RadixEnvironments().Create(
		context.Background(),
		&radixv1.RadixEnvironment{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "any-app-prod",
				Labels: labels.Set{kube.RadixAppLabel: "any-app"},
			},
		},
		metav1.CreateOptions{})
	assert.NoError(t, err)

	adGroups := []string{"5678-91011-1234", "9876-54321-0987"}
	rr := utils.NewRegistrationBuilder().
		WithName("any-app").
		WithAdGroups(adGroups).
		BuildRR()

	ra := utils.NewRadixApplicationBuilder().
		WithAppName("any-app").
		WithEnvironment("qa", "development").
		WithEnvironment("prod", "master").
		BuildRA()
	_, err = radixClient.RadixV1().RadixApplications(ra.Namespace).Create(context.Background(), ra, metav1.CreateOptions{})
	require.NoError(t, err)

	app := applicationconfig.NewApplicationConfig(client, kubeUtil, radixClient, rr, ra, nil)
	label := fmt.Sprintf("%s=%s", kube.RadixAppLabel, rr.Name)

	// Test
	err = app.OnSync(context.Background())
	assert.NoError(t, err)
	environments, err := radixClient.RadixV1().RadixEnvironments().List(
		context.Background(),
		metav1.ListOptions{
			LabelSelector: label,
		})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(environments.Items))
}

func TestIsThereAnythingToDeploy_multipleEnvsToOneBranch_ListsBoth(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "master").
		WithEnvironment("prod", "master").
		BuildRA()

	targetEnvs, _, _ := applicationconfig.GetTargetEnvironments(branch, "branch", ra, false)
	assert.ElementsMatch(t, []string{"prod", "qa"}, targetEnvs)
}

func TestIsThereAnythingToDeploy_FromType(t *testing.T) {
	const (
		branch1 = "branch1"
		branch2 = "branch2"
		tag1    = "v1.0.0"
		env1    = "env1"
		env2    = "env2"
		env3    = "env3"
	)
	scenarios := map[string]struct {
		branch, gitRef, gitRefType string
		raBuilder                  func() utils.ApplicationBuilder
		expectedTargetEnvs         []string
	}{
		"matching branch, empty FromType": {
			gitRef:     branch1,
			gitRefType: string(radixv1.GitRefBranch),
			raBuilder: func() utils.ApplicationBuilder {
				return utils.NewRadixApplicationBuilder().
					WithEnvironmentByBuild(env1, radixv1.EnvBuild{From: branch1})
			},
			expectedTargetEnvs: []string{env1},
		},
		"not matching branch, empty FromType": {
			gitRef:     branch1,
			gitRefType: string(radixv1.GitRefBranch),
			raBuilder: func() utils.ApplicationBuilder {
				return utils.NewRadixApplicationBuilder().
					WithEnvironmentByBuild(env1, radixv1.EnvBuild{From: branch2})
			},
			expectedTargetEnvs: nil,
		},
		"matching branch, FromType branch": {
			gitRef:     branch1,
			gitRefType: string(radixv1.GitRefBranch),
			raBuilder: func() utils.ApplicationBuilder {
				return utils.NewRadixApplicationBuilder().
					WithEnvironmentByBuild(env1, radixv1.EnvBuild{From: branch1}).
					WithEnvironmentByBuild(env2, radixv1.EnvBuild{From: branch1, FromType: string(radixv1.GitRefBranch)})
			},
			expectedTargetEnvs: []string{env1, env2},
		},
		"matching regex branch, FromType branch": {
			gitRef:     "feature-1.0.0",
			gitRefType: string(radixv1.GitRefBranch),
			raBuilder: func() utils.ApplicationBuilder {
				return utils.NewRadixApplicationBuilder().
					WithEnvironmentByBuild(env1, radixv1.EnvBuild{From: "feature.*"}).
					WithEnvironmentByBuild(env2, radixv1.EnvBuild{From: "feature.*", FromType: string(radixv1.GitRefBranch)}).
					WithEnvironmentByBuild(env3, radixv1.EnvBuild{From: "feature.*", FromType: string(radixv1.GitRefTag)})
			},
			expectedTargetEnvs: []string{env1, env2},
		},
		"matching tag, FromType tag": {
			gitRef:     tag1,
			gitRefType: string(radixv1.GitRefTag),
			raBuilder: func() utils.ApplicationBuilder {
				return utils.NewRadixApplicationBuilder().
					WithEnvironmentByBuild(env1, radixv1.EnvBuild{From: tag1}).
					WithEnvironmentByBuild(env2, radixv1.EnvBuild{From: tag1, FromType: string(radixv1.GitRefBranch)}).
					WithEnvironmentByBuild(env3, radixv1.EnvBuild{From: tag1, FromType: string(radixv1.GitRefTag)})
			},
			expectedTargetEnvs: []string{env1, env3},
		},
		"matching regex tag, FromType tag": {
			gitRef:     "v1.0.0",
			gitRefType: string(radixv1.GitRefTag),
			raBuilder: func() utils.ApplicationBuilder {
				return utils.NewRadixApplicationBuilder().
					WithEnvironmentByBuild(env1, radixv1.EnvBuild{From: "v1.*"}).
					WithEnvironmentByBuild(env2, radixv1.EnvBuild{From: "v1.*", FromType: string(radixv1.GitRefBranch)}).
					WithEnvironmentByBuild(env3, radixv1.EnvBuild{From: "v1.*", FromType: string(radixv1.GitRefTag)})
			},
			expectedTargetEnvs: []string{env1, env3},
		},
	}

	for name, ts := range scenarios {
		t.Run(name, func(t *testing.T) {
			ra := ts.raBuilder().BuildRA()
			targetEnvs, _, _ := applicationconfig.GetTargetEnvironments(ts.gitRef, ts.gitRefType, ra, false)
			assert.ElementsMatch(t, ts.expectedTargetEnvs, targetEnvs, "mismatched target environments")
		})
	}
}

func TestIsThereAnythingToDeploy_multipleEnvsToOneBranchOtherBranchIsChanged_ListsBothButNoneIsBuilding(t *testing.T) {
	branch := "development"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "master").
		WithEnvironment("prod", "master").
		BuildRA()

	targetEnvs, _, _ := applicationconfig.GetTargetEnvironments(branch, "branch", ra, false)
	assert.Equal(t, 0, len(targetEnvs))
}

func TestIsThereAnythingToDeploy_oneEnvToOneBranch_ListsBothButOnlyOneShouldBeBuilt(t *testing.T) {
	branch := "development"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "development").
		WithEnvironment("prod", "master").
		BuildRA()

	targetEnvs, _, _ := applicationconfig.GetTargetEnvironments(branch, "branch", ra, false)
	assert.ElementsMatch(t, []string{"qa"}, targetEnvs)
}

func TestIsThereAnythingToDeploy_twoEnvNoBranch(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironmentNoBranch("qa").
		WithEnvironmentNoBranch("prod").
		BuildRA()

	targetEnvs, _, _ := applicationconfig.GetTargetEnvironments(branch, "branch", ra, false)
	assert.Equal(t, 0, len(targetEnvs))
}

func TestIsThereAnythingToDeploy_NoEnv(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		BuildRA()

	targetEnvs, _, _ := applicationconfig.GetTargetEnvironments(branch, "branch", ra, false)
	assert.Equal(t, 0, len(targetEnvs))
}

func Test_TargetEnvironmentsForGitRefs(t *testing.T) {
	const (
		env1          = "env1"
		env2          = "env2"
		env3          = "env3"
		branch1       = "branch1"
		branch2       = "branch2"
		tag1          = "tag1"
		refTypeBranch = "branch"
		refTypeTag    = "tag"
	)
	scenarios := map[string]struct {
		gitRef, gitRefType            string
		triggeredFromWebhook          bool
		raBuilder                     func() utils.ApplicationBuilder
		expectedTargetEnvironments    []string
		expectedIgnoredForWebhookEnvs []string
		expectedIgnoredForGitRefType  []string
	}{
		"one env for one branch": {
			raBuilder: func() utils.ApplicationBuilder {
				return utils.NewRadixApplicationBuilder().
					WithEnvironment(env1, branch1).
					WithEnvironment(env2, branch2)
			},
			gitRef:                     branch1,
			gitRefType:                 refTypeBranch,
			triggeredFromWebhook:       true,
			expectedTargetEnvironments: []string{env1},
		},
		"two envs for one branch": {
			raBuilder: func() utils.ApplicationBuilder {
				return utils.NewRadixApplicationBuilder().
					WithEnvironment(env1, branch1).
					WithEnvironment(env2, branch1)
			},
			gitRef:                     branch1,
			gitRefType:                 refTypeBranch,
			triggeredFromWebhook:       true,
			expectedTargetEnvironments: []string{env1, env2},
		},
		"disable webhook for one env": {
			raBuilder: func() utils.ApplicationBuilder {
				return utils.NewRadixApplicationBuilder().
					WithEnvironmentByBuild(env1, radixv1.EnvBuild{From: branch1}).
					WithEnvironmentByBuild(env2, radixv1.EnvBuild{From: branch1, WebhookEnabled: pointers.Ptr(true)}).
					WithEnvironmentByBuild(env3, radixv1.EnvBuild{From: branch1, WebhookEnabled: pointers.Ptr(false)})
			},
			gitRef:                        branch1,
			triggeredFromWebhook:          true,
			expectedTargetEnvironments:    []string{env1, env2},
			expectedIgnoredForWebhookEnvs: []string{env3},
		},
		"triggeredFromWebhook for branch gitRefType": {
			raBuilder: func() utils.ApplicationBuilder {
				return utils.NewRadixApplicationBuilder().
					WithEnvironmentByBuild(env1, radixv1.EnvBuild{From: branch1, FromType: ""}).
					WithEnvironmentByBuild(env2, radixv1.EnvBuild{From: branch1, FromType: refTypeBranch}).
					WithEnvironmentByBuild(env3, radixv1.EnvBuild{From: branch1, FromType: refTypeTag})
			},
			gitRef:                       branch1,
			gitRefType:                   refTypeBranch,
			triggeredFromWebhook:         true,
			expectedTargetEnvironments:   []string{env1, env2},
			expectedIgnoredForGitRefType: []string{env3},
		},
		"triggeredFromWebhook for tag gitRefType": {
			raBuilder: func() utils.ApplicationBuilder {
				return utils.NewRadixApplicationBuilder().
					WithEnvironmentByBuild(env1, radixv1.EnvBuild{From: tag1, FromType: ""}).
					WithEnvironmentByBuild(env2, radixv1.EnvBuild{From: tag1, FromType: refTypeBranch}).
					WithEnvironmentByBuild(env3, radixv1.EnvBuild{From: tag1, FromType: refTypeTag})
			},
			gitRef:                       tag1,
			gitRefType:                   refTypeTag,
			triggeredFromWebhook:         true,
			expectedTargetEnvironments:   []string{env1, env3},
			expectedIgnoredForGitRefType: []string{env2},
		},
		"not triggeredFromWebhook for branch gitRefType": {
			raBuilder: func() utils.ApplicationBuilder {
				return utils.NewRadixApplicationBuilder().
					WithEnvironmentByBuild(env1, radixv1.EnvBuild{From: branch1, FromType: ""}).
					WithEnvironmentByBuild(env2, radixv1.EnvBuild{From: branch1, FromType: refTypeBranch}).
					WithEnvironmentByBuild(env3, radixv1.EnvBuild{From: branch1, FromType: refTypeTag})
			},
			gitRef:                       branch1,
			gitRefType:                   refTypeBranch,
			triggeredFromWebhook:         false,
			expectedTargetEnvironments:   []string{env1, env2},
			expectedIgnoredForGitRefType: []string{env3},
		},
		"not triggeredFromWebhook for tag gitRefType": {
			raBuilder: func() utils.ApplicationBuilder {
				return utils.NewRadixApplicationBuilder().
					WithEnvironmentByBuild(env1, radixv1.EnvBuild{From: tag1, FromType: ""}).
					WithEnvironmentByBuild(env2, radixv1.EnvBuild{From: tag1, FromType: refTypeBranch}).
					WithEnvironmentByBuild(env3, radixv1.EnvBuild{From: tag1, FromType: refTypeTag})
			},
			gitRef:                       tag1,
			gitRefType:                   refTypeTag,
			triggeredFromWebhook:         false,
			expectedTargetEnvironments:   []string{env1, env3},
			expectedIgnoredForGitRefType: []string{env2},
		},
	}

	for name, ts := range scenarios {
		t.Run(name, func(t *testing.T) {
			ra := ts.raBuilder().BuildRA()
			targetEnvs, ignoredForWebhookEnvs, ignoredForGitRefsType := applicationconfig.GetTargetEnvironments(ts.gitRef, ts.gitRefType, ra, ts.triggeredFromWebhook)
			assert.ElementsMatch(t, ts.expectedTargetEnvironments, targetEnvs, "mismatched target environments")
			assert.ElementsMatch(t, ts.expectedIgnoredForWebhookEnvs, ignoredForWebhookEnvs, "mismatched ignored for webhook environments")
			assert.ElementsMatch(t, ts.expectedIgnoredForGitRefType, ignoredForGitRefsType, "mismatched ignored for git refs type environments")
		})
	}
}

func TestIsThereAnythingToDeploy_promotionScheme_ListsBothButOnlyOneShouldBeBuilt(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "master").
		WithEnvironment("prod", "").
		BuildRA()

	targetEnvs, _, _ := applicationconfig.GetTargetEnvironments(branch, "branch", ra, false)
	assert.ElementsMatch(t, []string{"qa"}, targetEnvs)
}

func TestIsThereAnythingToDeploy_wildcardMatch_ListsBothButOnlyOneShouldBeBuilt(t *testing.T) {
	branch := "feature/RA-123-Test"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("feature", "feature/*").
		WithEnvironment("prod", "master").
		BuildRA()

	targetEnvs, _, _ := applicationconfig.GetTargetEnvironments(branch, "branch", ra, false)
	assert.ElementsMatch(t, []string{"feature"}, targetEnvs)
}

func Test_WithBuildSecretsSet_SecretsCorrectlyAdded(t *testing.T) {
	tu, client, kubeUtil, radixClient := setupTest(t)

	appNamespace := "any-app-app"
	err := applyApplicationWithSync(tu, client, kubeUtil, radixClient,
		utils.ARadixApplication().
			WithAppName("any-app").
			WithEnvironment("dev", "master").
			WithBuildSecrets("secret1", "secret2"))
	require.NoError(t, err)

	secrets, _ := client.CoreV1().Secrets(appNamespace).List(context.Background(), metav1.ListOptions{})
	defaultValue := []byte(defaults.BuildSecretDefaultData)

	buildSecrets := getSecretByName(defaults.BuildSecretsName, secrets)
	assert.NotNil(t, buildSecrets)
	assert.Equal(t, 2, len(buildSecrets.Data))
	assert.Equal(t, defaultValue, buildSecrets.Data["secret1"])
	assert.Equal(t, defaultValue, buildSecrets.Data["secret2"])

	roles, _ := client.RbacV1().Roles(appNamespace).List(context.Background(), metav1.ListOptions{})
	assert.True(t, roleByNameExists("radix-app-admin-build-secrets", roles))
	assert.True(t, roleByNameExists("pipeline-build-secrets", roles))

	rolebindings, _ := client.RbacV1().RoleBindings(appNamespace).List(context.Background(), metav1.ListOptions{})
	assert.True(t, roleBindingByNameExists("radix-app-admin-build-secrets", rolebindings))
	assert.True(t, roleBindingByNameExists("pipeline-build-secrets", rolebindings))

	err = applyApplicationWithSync(tu, client, kubeUtil, radixClient,
		utils.ARadixApplication().
			WithAppName("any-app").
			WithEnvironment("dev", "master").
			WithBuildSecrets("secret4", "secret5", "secret6"))
	require.NoError(t, err)

	secrets, _ = client.CoreV1().Secrets(appNamespace).List(context.Background(), metav1.ListOptions{})
	buildSecrets = getSecretByName(defaults.BuildSecretsName, secrets)
	assert.Equal(t, 3, len(buildSecrets.Data))
	assert.Equal(t, defaultValue, buildSecrets.Data["secret4"])
	assert.Equal(t, defaultValue, buildSecrets.Data["secret5"])
	assert.Equal(t, defaultValue, buildSecrets.Data["secret6"])

}

func Test_SubPipelineServiceAccountsCorrectlySynced(t *testing.T) {
	tu, client, kubeUtil, radixclient := setupTest(t)

	appNamespace := "any-app-app"
	// Already includes a "test" environment
	err := applyApplicationWithSync(tu, client, kubeUtil, radixclient,
		utils.ARadixApplication().
			WithAppName("any-app"))
	require.NoError(t, err)

	accounts, err := client.CoreV1().ServiceAccounts(appNamespace).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)

	saTest := getServiceAccountByName(utils.GetSubPipelineServiceAccountName("test"), accounts)
	assert.NotNil(t, saTest)
	assert.Equal(t, "true", saTest.Labels[kube.IsServiceAccountForSubPipelineLabel])
	assert.Equal(t, "test", saTest.Labels[kube.RadixEnvLabel])

	saProd := getServiceAccountByName(utils.GetSubPipelineServiceAccountName("prod"), accounts)
	assert.Nil(t, saProd)

	err = applyApplicationWithSync(tu, client, kubeUtil, radixclient,
		utils.ARadixApplication().
			WithAppName("any-app").
			WithEnvironment("prod", "release"))
	require.NoError(t, err)

	accounts, _ = client.CoreV1().ServiceAccounts(appNamespace).List(context.Background(), metav1.ListOptions{})
	saProd = getServiceAccountByName(utils.GetSubPipelineServiceAccountName("prod"), accounts)
	assert.NotNil(t, saProd)
	assert.Equal(t, "true", saProd.Labels[kube.IsServiceAccountForSubPipelineLabel])
	assert.Equal(t, "prod", saProd.Labels[kube.RadixEnvLabel])
}
func Test_SubPipelineServiceAccountsCorrectlyDeleted(t *testing.T) {
	tu, client, kubeUtil, radixclient := setupTest(t)

	appNamespace := "any-app-app"
	// Already includes a "test" environment
	err := applyApplicationWithSync(tu, client, kubeUtil, radixclient,
		utils.ARadixApplication().
			WithAppName("any-app").
			WithEnvironment("prod", "release"))
	require.NoError(t, err)

	accounts, err := client.CoreV1().ServiceAccounts(appNamespace).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)

	saTest := getServiceAccountByName(utils.GetSubPipelineServiceAccountName("test"), accounts)
	assert.NotNil(t, saTest)

	saProd := getServiceAccountByName(utils.GetSubPipelineServiceAccountName("prod"), accounts)
	assert.NotNil(t, saProd)

	// Already includes a "test" environment
	err = applyApplicationWithSync(tu, client, kubeUtil, radixclient,
		utils.ARadixApplication().
			WithAppName("any-app").
			WithEnvironment("dev", "master"))
	require.NoError(t, err)

	accounts, _ = client.CoreV1().ServiceAccounts(appNamespace).List(context.Background(), metav1.ListOptions{})

	saTest = getServiceAccountByName(utils.GetSubPipelineServiceAccountName("test"), accounts)
	assert.NotNil(t, saTest)

	saProd = getServiceAccountByName(utils.GetSubPipelineServiceAccountName("prod"), accounts)
	assert.Nil(t, saProd)
}

func Test_WithBuildSecretsDeleted_SecretsCorrectlyDeleted(t *testing.T) {
	tu, client, kubeUtil, radixClient := setupTest(t)

	err := applyApplicationWithSync(tu, client, kubeUtil, radixClient,
		utils.ARadixApplication().
			WithAppName("any-app").
			WithEnvironment("dev", "master").
			WithBuildSecrets("secret1", "secret2"))
	require.NoError(t, err)

	// Delete secret
	appNamespace := "any-app-app"
	err = applyApplicationWithSync(tu, client, kubeUtil, radixClient,
		utils.ARadixApplication().
			WithAppName("any-app").
			WithEnvironment("dev", "master").
			WithBuildSecrets("secret2"))
	require.NoError(t, err)

	secrets, _ := client.CoreV1().Secrets(appNamespace).List(context.Background(), metav1.ListOptions{})
	defaultValue := []byte(defaults.BuildSecretDefaultData)

	buildSecrets := getSecretByName(defaults.BuildSecretsName, secrets)
	assert.NotNil(t, buildSecrets)
	assert.Equal(t, 1, len(buildSecrets.Data))
	assert.Nil(t, buildSecrets.Data["secret1"])
	assert.Equal(t, defaultValue, buildSecrets.Data["secret2"])

	// Delete secret
	err = applyApplicationWithSync(tu, client, kubeUtil, radixClient,
		utils.ARadixApplication().
			WithAppName("any-app").
			WithEnvironment("dev", "master").
			WithBuildSecrets())
	require.NoError(t, err)

	// Secret is deleted
	secrets, _ = client.CoreV1().Secrets(appNamespace).List(context.Background(), metav1.ListOptions{})
	assert.False(t, secretByNameExists(defaults.BuildSecretsName, secrets))

	roles, _ := client.RbacV1().Roles(appNamespace).List(context.Background(), metav1.ListOptions{})
	assert.False(t, roleByNameExists("radix-app-admin-build-secrets", roles))
	assert.False(t, roleByNameExists("radix-app-reader-build-secrets", roles))
	assert.False(t, roleByNameExists("pipeline-build-secrets", roles))

	rolebindings, _ := client.RbacV1().RoleBindings(appNamespace).List(context.Background(), metav1.ListOptions{})
	assert.False(t, roleBindingByNameExists("radix-app-admin-build-secrets", rolebindings))
	assert.False(t, roleBindingByNameExists("radix-app-reader-build-secrets", rolebindings))
	assert.False(t, roleBindingByNameExists("pipeline-build-secrets", rolebindings))
}

func Test_AppReaderBuildSecretsRoleAndRoleBindingExists(t *testing.T) {
	tu, client, kubeUtil, radixClient := setupTest(t)

	err := applyApplicationWithSync(tu, client, kubeUtil, radixClient,
		utils.ARadixApplication().
			WithAppName("any-app").
			WithEnvironment("dev", "master").
			WithBuildSecrets("secret1", "secret2"))
	require.NoError(t, err)

	roles, _ := client.RbacV1().Roles("any-app-app").List(context.Background(), metav1.ListOptions{})
	assert.True(t, roleByNameExists("radix-app-reader-build-secrets", roles))

	rolebindings, _ := client.RbacV1().RoleBindings("any-app-app").List(context.Background(), metav1.ListOptions{})
	assert.True(t, roleBindingByNameExists("radix-app-reader-build-secrets", rolebindings))

	// Delete secret and verify that role and rolebinding is deleted
	err = applyApplicationWithSync(tu, client, kubeUtil, radixClient,
		utils.ARadixApplication().
			WithAppName("any-app").
			WithEnvironment("dev", "master"))
	require.NoError(t, err)

	roles, _ = client.RbacV1().Roles("any-app-app").List(context.Background(), metav1.ListOptions{})
	assert.False(t, roleByNameExists("radix-app-reader-build-secrets", roles))

	rolebindings, _ = client.RbacV1().RoleBindings("any-app-app").List(context.Background(), metav1.ListOptions{})
	assert.False(t, roleBindingByNameExists("radix-app-reader-build-secrets", rolebindings))
}

func Test_PrivateImageHubSecret_RoleAndRoleBinding(t *testing.T) {
	tu, client, kubeUtil, radixClient := setupTest(t)

	adminGroups, adminUsers := []string{"admin1", "admin2"}, []string{"adminUser1", "adminUser2"}
	readerGroups, readerUsers := []string{"reader1", "reader2"}, []string{"readerUser1", "readerUser2"}
	err := applyApplicationWithSync(tu, client, kubeUtil, radixClient,
		utils.ARadixApplication().
			WithRadixRegistration(
				utils.ARadixRegistration().
					WithAdGroups(adminGroups).
					WithAdUsers(adminUsers).
					WithReaderAdGroups(readerGroups).
					WithReaderAdUsers(readerUsers)).
			WithAppName("any-app").
			WithEnvironment("dev", "master"))
	require.NoError(t, err)

	type testSpec struct {
		roleName         string
		expectedVerbs    []string
		expectedSubjects []string
	}
	tests := []testSpec{
		{roleName: "radix-private-image-hubs-reader", expectedVerbs: []string{"get", "list", "watch"}, expectedSubjects: slices.Concat(readerGroups, readerUsers)},
		{roleName: "radix-private-image-hubs", expectedVerbs: []string{"get", "list", "watch", "update", "patch", "delete"}, expectedSubjects: slices.Concat(adminGroups, adminUsers)},
	}

	roles, _ := client.RbacV1().Roles("any-app-app").List(context.Background(), metav1.ListOptions{})
	rolebindings, _ := client.RbacV1().RoleBindings("any-app-app").List(context.Background(), metav1.ListOptions{})

	for _, test := range tests {
		t.Run(test.roleName, func(t *testing.T) {
			expectedRules := []rbacv1.PolicyRule{
				{Verbs: test.expectedVerbs, Resources: []string{"secrets"}, APIGroups: []string{""}, ResourceNames: []string{"radix-private-image-hubs"}},
			}
			assert.True(t, roleByNameExists(test.roleName, roles))
			assert.ElementsMatch(t, expectedRules, getRoleByName(test.roleName, roles).Rules)
			assert.True(t, roleBindingByNameExists(test.roleName, rolebindings))
			expectedRoleRef := rbacv1.RoleRef{Kind: "Role", APIGroup: "rbac.authorization.k8s.io", Name: test.roleName}
			assert.Equal(t, expectedRoleRef, getRoleBindingByName(test.roleName, rolebindings).RoleRef)
			actualSubjectNames := slice.Map(getRoleBindingByName(test.roleName, rolebindings).Subjects, func(s rbacv1.Subject) string { return s.Name })
			assert.ElementsMatch(t, test.expectedSubjects, actualSubjectNames)
		})
	}
}

func Test_PrivateImageHubSecret_LabelsAndAnnotations(t *testing.T) {
	tu, client, kubeUtil, radixClient := setupTest(t)
	err := applyRadixAppWithPrivateImageHub(tu, client, kubeUtil, radixClient, radixv1.PrivateImageHubEntries{})
	require.NoError(t, err)

	secret, err := client.CoreV1().Secrets("any-app-app").Get(context.Background(), defaults.PrivateImageHubSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	expectedLabels := map[string]string{kube.RadixAppLabel: "any-app"}
	assert.Equal(t, expectedLabels, secret.ObjectMeta.Labels)
	expectedAnnotations := map[string]string{"replicator.v1.mittwald.de/replicate-to-matching": "radix-private-image-hubs-sync=any-app"}
	assert.Equal(t, expectedAnnotations, secret.ObjectMeta.Annotations)
}

func Test_PrivateImageHubSecret_KeepCustomAnnotations(t *testing.T) {
	tu, client, kubeUtil, radixClient := setupTest(t)
	err := applyRadixAppWithPrivateImageHub(tu, client, kubeUtil, radixClient,
		radixv1.PrivateImageHubEntries{
			"privaterepodeleteme.azurecr.io": &radixv1.RadixPrivateImageHubCredential{
				Username: "814607e6-3d71-44a7-8476-50e8b281abbc",
				Email:    "radix@equinor.com",
			},
		})
	require.NoError(t, err)

	secret, err := client.CoreV1().Secrets("any-app-app").Get(context.Background(), defaults.PrivateImageHubSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	secret.Annotations["any-annotation"] = "any-value"
	_, err = client.CoreV1().Secrets("any-app-app").Update(context.Background(), secret, metav1.UpdateOptions{})
	require.NoError(t, err)

	err = applyRadixAppWithPrivateImageHub(tu, client, kubeUtil, radixClient,
		radixv1.PrivateImageHubEntries{
			"privaterepodeleteme.azurecr.io": &radixv1.RadixPrivateImageHubCredential{
				Username: "814607e6-3d71-44a7-8476-50e8b281abbc",
				Email:    "radix@equinor.com",
			},
		})
	require.NoError(t, err)
	secret, err = client.CoreV1().Secrets("any-app-app").Get(context.Background(), defaults.PrivateImageHubSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	expectedAnnotations := map[string]string{
		"replicator.v1.mittwald.de/replicate-to-matching": "radix-private-image-hubs-sync=any-app",
		"any-annotation": "any-value",
	}
	assert.Equal(t, expectedAnnotations, secret.ObjectMeta.Annotations)
}

func Test_PrivateImageHubSecret_UpdateSpec(t *testing.T) {
	tests := map[string]struct {
		initialSpec         radixv1.PrivateImageHubEntries
		initialExpectedAuth docker.Auths
		passwords           map[string]string
		updatedSpec         radixv1.PrivateImageHubEntries
		updatedExpectedAuth docker.Auths
	}{
		"password retained when resync with no changes": {
			initialSpec: radixv1.PrivateImageHubEntries{
				"registry.server.com": &radixv1.RadixPrivateImageHubCredential{
					Username: "anyuser",
					Email:    "any@email.com",
				},
			},
			initialExpectedAuth: docker.Auths{
				"registry.server.com": docker.Credential{
					Username: "anyuser",
					Email:    "any@email.com",
					Auth:     "YW55dXNlcjo=",
				},
			},
			passwords: map[string]string{"registry.server.com": "anypassword"},
			updatedSpec: radixv1.PrivateImageHubEntries{
				"registry.server.com": &radixv1.RadixPrivateImageHubCredential{
					Username: "anyuser",
					Email:    "any@email.com",
				},
			},
			updatedExpectedAuth: docker.Auths{
				"registry.server.com": docker.Credential{
					Username: "anyuser",
					Email:    "any@email.com",
					Password: "anypassword",
					Auth:     "YW55dXNlcjphbnlwYXNzd29yZA==",
				},
			},
		},
		"password retained when resync with changed username": {
			initialSpec: radixv1.PrivateImageHubEntries{
				"registry.server.com": &radixv1.RadixPrivateImageHubCredential{
					Username: "anyuser",
					Email:    "any@email.com",
				},
			},
			initialExpectedAuth: docker.Auths{
				"registry.server.com": docker.Credential{
					Username: "anyuser",
					Email:    "any@email.com",
					Auth:     "YW55dXNlcjo=",
				},
			},
			passwords: map[string]string{"registry.server.com": "anypassword"},
			updatedSpec: radixv1.PrivateImageHubEntries{
				"registry.server.com": &radixv1.RadixPrivateImageHubCredential{
					Username: "newuser",
					Email:    "any@email.com",
				},
			},
			updatedExpectedAuth: docker.Auths{
				"registry.server.com": docker.Credential{
					Username: "newuser",
					Email:    "any@email.com",
					Password: "anypassword",
					Auth:     "bmV3dXNlcjphbnlwYXNzd29yZA==",
				},
			},
		},
		"password retained when resync with changed email": {
			initialSpec: radixv1.PrivateImageHubEntries{
				"registry.server.com": &radixv1.RadixPrivateImageHubCredential{
					Username: "anyuser",
					Email:    "any@email.com",
				},
			},
			initialExpectedAuth: docker.Auths{
				"registry.server.com": docker.Credential{
					Username: "anyuser",
					Email:    "any@email.com",
					Auth:     "YW55dXNlcjo=",
				},
			},
			passwords: map[string]string{"registry.server.com": "anypassword"},
			updatedSpec: radixv1.PrivateImageHubEntries{
				"registry.server.com": &radixv1.RadixPrivateImageHubCredential{
					Username: "anyuser",
					Email:    "new@email.com",
				},
			},
			updatedExpectedAuth: docker.Auths{
				"registry.server.com": docker.Credential{
					Username: "anyuser",
					Email:    "new@email.com",
					Password: "anypassword",
					Auth:     "YW55dXNlcjphbnlwYXNzd29yZA==",
				},
			},
		},
		"password cleared when resync with changed server": {
			initialSpec: radixv1.PrivateImageHubEntries{
				"registry.server.com": &radixv1.RadixPrivateImageHubCredential{
					Username: "anyuser",
					Email:    "any@email.com",
				},
			},
			initialExpectedAuth: docker.Auths{
				"registry.server.com": docker.Credential{
					Username: "anyuser",
					Email:    "any@email.com",
					Auth:     "YW55dXNlcjo=",
				},
			},
			passwords: map[string]string{"registry.server.com": "anypassword"},
			updatedSpec: radixv1.PrivateImageHubEntries{
				"newregistry.server.com": &radixv1.RadixPrivateImageHubCredential{
					Username: "anyuser",
					Email:    "any@email.com",
				},
			},
			updatedExpectedAuth: docker.Auths{
				"newregistry.server.com": docker.Credential{
					Username: "anyuser",
					Email:    "any@email.com",
					Auth:     "YW55dXNlcjo=",
				},
			},
		},
		"password retained when adding and removing servers": {
			initialSpec: radixv1.PrivateImageHubEntries{
				"registry.server.com": &radixv1.RadixPrivateImageHubCredential{
					Username: "anyuser",
					Email:    "any@email.com",
				},
				"oldregistry.server.com": &radixv1.RadixPrivateImageHubCredential{
					Username: "olduser",
					Email:    "old@email.com",
				},
			},
			initialExpectedAuth: docker.Auths{
				"registry.server.com": docker.Credential{
					Username: "anyuser",
					Email:    "any@email.com",
					Auth:     "YW55dXNlcjo=",
				},
				"oldregistry.server.com": docker.Credential{
					Username: "olduser",
					Email:    "old@email.com",
					Auth:     "b2xkdXNlcjo=",
				},
			},
			passwords: map[string]string{"registry.server.com": "anypassword"},
			updatedSpec: radixv1.PrivateImageHubEntries{
				"registry.server.com": &radixv1.RadixPrivateImageHubCredential{
					Username: "anyuser",
					Email:    "any@email.com",
				},
				"newregistry.server.com": &radixv1.RadixPrivateImageHubCredential{
					Username: "newuser",
					Email:    "new@email.com",
				},
			},
			updatedExpectedAuth: docker.Auths{
				"registry.server.com": docker.Credential{
					Username: "anyuser",
					Email:    "any@email.com",
					Password: "anypassword",
					Auth:     "YW55dXNlcjphbnlwYXNzd29yZA==",
				},
				"newregistry.server.com": docker.Credential{
					Username: "newuser",
					Email:    "new@email.com",
					Auth:     "bmV3dXNlcjo=",
				},
			},
		},
	}

	for testName, testSpec := range tests {
		t.Run(testName, func(t *testing.T) {
			tu, client, kubeUtil, radixClient := setupTest(t)

			// Check initial private image hub spec
			err := applyRadixAppWithPrivateImageHub(tu, client, kubeUtil, radixClient, testSpec.initialSpec)
			require.NoError(t, err)
			initialSecret, err := client.CoreV1().Secrets("any-app-app").Get(context.Background(), defaults.PrivateImageHubSecretName, metav1.GetOptions{})
			require.NoError(t, err)
			initialActualAuthConfig, err := applicationconfig.UnmarshalPrivateImageHubAuthConfig(initialSecret.Data[corev1.DockerConfigJsonKey])
			require.NoError(t, err)
			assert.Equal(t, testSpec.initialExpectedAuth, initialActualAuthConfig.Auths)

			// Update secret with passwords
			for server, cred := range initialActualAuthConfig.Auths {
				if pwd, found := testSpec.passwords[server]; found {
					cred.Password = pwd
					initialActualAuthConfig.Auths[server] = cred
				}
			}
			authConfigData, err := applicationconfig.MarshalPrivateImageHubAuthConfig(initialActualAuthConfig)
			require.NoError(t, err)
			initialSecret.Data[corev1.DockerConfigJsonKey] = authConfigData
			_, err = client.CoreV1().Secrets("any-app-app").Update(context.Background(), initialSecret, metav1.UpdateOptions{})
			require.NoError(t, err)

			// Check updated private image hub spec
			err = applyRadixAppWithPrivateImageHub(tu, client, kubeUtil, radixClient, testSpec.updatedSpec)
			require.NoError(t, err)
			updatedSecret, err := client.CoreV1().Secrets("any-app-app").Get(context.Background(), defaults.PrivateImageHubSecretName, metav1.GetOptions{})
			require.NoError(t, err)
			updatedActualAuthConfig, err := applicationconfig.UnmarshalPrivateImageHubAuthConfig(updatedSecret.Data[corev1.DockerConfigJsonKey])
			require.NoError(t, err)
			assert.Equal(t, testSpec.updatedExpectedAuth, updatedActualAuthConfig.Auths)
		})
	}
}

func Test_RadixEnvironment(t *testing.T) {
	tu, client, kubeUtil, radixClient := setupTest(t)

	err := applyApplicationWithSync(tu, client, kubeUtil, radixClient,
		utils.ARadixApplication().
			WithAppName("any-app"))
	require.NoError(t, err)

	rr, _ := radixClient.RadixV1().RadixRegistrations().Get(context.Background(), "any-app", metav1.GetOptions{})

	environments, err := radixClient.RadixV1().RadixEnvironments().List(context.Background(), metav1.ListOptions{})

	t.Run("It creates a single environment", func(t *testing.T) {
		assert.NoError(t, err)
		assert.Len(t, environments.Items, 1)
	})

	t.Run("Environment has a correct name", func(t *testing.T) {
		assert.Equal(t, "any-app-test", environments.Items[0].GetName())
	})

	t.Run("Environment has a correct owner", func(t *testing.T) {
		assert.Equal(t, rrAsOwnerReference(rr), environments.Items[0].GetOwnerReferences())
	})

	t.Run("Environment is not orphaned", func(t *testing.T) {
		assert.False(t, environments.Items[0].Status.Orphaned)
	})
}

func Test_UseBuildKit(t *testing.T) {
	var testScenarios = []struct {
		appName             string
		useBuildKit         *bool
		expectedUseBuildKit *bool
	}{
		{
			appName:             "any-app1",
			useBuildKit:         nil,
			expectedUseBuildKit: nil,
		},
		{
			appName:             "any-app2",
			useBuildKit:         pointers.Ptr(false),
			expectedUseBuildKit: pointers.Ptr(false),
		},
		{
			appName:             "any-app3",
			useBuildKit:         pointers.Ptr(true),
			expectedUseBuildKit: pointers.Ptr(true),
		},
	}
	tu, client, kubeUtil, radixClient := setupTest(t)

	for _, testScenario := range testScenarios {
		ra := utils.ARadixApplication().WithAppName(testScenario.appName)
		if testScenario.useBuildKit != nil {
			ra = ra.WithBuildKit(testScenario.useBuildKit)
		}
		err := applyApplicationWithSync(tu, client, kubeUtil, radixClient, ra)
		require.NoError(t, err)

		raAfterSync, _ := radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(testScenario.appName)).Get(context.Background(), testScenario.appName, metav1.GetOptions{})

		var useBuildKit *bool
		if raAfterSync.Spec.Build == nil {
			useBuildKit = nil
		} else {
			useBuildKit = raAfterSync.Spec.Build.UseBuildKit
		}
		assert.Equal(t, testScenario.expectedUseBuildKit, useBuildKit)
	}
}

func Test_GetConfigBranch_notSet(t *testing.T) {
	rr := utils.NewRegistrationBuilder().
		BuildRR()

	assert.Equal(t, applicationconfig.ConfigBranchFallback, applicationconfig.GetConfigBranch(rr))
}

func Test_GetConfigBranch_set(t *testing.T) {
	configBranch := "main"

	rr := utils.NewRegistrationBuilder().
		WithConfigBranch(configBranch).
		BuildRR()

	assert.Equal(t, configBranch, applicationconfig.GetConfigBranch(rr))
}

func Test_IsConfigBranch(t *testing.T) {
	configBranch, otherBranch := "main", "master"

	rr := utils.NewRegistrationBuilder().
		WithConfigBranch(configBranch).
		BuildRR()

	t.Run("Branch is configBranch", func(t *testing.T) {
		assert.True(t, applicationconfig.IsConfigBranch(configBranch, rr))
	})

	t.Run("Branch is not configBranch", func(t *testing.T) {
		assert.False(t, applicationconfig.IsConfigBranch(otherBranch, rr))
	})
}

func rrAsOwnerReference(rr *radixv1.RadixRegistration) []metav1.OwnerReference {
	trueVar := true
	return []metav1.OwnerReference{
		{
			APIVersion: radixv1.SchemeGroupVersion.Identifier(),
			Kind:       radixv1.KindRadixRegistration,
			Name:       rr.Name,
			UID:        rr.UID,
			Controller: &trueVar,
		},
	}
}

func applyRadixAppWithPrivateImageHub(tu *test.Utils, client kubernetes.Interface, kubeUtil *kube.Kube, radixClient radixclient.Interface, privateImageHubs radixv1.PrivateImageHubEntries) error {
	appBuilder := utils.ARadixApplication().
		WithAppName("any-app").
		WithEnvironment("dev", "master")
	for key, config := range privateImageHubs {
		appBuilder.WithPrivateImageRegistry(key, config.Username, config.Email)
	}

	return applyApplicationWithSync(tu, client, kubeUtil, radixClient, appBuilder)
}

func applyApplicationWithSync(tu *test.Utils, client kubernetes.Interface, kubeUtil *kube.Kube, radixClient radixclient.Interface, applicationBuilder utils.ApplicationBuilder) error {

	ra, err := tu.ApplyApplication(applicationBuilder)
	if err != nil {
		return err
	}

	radixRegistration, err := radixClient.RadixV1().RadixRegistrations().Get(context.Background(), ra.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	applicationConfig := applicationconfig.NewApplicationConfig(client, kubeUtil, radixClient, radixRegistration, ra, nil)

	err = applicationConfig.OnSync(context.Background())
	if err != nil {
		return err
	}

	return nil
}

func getServiceAccountByName(name string, serviceAccounts *corev1.ServiceAccountList) *corev1.ServiceAccount {
	for _, sa := range serviceAccounts.Items {
		if sa.Name == name {
			return &sa
		}
	}

	return nil
}

func getSecretByName(name string, secrets *corev1.SecretList) *corev1.Secret {
	for _, secret := range secrets.Items {
		if secret.Name == name {
			return &secret
		}
	}

	return nil
}

func secretByNameExists(name string, secrets *corev1.SecretList) bool {
	return getSecretByName(name, secrets) != nil
}

func getRoleByName(name string, roles *rbacv1.RoleList) *rbacv1.Role {
	for _, role := range roles.Items {
		if role.Name == name {
			return &role
		}
	}

	return nil
}

func roleByNameExists(name string, roles *rbacv1.RoleList) bool {
	return getRoleByName(name, roles) != nil
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
	return getRoleBindingByName(name, roleBindings) != nil
}
