package applicationconfig_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	sampleRegistration = "./testdata/sampleregistration.yaml"
	sampleApp          = "./testdata/radixconfig.yaml"
	clusterName        = "AnyClusterName"
)

func init() {
	log.SetOutput(io.Discard)
}

func setupTest() (*test.Utils, kubernetes.Interface, *kube.Kube, radixclient.Interface) {
	kubeClient := kubefake.NewSimpleClientset()
	radixClient := radixfake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(kubeClient, radixClient, secretproviderclient)
	handlerTestUtils := test.NewTestUtils(kubeClient, radixClient, secretproviderclient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, "0.0.0.0", "anysubid")
	return &handlerTestUtils, kubeClient, kubeUtil, radixClient
}

func getApplication(ra *radixv1.RadixApplication) *applicationconfig.ApplicationConfig {
	// The other arguments are not relevant for this test
	application, _ := applicationconfig.NewApplicationConfig(nil, nil, nil, nil, ra)
	return application
}

func Test_Create_Radix_Environments(t *testing.T) {
	_, client, kubeUtil, radixClient := setupTest()

	radixRegistration, _ := utils.GetRadixRegistrationFromFile(sampleRegistration)
	radixApp, _ := utils.GetRadixApplicationFromFile(sampleApp)
	app, _ := applicationconfig.NewApplicationConfig(client, kubeUtil, radixClient, radixRegistration, radixApp)

	label := fmt.Sprintf("%s=%s", kube.RadixAppLabel, radixRegistration.Name)
	t.Run("It can create environments", func(t *testing.T) {
		err := app.OnSync()
		assert.NoError(t, err)
		environments, _ := radixClient.RadixV1().RadixEnvironments().List(
			context.TODO(),
			metav1.ListOptions{
				LabelSelector: label,
			})
		assert.Len(t, environments.Items, 2)
	})

	t.Run("It doesn't fail when re-running creation", func(t *testing.T) {
		err := app.OnSync()
		assert.NoError(t, err)
		environments, _ := radixClient.RadixV1().RadixEnvironments().List(
			context.TODO(),
			metav1.ListOptions{
				LabelSelector: label,
			})
		assert.Len(t, environments.Items, 2)
	})
}

func Test_Reconciles_Radix_Environments(t *testing.T) {
	// Setup
	_, client, kubeUtil, radixClient := setupTest()

	// Create environments manually
	radixClient.RadixV1().RadixEnvironments().Create(
		context.TODO(),
		&radixv1.RadixEnvironment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "any-app-qa",
			},
		},
		metav1.CreateOptions{})

	radixClient.RadixV1().RadixEnvironments().Create(
		context.TODO(),
		&radixv1.RadixEnvironment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "any-app-prod",
			},
		},
		metav1.CreateOptions{})

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

	app, _ := applicationconfig.NewApplicationConfig(client, kubeUtil, radixClient, rr, ra)
	label := fmt.Sprintf("%s=%s", kube.RadixAppLabel, rr.Name)

	// Test
	app.OnSync()
	environments, _ := radixClient.RadixV1().RadixEnvironments().List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: label,
		})
	assert.Equal(t, 2, len(environments.Items))
}

func TestIsThereAnythingToDeploy_multipleEnvsToOneBranch_ListsBoth(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "master").
		WithEnvironment("prod", "master").
		BuildRA()

	application := getApplication(ra)
	isThereAnythingToDeploy, targetEnvs := application.IsThereAnythingToDeploy(branch)

	assert.True(t, isThereAnythingToDeploy)
	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, targetEnvs["prod"], true)
	assert.Equal(t, targetEnvs["qa"], true)
}

func TestIsThereAnythingToDeploy_multipleEnvsToOneBranchOtherBranchIsChanged_ListsBothButNoneIsBuilding(t *testing.T) {
	branch := "development"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "master").
		WithEnvironment("prod", "master").
		BuildRA()

	application := getApplication(ra)
	isThereAnythingToDeploy, targetEnvs := application.IsThereAnythingToDeploy(branch)

	assert.False(t, isThereAnythingToDeploy)
	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, targetEnvs["prod"], false)
	assert.Equal(t, targetEnvs["qa"], false)
}

func TestIsThereAnythingToDeploy_oneEnvToOneBranch_ListsBothButOnlyOneShouldBeBuilt(t *testing.T) {
	branch := "development"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "development").
		WithEnvironment("prod", "master").
		BuildRA()

	application := getApplication(ra)
	isThereAnythingToDeploy, targetEnvs := application.IsThereAnythingToDeploy(branch)

	assert.True(t, isThereAnythingToDeploy)
	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, targetEnvs["prod"], false)
	assert.Equal(t, targetEnvs["qa"], true)
}

func TestIsThereAnythingToDeploy_twoEnvNoBranch(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironmentNoBranch("qa").
		WithEnvironmentNoBranch("prod").
		BuildRA()

	application := getApplication(ra)
	isThereAnythingToDeploy, targetEnvs := application.IsThereAnythingToDeploy(branch)

	assert.False(t, isThereAnythingToDeploy)
	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, false, targetEnvs["qa"])
	assert.Equal(t, false, targetEnvs["prod"])
}

func TestIsThereAnythingToDeploy_NoEnv(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		BuildRA()

	application := getApplication(ra)
	isThereAnythingToDeploy, targetEnvs := application.IsThereAnythingToDeploy(branch)

	assert.False(t, isThereAnythingToDeploy)
	assert.Equal(t, 0, len(targetEnvs))
}

func TestIsThereAnythingToDeploy_promotionScheme_ListsBothButOnlyOneShouldBeBuilt(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "master").
		WithEnvironment("prod", "").
		BuildRA()

	application := getApplication(ra)
	isThereAnythingToDeploy, targetEnvs := application.IsThereAnythingToDeploy(branch)

	assert.True(t, isThereAnythingToDeploy)
	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, targetEnvs["prod"], false)
	assert.Equal(t, targetEnvs["qa"], true)
}

func TestIsThereAnythingToDeploy_wildcardMatch_ListsBothButOnlyOneShouldBeBuilt(t *testing.T) {
	branch := "feature/RA-123-Test"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("feature", "feature/*").
		WithEnvironment("prod", "master").
		BuildRA()

	application := getApplication(ra)
	isThereAnythingToDeploy, targetEnvs := application.IsThereAnythingToDeploy(branch)

	assert.True(t, isThereAnythingToDeploy)
	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, targetEnvs["prod"], false)
	assert.Equal(t, targetEnvs["feature"], true)
}

func TestIsTargetEnvsEmpty_noEntry(t *testing.T) {
	ra := utils.NewRadixApplicationBuilder().BuildRA()
	hasEnvToDeploy, _ := applicationconfig.IsThereAnythingToDeployForRadixApplication("main", ra)
	assert.False(t, hasEnvToDeploy)
}

func TestIsTargetEnvsEmpty_twoEntriesWithBranchMapping(t *testing.T) {
	ra := utils.NewRadixApplicationBuilder().WithEnvironment("qa", "main").WithEnvironment("qa", "main").BuildRA()
	hasEnvToDeploy, _ := applicationconfig.IsThereAnythingToDeployForRadixApplication("main", ra)
	assert.True(t, hasEnvToDeploy)
}

func TestIsTargetEnvsEmpty_twoEntriesWithNoMapping(t *testing.T) {
	ra := utils.NewRadixApplicationBuilder().WithEnvironment("qa", "main").WithEnvironment("prod", "release").BuildRA()
	hasEnvToDeploy, _ := applicationconfig.IsThereAnythingToDeployForRadixApplication("test", ra)
	assert.False(t, hasEnvToDeploy)
}

func TestIsTargetEnvsEmpty_twoEntriesWithOneMapping(t *testing.T) {
	ra := utils.NewRadixApplicationBuilder().WithEnvironment("qa", "main").WithEnvironment("prod", "release").BuildRA()
	hasEnvToDeploy, _ := applicationconfig.IsThereAnythingToDeployForRadixApplication("main", ra)
	assert.True(t, hasEnvToDeploy)
}

func Test_WithBuildSecretsSet_SecretsCorrectlyAdded(t *testing.T) {
	tu, client, kubeUtil, radixClient := setupTest()

	appNamespace := "any-app-app"
	applyApplicationWithSync(tu, client, kubeUtil, radixClient,
		utils.ARadixApplication().
			WithAppName("any-app").
			WithEnvironment("dev", "master").
			WithBuildSecrets("secret1", "secret2"))

	secrets, _ := client.CoreV1().Secrets(appNamespace).List(context.TODO(), metav1.ListOptions{})
	defaultValue := []byte(defaults.BuildSecretDefaultData)

	buildSecrets := getSecretByName(defaults.BuildSecretsName, secrets)
	assert.NotNil(t, buildSecrets)
	assert.Equal(t, 2, len(buildSecrets.Data))
	assert.Equal(t, defaultValue, buildSecrets.Data["secret1"])
	assert.Equal(t, defaultValue, buildSecrets.Data["secret2"])

	roles, _ := client.RbacV1().Roles(appNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.True(t, roleByNameExists("radix-app-admin-build-secrets", roles))
	assert.True(t, roleByNameExists("pipeline-build-secrets", roles))

	rolebindings, _ := client.RbacV1().RoleBindings(appNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.True(t, roleBindingByNameExists("radix-app-admin-build-secrets", rolebindings))
	assert.True(t, roleBindingByNameExists("pipeline-build-secrets", rolebindings))

	applyApplicationWithSync(tu, client, kubeUtil, radixClient,
		utils.ARadixApplication().
			WithAppName("any-app").
			WithEnvironment("dev", "master").
			WithBuildSecrets("secret4", "secret5", "secret6"))

	secrets, _ = client.CoreV1().Secrets(appNamespace).List(context.TODO(), metav1.ListOptions{})
	buildSecrets = getSecretByName(defaults.BuildSecretsName, secrets)
	assert.Equal(t, 3, len(buildSecrets.Data))
	assert.Equal(t, defaultValue, buildSecrets.Data["secret4"])
	assert.Equal(t, defaultValue, buildSecrets.Data["secret5"])
	assert.Equal(t, defaultValue, buildSecrets.Data["secret6"])

}

func Test_WithBuildSecretsDeleted_SecretsCorrectlyDeleted(t *testing.T) {
	tu, client, kubeUtil, radixClient := setupTest()

	applyApplicationWithSync(tu, client, kubeUtil, radixClient,
		utils.ARadixApplication().
			WithAppName("any-app").
			WithEnvironment("dev", "master").
			WithBuildSecrets("secret1", "secret2"))

	// Delete secret
	appNamespace := "any-app-app"
	applyApplicationWithSync(tu, client, kubeUtil, radixClient,
		utils.ARadixApplication().
			WithAppName("any-app").
			WithEnvironment("dev", "master").
			WithBuildSecrets("secret2"))

	secrets, _ := client.CoreV1().Secrets(appNamespace).List(context.TODO(), metav1.ListOptions{})
	defaultValue := []byte(defaults.BuildSecretDefaultData)

	buildSecrets := getSecretByName(defaults.BuildSecretsName, secrets)
	assert.NotNil(t, buildSecrets)
	assert.Equal(t, 1, len(buildSecrets.Data))
	assert.Nil(t, buildSecrets.Data["secret1"])
	assert.Equal(t, defaultValue, buildSecrets.Data["secret2"])

	// Delete secret
	applyApplicationWithSync(tu, client, kubeUtil, radixClient,
		utils.ARadixApplication().
			WithAppName("any-app").
			WithEnvironment("dev", "master").
			WithBuildSecrets())

	// Secret is deleted
	secrets, _ = client.CoreV1().Secrets(appNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.False(t, secretByNameExists(defaults.BuildSecretsName, secrets))

	roles, _ := client.RbacV1().Roles(appNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.False(t, roleByNameExists("radix-app-admin-build-secrets", roles))
	assert.False(t, roleByNameExists("radix-app-reader-build-secrets", roles))
	assert.False(t, roleByNameExists("pipeline-build-secrets", roles))

	rolebindings, _ := client.RbacV1().RoleBindings(appNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.False(t, roleBindingByNameExists("radix-app-admin-build-secrets", rolebindings))
	assert.False(t, roleBindingByNameExists("radix-app-reader-build-secrets", rolebindings))
	assert.False(t, roleBindingByNameExists("pipeline-build-secrets", rolebindings))
}

func Test_AppReaderBuildSecretsRoleAndRoleBindingExists(t *testing.T) {
	tu, client, kubeUtil, radixClient := setupTest()

	applyApplicationWithSync(tu, client, kubeUtil, radixClient,
		utils.ARadixApplication().
			WithAppName("any-app").
			WithEnvironment("dev", "master").
			WithBuildSecrets("secret1", "secret2"))

	roles, _ := client.RbacV1().Roles("any-app-app").List(context.TODO(), metav1.ListOptions{})
	assert.True(t, roleByNameExists("radix-app-reader-build-secrets", roles))

	rolebindings, _ := client.RbacV1().RoleBindings("any-app-app").List(context.TODO(), metav1.ListOptions{})
	assert.True(t, roleBindingByNameExists("radix-app-reader-build-secrets", rolebindings))

	// Delete secret and verify that role and rolebinding is deleted
	applyApplicationWithSync(tu, client, kubeUtil, radixClient,
		utils.ARadixApplication().
			WithAppName("any-app").
			WithEnvironment("dev", "master"))

	roles, _ = client.RbacV1().Roles("any-app-app").List(context.TODO(), metav1.ListOptions{})
	assert.False(t, roleByNameExists("radix-app-reader-build-secrets", roles))

	rolebindings, _ = client.RbacV1().RoleBindings("any-app-app").List(context.TODO(), metav1.ListOptions{})
	assert.False(t, roleBindingByNameExists("radix-app-reader-build-secrets", rolebindings))
}

func Test_AppReaderPrivateImageHubRoleAndRoleBindingExists(t *testing.T) {
	tu, client, kubeUtil, radixClient := setupTest()

	applyApplicationWithSync(tu, client, kubeUtil, radixClient,
		utils.ARadixApplication().
			WithAppName("any-app").
			WithEnvironment("dev", "master").
			WithBuildSecrets("secret1", "secret2"))

	roles, _ := client.RbacV1().Roles("any-app-app").List(context.TODO(), metav1.ListOptions{})
	assert.True(t, roleByNameExists("radix-private-image-hubs-reader", roles))

	rolebindings, _ := client.RbacV1().RoleBindings("any-app-app").List(context.TODO(), metav1.ListOptions{})
	assert.True(t, roleBindingByNameExists("radix-private-image-hubs-reader", rolebindings))
}
func Test_WithPrivateImageHubSet_SecretsCorrectly_Added(t *testing.T) {
	client, _, _ := applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
		"privaterepodeleteme.azurecr.io": &radixv1.RadixPrivateImageHubCredential{
			Username: "814607e6-3d71-44a7-8476-50e8b281abbc",
			Email:    "radix@equinor.com",
		},
	})

	secret, _ := client.CoreV1().Secrets("any-app-app").Get(context.TODO(), defaults.PrivateImageHubSecretName, metav1.GetOptions{})
	assert.Equal(t,
		"{\"auths\":{\"privaterepodeleteme.azurecr.io\":{\"username\":\"814607e6-3d71-44a7-8476-50e8b281abbc\",\"password\":\"\",\"email\":\"radix@equinor.com\",\"auth\":\"ODE0NjA3ZTYtM2Q3MS00NGE3LTg0NzYtNTBlOGIyODFhYmJjOg==\"}}}",
		string(secret.Data[corev1.DockerConfigJsonKey]))
	assert.Equal(t, "radix-private-image-hubs-sync=any-app", secret.ObjectMeta.Annotations["kubed.appscode.com/sync"])
}

func Test_WithPrivateImageHubSet_SecretsCorrectly_SetPassword(t *testing.T) {
	client, appConfig, _ := applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
		"privaterepodeleteme.azurecr.io": &radixv1.RadixPrivateImageHubCredential{
			Username: "814607e6-3d71-44a7-8476-50e8b281abbc",
			Email:    "radix@equinor.com",
		},
	})
	pendingSecrets, _ := appConfig.GetPendingPrivateImageHubSecrets()

	assert.Equal(t, "privaterepodeleteme.azurecr.io", pendingSecrets[0])

	appConfig.UpdatePrivateImageHubsSecretsPassword("privaterepodeleteme.azurecr.io", "a-password")
	secret, _ := client.CoreV1().Secrets("any-app-app").Get(context.TODO(), defaults.PrivateImageHubSecretName, metav1.GetOptions{})
	pendingSecrets, _ = appConfig.GetPendingPrivateImageHubSecrets()

	assert.Equal(t,
		"{\"auths\":{\"privaterepodeleteme.azurecr.io\":{\"username\":\"814607e6-3d71-44a7-8476-50e8b281abbc\",\"password\":\"a-password\",\"email\":\"radix@equinor.com\",\"auth\":\"ODE0NjA3ZTYtM2Q3MS00NGE3LTg0NzYtNTBlOGIyODFhYmJjOmEtcGFzc3dvcmQ=\"}}}",
		string(secret.Data[corev1.DockerConfigJsonKey]))
	assert.Equal(t, 0, len(pendingSecrets))
}

func Test_WithPrivateImageHubSet_SecretsCorrectly_UpdatedNewAdded(t *testing.T) {
	applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
		"privaterepodeleteme.azurecr.io": &radixv1.RadixPrivateImageHubCredential{
			Username: "814607e6-3d71-44a7-8476-50e8b281abbc",
			Email:    "radix@equinor.com",
		},
	})

	client, _, _ := applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
		"privaterepodeleteme.azurecr.io": &radixv1.RadixPrivateImageHubCredential{
			Username: "814607e6-3d71-44a7-8476-50e8b281abbc",
			Email:    "radix@equinor.com",
		},
		"privaterepodeleteme2.azurecr.io": &radixv1.RadixPrivateImageHubCredential{
			Username: "814607e6-3d71-44a7-8476-50e8b281abbc",
			Email:    "radix@equinor.com",
		},
	})

	secret, _ := client.CoreV1().Secrets("any-app-app").Get(context.TODO(), defaults.PrivateImageHubSecretName, metav1.GetOptions{})

	assert.Equal(t,
		"{\"auths\":{\"privaterepodeleteme.azurecr.io\":{\"username\":\"814607e6-3d71-44a7-8476-50e8b281abbc\",\"password\":\"\",\"email\":\"radix@equinor.com\",\"auth\":\"ODE0NjA3ZTYtM2Q3MS00NGE3LTg0NzYtNTBlOGIyODFhYmJjOg==\"},\"privaterepodeleteme2.azurecr.io\":{\"username\":\"814607e6-3d71-44a7-8476-50e8b281abbc\",\"password\":\"\",\"email\":\"radix@equinor.com\",\"auth\":\"ODE0NjA3ZTYtM2Q3MS00NGE3LTg0NzYtNTBlOGIyODFhYmJjOg==\"}}}",
		string(secret.Data[corev1.DockerConfigJsonKey]))
}

func Test_WithPrivateImageHubSet_SecretsCorrectly_UpdateUsername(t *testing.T) {
	applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
		"privaterepodeleteme.azurecr.io": &radixv1.RadixPrivateImageHubCredential{
			Username: "814607e6-3d71-44a7-8476-50e8b281abbc",
			Email:    "radix@equinor.com",
		},
	})

	client, _, _ := applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
		"privaterepodeleteme.azurecr.io": &radixv1.RadixPrivateImageHubCredential{
			Username: "814607e6-3d71-44a7-8476-50e8b281abb2",
			Email:    "radix@equinor.com",
		},
	})

	secret, _ := client.CoreV1().Secrets("any-app-app").Get(context.TODO(), defaults.PrivateImageHubSecretName, metav1.GetOptions{})

	assert.Equal(t,
		"{\"auths\":{\"privaterepodeleteme.azurecr.io\":{\"username\":\"814607e6-3d71-44a7-8476-50e8b281abb2\",\"password\":\"\",\"email\":\"radix@equinor.com\",\"auth\":\"ODE0NjA3ZTYtM2Q3MS00NGE3LTg0NzYtNTBlOGIyODFhYmIyOg==\"}}}",
		string(secret.Data[corev1.DockerConfigJsonKey]))
}

func Test_WithPrivateImageHubSet_SecretsCorrectly_UpdateServerName(t *testing.T) {
	applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
		"privaterepodeleteme.azurecr.io": &radixv1.RadixPrivateImageHubCredential{
			Username: "814607e6-3d71-44a7-8476-50e8b281abbc",
			Email:    "radix@equinor.com",
		},
	})

	client, _, _ := applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
		"privaterepodeleteme1.azurecr.io": &radixv1.RadixPrivateImageHubCredential{
			Username: "814607e6-3d71-44a7-8476-50e8b281abbc",
			Email:    "radix@equinor.com",
		},
	})
	secret, _ := client.CoreV1().Secrets("any-app-app").Get(context.TODO(), defaults.PrivateImageHubSecretName, metav1.GetOptions{})

	assert.Equal(t,
		"{\"auths\":{\"privaterepodeleteme1.azurecr.io\":{\"username\":\"814607e6-3d71-44a7-8476-50e8b281abbc\",\"password\":\"\",\"email\":\"radix@equinor.com\",\"auth\":\"ODE0NjA3ZTYtM2Q3MS00NGE3LTg0NzYtNTBlOGIyODFhYmJjOg==\"}}}",
		string(secret.Data[corev1.DockerConfigJsonKey]))
}

func Test_WithPrivateImageHubSet_SecretsCorrectly_Delete(t *testing.T) {
	client, _, _ := applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
		"privaterepodeleteme.azurecr.io": &radixv1.RadixPrivateImageHubCredential{
			Username: "814607e6-3d71-44a7-8476-50e8b281abbc",
			Email:    "radix@equinor.com",
		},
		"privaterepodeleteme2.azurecr.io": &radixv1.RadixPrivateImageHubCredential{
			Username: "814607e6-3d71-44a7-8476-50e8b281abbc",
			Email:    "radix@equinor.com",
		},
	})

	secret, _ := client.CoreV1().Secrets("any-app-app").Get(context.TODO(), defaults.PrivateImageHubSecretName, metav1.GetOptions{})
	assert.Equal(t,
		"{\"auths\":{\"privaterepodeleteme.azurecr.io\":{\"username\":\"814607e6-3d71-44a7-8476-50e8b281abbc\",\"password\":\"\",\"email\":\"radix@equinor.com\",\"auth\":\"ODE0NjA3ZTYtM2Q3MS00NGE3LTg0NzYtNTBlOGIyODFhYmJjOg==\"},\"privaterepodeleteme2.azurecr.io\":{\"username\":\"814607e6-3d71-44a7-8476-50e8b281abbc\",\"password\":\"\",\"email\":\"radix@equinor.com\",\"auth\":\"ODE0NjA3ZTYtM2Q3MS00NGE3LTg0NzYtNTBlOGIyODFhYmJjOg==\"}}}",
		string(secret.Data[corev1.DockerConfigJsonKey]))

	client, _, _ = applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
		"privaterepodeleteme2.azurecr.io": &radixv1.RadixPrivateImageHubCredential{
			Username: "814607e6-3d71-44a7-8476-50e8b281abbc",
			Email:    "radix@equinor.com",
		},
	})

	secret, _ = client.CoreV1().Secrets("any-app-app").Get(context.TODO(), defaults.PrivateImageHubSecretName, metav1.GetOptions{})
	assert.Equal(t,
		"{\"auths\":{\"privaterepodeleteme2.azurecr.io\":{\"username\":\"814607e6-3d71-44a7-8476-50e8b281abbc\",\"password\":\"\",\"email\":\"radix@equinor.com\",\"auth\":\"ODE0NjA3ZTYtM2Q3MS00NGE3LTg0NzYtNTBlOGIyODFhYmJjOg==\"}}}",
		string(secret.Data[corev1.DockerConfigJsonKey]))
}

func Test_WithPrivateImageHubSet_SecretsCorrectly_NoImageHubs(t *testing.T) {
	client, appConfig, _ := applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{})
	pendingSecrets, _ := appConfig.GetPendingPrivateImageHubSecrets()

	secret, _ := client.CoreV1().Secrets("any-app-app").Get(context.TODO(), defaults.PrivateImageHubSecretName, metav1.GetOptions{})

	assert.NotNil(t, secret)
	assert.Equal(t,
		"{\"auths\":{}}",
		string(secret.Data[corev1.DockerConfigJsonKey]))
	assert.Equal(t, 0, len(pendingSecrets))
	assert.Error(t, appConfig.UpdatePrivateImageHubsSecretsPassword("privaterepodeleteme.azurecr.io", "a-password"))
}

func Test_RadixEnvironment(t *testing.T) {
	tu, client, kubeUtil, radixClient := setupTest()

	applyApplicationWithSync(tu, client, kubeUtil, radixClient,
		utils.ARadixApplication().
			WithAppName("any-app"))

	rr, _ := radixClient.RadixV1().RadixRegistrations().Get(context.TODO(), "any-app", metav1.GetOptions{})

	environments, err := radixClient.RadixV1().RadixEnvironments().List(context.TODO(), metav1.ListOptions{})

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
			useBuildKit:         utils.BoolPtr(false),
			expectedUseBuildKit: utils.BoolPtr(false),
		},
		{
			appName:             "any-app3",
			useBuildKit:         utils.BoolPtr(true),
			expectedUseBuildKit: utils.BoolPtr(true),
		},
	}
	tu, client, kubeUtil, radixClient := setupTest()

	for _, testScenario := range testScenarios {
		ra := utils.ARadixApplication().WithAppName(testScenario.appName)
		if testScenario.useBuildKit != nil {
			ra = ra.WithBuildKit(testScenario.useBuildKit)
		}
		applyApplicationWithSync(tu, client, kubeUtil, radixClient, ra)

		raAfterSync, _ := radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(testScenario.appName)).Get(context.TODO(), testScenario.appName, metav1.GetOptions{})

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

func Test_DNSAliases_CreateUpdateDelete(t *testing.T) {
	const (
		appName1   = "any-app1"
		appName2   = "any-app2"
		env1       = "env1"
		env2       = "env2"
		component1 = "server1"
		component2 = "server2"
		branch1    = "branch1"
		branch2    = "branch2"
		portA      = "port-a"
		port8080   = 8080
		port9090   = 9090
		domain1    = "domain1"
		domain2    = "domain2"
		domain3    = "domain3"
		domain4    = "domain4"
	)
	var testScenarios = []struct {
		name                    string
		applicationBuilder      utils.ApplicationBuilder
		existingRadixDNSAliases map[string]radixv1.RadixDNSAliasSpec
		expectedRadixDNSAliases map[string]radixv1.RadixDNSAliasSpec
	}{
		{
			name: "no aliases",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithComponent(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{},
		},
		{
			name: "one alias",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithDNSAlias(radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component1}).
				WithComponent(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
			},
		},
		{
			name: "aliases for multiple components",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithDNSAlias(
					radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component1},
					radixv1.DNSAlias{Domain: domain2, Environment: env1, Component: component2},
				).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
				domain2: {AppName: appName1, Environment: env1, Component: component2, Port: port9090},
			},
		},
		{
			name: "aliases for multiple environments",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(
					radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component1},
					radixv1.DNSAlias{Domain: domain2, Environment: env2, Component: component1},
				).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
				domain2: {AppName: appName1, Environment: env2, Component: component1, Port: port8080},
			},
		},
		{
			name: "multiple aliases for one component and environment",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithDNSAlias(
					radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component1},
					radixv1.DNSAlias{Domain: domain2, Environment: env1, Component: component1},
				).
				WithComponents(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
				domain2: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
			},
		},
		{
			name: "aliases for multiple components in different environments",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(
					radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component1},
					radixv1.DNSAlias{Domain: domain2, Environment: env2, Component: component2},
				).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
				domain2: {AppName: appName1, Environment: env2, Component: component2, Port: port9090},
			},
		},
		{
			name: "one alias, exist other RDA, two expected RDA",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithDNSAlias(radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component1}).
				WithComponent(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			existingRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain2: {AppName: appName2, Environment: env1, Component: component1, Port: port9090},
			},
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain2: {AppName: appName2, Environment: env1, Component: component1, Port: port9090},
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
			},
		},
		{
			name: "change env and component for existing RDA",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(radixv1.DNSAlias{Domain: domain1, Environment: env2, Component: component2}).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			existingRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
			},
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env2, Component: component2, Port: port9090},
			},
		},
		{
			name: "swap env and component for existing RDA-s",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(
					radixv1.DNSAlias{Domain: domain1, Environment: env2, Component: component2},
					radixv1.DNSAlias{Domain: domain2, Environment: env1, Component: component1},
				).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			existingRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
				domain2: {AppName: appName1, Environment: env2, Component: component2, Port: port9090},
			},
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env2, Component: component2, Port: port9090},
				domain2: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
			},
		},
		{
			name: "remove single alias",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithComponents(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			existingRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
			},
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{},
		},
		{
			name: "remove multiple aliases, keep other",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(
					radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component1},
					radixv1.DNSAlias{Domain: domain3, Environment: env2, Component: component1},
				).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			existingRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
				domain2: {AppName: appName1, Environment: env1, Component: component2, Port: port9090},
				domain3: {AppName: appName1, Environment: env2, Component: component1, Port: port8080},
				domain4: {AppName: appName2, Environment: env1, Component: component1, Port: port9090},
			},
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
				domain3: {AppName: appName1, Environment: env2, Component: component1, Port: port8080},
				domain4: {AppName: appName2, Environment: env1, Component: component1, Port: port9090},
			},
		},
		{
			name: "create new, remove some, change other aliases",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(
					radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component2},
					radixv1.DNSAlias{Domain: domain3, Environment: env2, Component: component1},
				).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			existingRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
				domain2: {AppName: appName1, Environment: env1, Component: component2, Port: port9090},
				domain4: {AppName: appName2, Environment: env1, Component: component1, Port: port9090},
			},
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component2, Port: port9090},
				domain3: {AppName: appName1, Environment: env2, Component: component1, Port: port8080},
				domain4: {AppName: appName2, Environment: env1, Component: component1, Port: port9090},
			},
		},
	}

	for _, ts := range testScenarios {
		t.Run(ts.name, func(t *testing.T) {
			tu, kubeClient, kubeUtil, radixClient := setupTest()

			require.NoError(t, registerExistingRadixDNSAliases(radixClient, ts.existingRadixDNSAliases), "create existing RadixDNSAlias")
			require.NoError(t, applyApplicationWithSync(tu, kubeClient, kubeUtil, radixClient, ts.applicationBuilder), "register radix application")

			radixDNSAliases, err := radixClient.RadixV1().RadixDNSAliases().List(context.TODO(), metav1.ListOptions{})
			require.NoError(t, err)

			if ts.expectedRadixDNSAliases == nil {
				require.Len(t, radixDNSAliases.Items, 0, "not expected Radix DNS aliases")
				return
			}

			require.Len(t, radixDNSAliases.Items, len(ts.expectedRadixDNSAliases), "not matching expected Radix DNS aliases count")
			if len(radixDNSAliases.Items) == len(ts.expectedRadixDNSAliases) {
				for _, radixDNSAlias := range radixDNSAliases.Items {
					if expectedDNSAlias, ok := ts.expectedRadixDNSAliases[radixDNSAlias.Name]; ok {
						assert.Equal(t, expectedDNSAlias.AppName, radixDNSAlias.Spec.AppName, "app name")
						assert.Equal(t, expectedDNSAlias.Environment, radixDNSAlias.Spec.Environment, "environment")
						assert.Equal(t, expectedDNSAlias.Component, radixDNSAlias.Spec.Component, "component")
						assert.Equal(t, expectedDNSAlias.Port, radixDNSAlias.Spec.Port, "port")
						if _, itWasExistingAlias := ts.existingRadixDNSAliases[radixDNSAlias.Name]; !itWasExistingAlias {
							ownerReferences := radixDNSAlias.GetOwnerReferences()
							require.Len(t, ownerReferences, 1)
							ownerReference := ownerReferences[0]
							assert.Equal(t, radixDNSAlias.Spec.AppName, ownerReference.Name, "invalid or empty ownerReference.Name")
							assert.Equal(t, radix.KindRadixApplication, ownerReference.Kind, "invalid or empty ownerReference.Kind")
							assert.NotEmpty(t, ownerReference.UID, "ownerReference.UID is empty")
							require.NotNil(t, ownerReference.Controller, "ownerReference.Controller is nil")
							assert.True(t, *ownerReference.Controller, "ownerReference.Controller is false")
						}
						continue
					}
					assert.Fail(t, fmt.Sprintf("found not expected RadixDNSAlias %s: env %s, component %s, port %d, appName %s",
						radixDNSAlias.GetName(), radixDNSAlias.Spec.Environment, radixDNSAlias.Spec.Component, radixDNSAlias.Spec.Port, radixDNSAlias.Spec.AppName))
				}
			}
		})
	}
}

func Test_DNSAliases_FileOndInvalidAppName(t *testing.T) {
	const (
		appName1   = "any-app1"
		appName2   = "any-app2"
		env1       = "env1"
		component1 = "server1"
		domain1    = "domain1"
		branch1    = "branch1"
		portA      = "port-a"
		port8080   = 8080
	)
	tu, kubeClient, kubeUtil, radixClient := setupTest()

	_, err := radixClient.RadixV1().RadixDNSAliases().Create(context.Background(),
		&radixv1.RadixDNSAlias{
			ObjectMeta: metav1.ObjectMeta{Name: domain1, Labels: map[string]string{kube.RadixAppLabel: appName1}},
			Spec:       radixv1.RadixDNSAliasSpec{AppName: appName2, Environment: env1, Component: component1},
		}, metav1.CreateOptions{})
	require.NoError(t, err, "create existing RadixDNSAlias")

	applicationBuilder := utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
		WithDNSAlias(radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component1}).
		WithComponents(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA))
	err = applyApplicationWithSync(tu, kubeClient, kubeUtil, radixClient, applicationBuilder)
	require.Error(t, err, "register radix application")
}

func registerExistingRadixDNSAliases(radixClient radixclient.Interface, radixDNSAliasesMap map[string]radixv1.RadixDNSAliasSpec) error {
	for domain, rdaSpec := range radixDNSAliasesMap {
		_, err := radixClient.RadixV1().RadixDNSAliases().Create(context.Background(),
			&radixv1.RadixDNSAlias{
				ObjectMeta: metav1.ObjectMeta{
					Name:   domain,
					Labels: map[string]string{kube.RadixAppLabel: rdaSpec.AppName},
				},
				Spec: rdaSpec,
			}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func rrAsOwnerReference(rr *radixv1.RadixRegistration) []metav1.OwnerReference {
	trueVar := true
	return []metav1.OwnerReference{
		{
			APIVersion: radix.APIVersion,
			Kind:       radix.KindRadixRegistration,
			Name:       rr.Name,
			UID:        rr.UID,
			Controller: &trueVar,
		},
	}
}

func applyRadixAppWithPrivateImageHub(privateImageHubs radixv1.PrivateImageHubEntries) (kubernetes.Interface, *applicationconfig.ApplicationConfig, error) {
	tu, client, kubeUtil, radixClient := setupTest()
	appBuilder := utils.ARadixApplication().
		WithAppName("any-app").
		WithEnvironment("dev", "master")
	for key, config := range privateImageHubs {
		appBuilder.WithPrivateImageRegistry(key, config.Username, config.Email)
	}

	err := applyApplicationWithSync(tu, client, kubeUtil, radixClient, appBuilder)
	if err != nil {
		return nil, nil, err
	}
	appConfig, err := getAppConfig(client, kubeUtil, radixClient, appBuilder)
	return client, appConfig, err
}

func getAppConfig(client kubernetes.Interface, kubeUtil *kube.Kube, radixClient radixclient.Interface, applicationBuilder utils.ApplicationBuilder) (*applicationconfig.ApplicationConfig, error) {
	ra := applicationBuilder.BuildRA()
	radixRegistration, _ := radixClient.RadixV1().RadixRegistrations().Get(context.TODO(), ra.Name, metav1.GetOptions{})

	return applicationconfig.NewApplicationConfig(client, kubeUtil, radixClient, radixRegistration, ra)
}

func applyApplicationWithSync(tu *test.Utils, client kubernetes.Interface, kubeUtil *kube.Kube,
	radixClient radixclient.Interface, applicationBuilder utils.ApplicationBuilder) error {

	ra, err := tu.ApplyApplication(applicationBuilder)
	if err != nil {
		return err
	}

	radixRegistration, err := radixClient.RadixV1().RadixRegistrations().Get(context.TODO(), ra.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	applicationConfig, err := applicationconfig.NewApplicationConfig(client, kubeUtil, radixClient, radixRegistration, ra)
	if err != nil {
		return err
	}

	err = applicationConfig.OnSync()
	if err != nil {
		return err
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
