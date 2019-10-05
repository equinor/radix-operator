package applicationconfig

import (
	"encoding/json"
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
	kubeUtil, _ := kube.New(kubeclient)

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

	for _, namespace := range namespaces.Items {
		var setAdGroupsAnnotation []string
		json.Unmarshal([]byte(namespace.Annotations[kube.AdGroupsAnnotation]), &setAdGroupsAnnotation)
		assert.Equal(t, adGroups, setAdGroupsAnnotation)
	}
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
