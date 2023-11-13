package applicationconfig_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
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
	application := applicationconfig.NewApplicationConfig(nil, nil, nil, nil, ra, nil)
	return application
}

func Test_Create_Radix_Environments(t *testing.T) {
	_, client, kubeUtil, radixClient := setupTest()

	radixRegistration, _ := utils.GetRadixRegistrationFromFile(sampleRegistration)
	radixApp, _ := utils.GetRadixApplicationFromFile(sampleApp)
	app := applicationconfig.NewApplicationConfig(client, kubeUtil, radixClient, radixRegistration, radixApp, nil)

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

	app := applicationconfig.NewApplicationConfig(client, kubeUtil, radixClient, rr, ra, getDNSAliasConfig())
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
	targetEnvs := application.GetTargetEnvironments(branch)
	assert.ElementsMatch(t, []string{"prod", "qa"}, targetEnvs)
}

func TestIsThereAnythingToDeploy_multipleEnvsToOneBranchOtherBranchIsChanged_ListsBothButNoneIsBuilding(t *testing.T) {
	branch := "development"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "master").
		WithEnvironment("prod", "master").
		BuildRA()

	application := getApplication(ra)
	targetEnvs := application.GetTargetEnvironments(branch)
	assert.Equal(t, 0, len(targetEnvs))
}

func TestIsThereAnythingToDeploy_oneEnvToOneBranch_ListsBothButOnlyOneShouldBeBuilt(t *testing.T) {
	branch := "development"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "development").
		WithEnvironment("prod", "master").
		BuildRA()

	application := getApplication(ra)
	targetEnvs := application.GetTargetEnvironments(branch)
	assert.ElementsMatch(t, []string{"qa"}, targetEnvs)
}

func TestIsThereAnythingToDeploy_twoEnvNoBranch(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironmentNoBranch("qa").
		WithEnvironmentNoBranch("prod").
		BuildRA()

	application := getApplication(ra)
	targetEnvs := application.GetTargetEnvironments(branch)
	assert.Equal(t, 0, len(targetEnvs))
}

func TestIsThereAnythingToDeploy_NoEnv(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		BuildRA()

	application := getApplication(ra)
	targetEnvs := application.GetTargetEnvironments(branch)
	assert.Equal(t, 0, len(targetEnvs))
}

func TestIsThereAnythingToDeploy_promotionScheme_ListsBothButOnlyOneShouldBeBuilt(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "master").
		WithEnvironment("prod", "").
		BuildRA()

	application := getApplication(ra)
	targetEnvs := application.GetTargetEnvironments(branch)
	assert.ElementsMatch(t, []string{"qa"}, targetEnvs)
}

func TestIsThereAnythingToDeploy_wildcardMatch_ListsBothButOnlyOneShouldBeBuilt(t *testing.T) {
	branch := "feature/RA-123-Test"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("feature", "feature/*").
		WithEnvironment("prod", "master").
		BuildRA()

	application := getApplication(ra)
	targetEnvs := application.GetTargetEnvironments(branch)
	assert.ElementsMatch(t, []string{"feature"}, targetEnvs)
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
	client, _ := applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
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
	client, appConfig := applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
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

	client, _ := applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
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

	client, _ := applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
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

	client, _ := applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
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
	client, _ := applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
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

	client, _ = applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
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
	client, appConfig := applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{})
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

func applyRadixAppWithPrivateImageHub(privateImageHubs radixv1.PrivateImageHubEntries) (kubernetes.Interface, *applicationconfig.ApplicationConfig) {
	tu, client, kubeUtil, radixClient := setupTest()
	appBuilder := utils.ARadixApplication().
		WithAppName("any-app").
		WithEnvironment("dev", "master")
	for key, config := range privateImageHubs {
		appBuilder.WithPrivateImageRegistry(key, config.Username, config.Email)
	}

	err := applyApplicationWithSync(tu, client, kubeUtil, radixClient, appBuilder)
	if err != nil {
		return nil, nil
	}
	appConfig := getAppConfig(client, kubeUtil, radixClient, appBuilder)
	return client, appConfig
}

func getAppConfig(client kubernetes.Interface, kubeUtil *kube.Kube, radixClient radixclient.Interface, applicationBuilder utils.ApplicationBuilder) *applicationconfig.ApplicationConfig {
	ra := applicationBuilder.BuildRA()
	radixRegistration, _ := radixClient.RadixV1().RadixRegistrations().Get(context.TODO(), ra.Name, metav1.GetOptions{})

	return applicationconfig.NewApplicationConfig(client, kubeUtil, radixClient, radixRegistration, ra, getDNSAliasConfig())
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

	applicationConfig := applicationconfig.NewApplicationConfig(client, kubeUtil, radixClient, radixRegistration, ra, getDNSAliasConfig())

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

func getDNSAliasConfig() *dnsalias.DNSConfig {
	return &dnsalias.DNSConfig{
		DNSZone:               "dev.radix.equinor.com",
		ReservedAppDNSAliases: dnsalias.AppReservedDNSAlias{"api": "radix-api"},
		ReservedDNSAliases:    []string{"grafana"},
	}
}
