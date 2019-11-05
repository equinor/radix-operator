package applicationconfig

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	sampleRegistration = "./testdata/sampleregistration.yaml"
	sampleApp          = "./testdata/radixconfig.yaml"
	clusterName        = "AnyClusterName"
	containerRegistry  = "any.container.registry"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

func setupTest() (*test.Utils, kubernetes.Interface, *kube.Kube, radixclient.Interface) {
	kubeclient := fake.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()
	kubeUtil, _ := kube.New(kubeclient, radixclient)

	handlerTestUtils := test.NewTestUtils(kubeclient, radixclient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	return &handlerTestUtils, kubeclient, kubeUtil, radixclient
}

func getApplication(ra *radixv1.RadixApplication) *ApplicationConfig {
	// The other arguments are not relevant for this test
	application, _ := NewApplicationConfig(nil, nil, nil, nil, ra)
	return application
}

func Test_Create_Radix_Environments(t *testing.T) {
	_, client, kubeUtil, radixclient := setupTest()

	radixRegistration, _ := utils.GetRadixRegistrationFromFile(sampleRegistration)
	radixApp, _ := utils.GetRadixApplication(sampleApp)
	app, _ := NewApplicationConfig(client, kubeUtil, radixclient, radixRegistration, radixApp)

	label := fmt.Sprintf("%s=%s", kube.RadixAppLabel, radixRegistration.Name)
	t.Run("It can create environments", func(t *testing.T) {
		err := app.createEnvironments()
		assert.NoError(t, err)
		namespaces, _ := client.CoreV1().Namespaces().List(metav1.ListOptions{
			LabelSelector: label,
		})
		assert.Len(t, namespaces.Items, 2)
	})

	t.Run("It doesn't fail when re-running creation", func(t *testing.T) {
		err := app.createEnvironments()
		assert.NoError(t, err)
		namespaces, _ := client.CoreV1().Namespaces().List(metav1.ListOptions{
			LabelSelector: label,
		})
		assert.Len(t, namespaces.Items, 2)
	})
}

func Test_Reconciles_Radix_Environments(t *testing.T) {
	// Setup
	_, client, kubeUtil, radixclient := setupTest()

	// Create namespaces manually
	client.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "any-app-qa",
		},
	})

	client.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "any-app-prod",
		},
	})

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

	app, _ := NewApplicationConfig(client, kubeUtil, radixclient, rr, ra)
	label := fmt.Sprintf("%s=%s", kube.RadixAppLabel, rr.Name)

	// Test
	app.createEnvironments()
	namespaces, _ := client.CoreV1().Namespaces().List(metav1.ListOptions{
		LabelSelector: label,
	})
	assert.Equal(t, 2, len(namespaces.Items))
}

func TestIsBranchMappedToEnvironment_multipleEnvsToOneBranch_ListsBoth(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "master").
		WithEnvironment("prod", "master").
		BuildRA()

	application := getApplication(ra)
	branchMapped, targetEnvs := application.IsBranchMappedToEnvironment(branch)

	assert.True(t, branchMapped)
	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, targetEnvs["prod"], true)
	assert.Equal(t, targetEnvs["qa"], true)
}

func TestIsBranchMappedToEnvironment_multipleEnvsToOneBranchOtherBranchIsChanged_ListsBothButNoneIsBuilding(t *testing.T) {
	branch := "development"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "master").
		WithEnvironment("prod", "master").
		BuildRA()

	application := getApplication(ra)
	branchMapped, targetEnvs := application.IsBranchMappedToEnvironment(branch)

	assert.False(t, branchMapped)
	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, targetEnvs["prod"], false)
	assert.Equal(t, targetEnvs["qa"], false)
}

func TestIsBranchMappedToEnvironment_oneEnvToOneBranch_ListsBothButOnlyOneShouldBeBuilt(t *testing.T) {
	branch := "development"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "development").
		WithEnvironment("prod", "master").
		BuildRA()

	application := getApplication(ra)
	branchMapped, targetEnvs := application.IsBranchMappedToEnvironment(branch)

	assert.True(t, branchMapped)
	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, targetEnvs["prod"], false)
	assert.Equal(t, targetEnvs["qa"], true)
}

func TestIsBranchMappedToEnvironment_twoEnvNoBranch(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironmentNoBranch("qa").
		WithEnvironmentNoBranch("prod").
		BuildRA()

	application := getApplication(ra)
	branchMapped, targetEnvs := application.IsBranchMappedToEnvironment(branch)

	assert.False(t, branchMapped)
	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, false, targetEnvs["qa"])
	assert.Equal(t, false, targetEnvs["prod"])
}

func TestIsBranchMappedToEnvironment_NoEnv(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		BuildRA()

	application := getApplication(ra)
	branchMapped, targetEnvs := application.IsBranchMappedToEnvironment(branch)

	assert.False(t, branchMapped)
	assert.Equal(t, 0, len(targetEnvs))
}

func TestIsBranchMappedToEnvironment_promotionScheme_ListsBothButOnlyOneShouldBeBuilt(t *testing.T) {
	branch := "master"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("qa", "master").
		WithEnvironment("prod", "").
		BuildRA()

	application := getApplication(ra)
	branchMapped, targetEnvs := application.IsBranchMappedToEnvironment(branch)

	assert.True(t, branchMapped)
	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, targetEnvs["prod"], false)
	assert.Equal(t, targetEnvs["qa"], true)
}

func TestIsBranchMappedToEnvironment_wildcardMatch_ListsBothButOnlyOneShouldBeBuilt(t *testing.T) {
	branch := "feature/RA-123-Test"

	ra := utils.NewRadixApplicationBuilder().
		WithEnvironment("feature", "feature/*").
		WithEnvironment("prod", "master").
		BuildRA()

	application := getApplication(ra)
	branchMapped, targetEnvs := application.IsBranchMappedToEnvironment(branch)

	assert.True(t, branchMapped)
	assert.Equal(t, 2, len(targetEnvs))
	assert.Equal(t, targetEnvs["prod"], false)
	assert.Equal(t, targetEnvs["feature"], true)
}

func TestIsTargetEnvsEmpty_noEntry(t *testing.T) {
	targetEnvs := map[string]bool{}
	assert.Equal(t, true, isTargetEnvsEmpty(targetEnvs))
}

func TestIsTargetEnvsEmpty_twoEntriesWithBranchMapping(t *testing.T) {
	targetEnvs := map[string]bool{
		"qa":   true,
		"prod": true,
	}
	assert.Equal(t, false, isTargetEnvsEmpty(targetEnvs))
}

func TestIsTargetEnvsEmpty_twoEntriesWithNoMapping(t *testing.T) {
	targetEnvs := map[string]bool{
		"qa":   false,
		"prod": false,
	}
	assert.Equal(t, true, isTargetEnvsEmpty(targetEnvs))
}

func TestIsTargetEnvsEmpty_twoEntriesWithOneMapping(t *testing.T) {
	targetEnvs := map[string]bool{
		"qa":   true,
		"prod": false,
	}
	assert.Equal(t, false, isTargetEnvsEmpty(targetEnvs))
}

func TestObjectSynced_WithEnvironmentsNoLimitsSet_NamespacesAreCreatedWithNoLimits(t *testing.T) {
	tu, client, kubeUtil, radixclient := setupTest()

	applyApplicationWithSync(tu, client, kubeUtil, radixclient, utils.ARadixApplication().
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
	tu, client, kubeUtil, radixclient := setupTest()

	// Setup
	os.Setenv(defaults.OperatorEnvLimitDefaultCPUEnvironmentVariable, "0.5")
	os.Setenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable, "300M")
	os.Setenv(defaults.OperatorEnvLimitDefaultReqestCPUEnvironmentVariable, "0.25")
	os.Setenv(defaults.OperatorEnvLimitDefaultRequestMemoryEnvironmentVariable, "256M")

	applyApplicationWithSync(tu, client, kubeUtil, radixclient, utils.ARadixApplication().
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

func Test_WithPrivateImageHubSet_SecretsCorrectly_Added(t *testing.T) {
	client, _, _ := applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
		"privaterepodeleteme.azurecr.io": &radixv1.RadixPrivateImageHubCredential{
			Username: "814607e6-3d71-44a7-8476-50e8b281abbc",
			Email:    "radix@equinor.com",
		},
	})

	secret, _ := client.CoreV1().Secrets("any-app-app").Get(PrivateImageHubSecretName, metav1.GetOptions{})
	assert.Equal(t,
		"{\"auths\":{\"privaterepodeleteme.azurecr.io\":{\"username\":\"814607e6-3d71-44a7-8476-50e8b281abbc\",\"password\":\"\",\"email\":\"radix@equinor.com\",\"auth\":\"ODE0NjA3ZTYtM2Q3MS00NGE3LTg0NzYtNTBlOGIyODFhYmJjOg==\"}}}",
		string(secret.Data[corev1.DockerConfigJsonKey]))
	assert.Equal(t, "radix-private-image-hubs-sync=any-app-app", secret.ObjectMeta.Annotations["kubed.appscode.com/sync"])
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
	secret, _ := client.CoreV1().Secrets("any-app-app").Get(PrivateImageHubSecretName, metav1.GetOptions{})
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

	secret, _ := client.CoreV1().Secrets("any-app-app").Get(PrivateImageHubSecretName, metav1.GetOptions{})

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

	secret, _ := client.CoreV1().Secrets("any-app-app").Get(PrivateImageHubSecretName, metav1.GetOptions{})

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
	secret, _ := client.CoreV1().Secrets("any-app-app").Get(PrivateImageHubSecretName, metav1.GetOptions{})

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

	secret, _ := client.CoreV1().Secrets("any-app-app").Get(PrivateImageHubSecretName, metav1.GetOptions{})
	assert.Equal(t,
		"{\"auths\":{\"privaterepodeleteme.azurecr.io\":{\"username\":\"814607e6-3d71-44a7-8476-50e8b281abbc\",\"password\":\"\",\"email\":\"radix@equinor.com\",\"auth\":\"ODE0NjA3ZTYtM2Q3MS00NGE3LTg0NzYtNTBlOGIyODFhYmJjOg==\"},\"privaterepodeleteme2.azurecr.io\":{\"username\":\"814607e6-3d71-44a7-8476-50e8b281abbc\",\"password\":\"\",\"email\":\"radix@equinor.com\",\"auth\":\"ODE0NjA3ZTYtM2Q3MS00NGE3LTg0NzYtNTBlOGIyODFhYmJjOg==\"}}}",
		string(secret.Data[corev1.DockerConfigJsonKey]))

	// TODO - split -  delete image registry
	client, _, _ = applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
		"privaterepodeleteme2.azurecr.io": &radixv1.RadixPrivateImageHubCredential{
			Username: "814607e6-3d71-44a7-8476-50e8b281abbc",
			Email:    "radix@equinor.com",
		},
	})

	secret, _ = client.CoreV1().Secrets("any-app-app").Get(PrivateImageHubSecretName, metav1.GetOptions{})
	assert.Equal(t,
		"{\"auths\":{\"privaterepodeleteme2.azurecr.io\":{\"username\":\"814607e6-3d71-44a7-8476-50e8b281abbc\",\"password\":\"\",\"email\":\"radix@equinor.com\",\"auth\":\"ODE0NjA3ZTYtM2Q3MS00NGE3LTg0NzYtNTBlOGIyODFhYmJjOg==\"}}}",
		string(secret.Data[corev1.DockerConfigJsonKey]))
}

func Test_WithPrivateImageHubSet_SecretsCorrectly_NoImageHubs(t *testing.T) {
	client, _, _ := applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{})
	secret, _ := client.CoreV1().Secrets("any-app-app").Get(PrivateImageHubSecretName, metav1.GetOptions{})

	assert.NotNil(t, secret)
	assert.Nil(t, secret.Data[corev1.DockerConfigJsonKey])
}

func applyRadixAppWithPrivateImageHub(privateImageHubs radixv1.PrivateImageHubEntries) (kubernetes.Interface, *ApplicationConfig, error) {
	tu, client, kubeUtil, radixclient := setupTest()
	appBuilder := utils.ARadixApplication().
		WithAppName("any-app").
		WithEnvironment("dev", "master")
	for key, config := range privateImageHubs {
		appBuilder.WithPrivateImageRegistry(key, config.Username, config.Email)
	}

	applyApplicationWithSync(tu, client, kubeUtil, radixclient, appBuilder)
	appConfig, err := getAppConfig(client, kubeUtil, radixclient, appBuilder)
	return client, appConfig, err
}

func getAppConfig(client kubernetes.Interface, kubeUtil *kube.Kube, radixclient radixclient.Interface, applicationBuilder utils.ApplicationBuilder) (*ApplicationConfig, error) {
	ra := applicationBuilder.BuildRA()
	radixRegistration, _ := radixclient.RadixV1().RadixRegistrations().Get(ra.Name, metav1.GetOptions{})

	return NewApplicationConfig(client, kubeUtil, radixclient, radixRegistration, ra)
}

func applyApplicationWithSync(tu *test.Utils, client kubernetes.Interface, kubeUtil *kube.Kube,
	radixclient radixclient.Interface, applicationBuilder utils.ApplicationBuilder) error {

	err := tu.ApplyApplication(applicationBuilder)
	if err != nil {
		return err
	}

	ra := applicationBuilder.BuildRA()
	radixRegistration, err := radixclient.RadixV1().RadixRegistrations().Get(ra.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	applicationconfig, err := NewApplicationConfig(client, kubeUtil, radixclient, radixRegistration, ra)

	err = applicationconfig.OnSync()
	if err != nil {
		return err
	}

	return nil
}
