package applicationconfig

import (
	"fmt"
	"io/ioutil"
	"log"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

const (
	sampleRegistration = "./testdata/sampleregistration.yaml"
	sampleApp          = "./testdata/radixconfig.yaml"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

func getApplication(ra *radixv1.RadixApplication) *ApplicationConfig {
	// The other arguments are not relevant for this test
	application, _ := NewApplicationConfig(nil, nil, nil, ra)
	return application
}

func Test_Create_Radix_Environments(t *testing.T) {
	radixRegistration, _ := utils.GetRadixRegistrationFromFile(sampleRegistration)
	radixApp, _ := utils.GetRadixApplication(sampleApp)

	kubeclient := kubernetes.NewSimpleClientset()
	radixClient := radix.NewSimpleClientset()
	app, _ := NewApplicationConfig(kubeclient, radixClient, radixRegistration, radixApp)

	label := fmt.Sprintf("%s=%s", kube.RadixAppLabel, radixRegistration.Name)
	t.Run("It can create environments", func(t *testing.T) {
		err := app.createEnvironments()
		assert.NoError(t, err)
		namespaces, _ := kubeclient.CoreV1().Namespaces().List(metav1.ListOptions{
			LabelSelector: label,
		})
		assert.Len(t, namespaces.Items, 2)
	})

	t.Run("It doesn't fail when re-running creation", func(t *testing.T) {
		err := app.createEnvironments()
		assert.NoError(t, err)
		namespaces, _ := kubeclient.CoreV1().Namespaces().List(metav1.ListOptions{
			LabelSelector: label,
		})
		assert.Len(t, namespaces.Items, 2)
	})
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

func TestApplyConfig_Create(t *testing.T) {
	newRA := utils.NewRadixApplicationBuilder().
		WithAppName("test").
		WithEnvironment("dev", "master").
		WithComponent(utils.NewApplicationComponentBuilder().
			WithName("www").
			WithPort("http", 3000).
			WithPublic(true)).
		BuildRA()

	updateRA := utils.NewRadixApplicationBuilder().
		WithAppName("test").
		WithEnvironment("dev", "master").
		WithComponent(utils.NewApplicationComponentBuilder().
			WithName("www").
			WithPort("http", 3000)).
		BuildRA()

	kubeclient := kubernetes.NewSimpleClientset()
	radixClient := radix.NewSimpleClientset()
	app, _ := NewApplicationConfig(kubeclient, radixClient, nil, newRA)

	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-app", app.config.Name),
		},
	}
	existingNamespace, _ := app.kubeclient.CoreV1().Namespaces().Create(&namespace)

	t.Run("It can create new RadixApplication", func(t *testing.T) {
		app.config.SetResourceVersion("123")
		err := app.ApplyConfigToApplicationNamespace()
		assert.NoError(t, err)
		createdRA, err := app.radixclient.RadixV1().RadixApplications(existingNamespace.Name).Get(app.config.Name, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, createdRA)
		assert.Equal(t, "test", createdRA.Name)
		assert.Equal(t, "123", createdRA.ResourceVersion)
		assert.Equal(t, true, createdRA.Spec.Components[0].Public)
	})

	t.Run("It can update existing RadixApplication", func(t *testing.T) {
		app.config = updateRA
		// Warning: Radix fake client doesn't care about ResourceVersion when updating. This is different in reality.
		app.config.SetResourceVersion("456")
		err := app.ApplyConfigToApplicationNamespace()
		assert.NoError(t, err)
		updatedRA, err := app.radixclient.RadixV1().RadixApplications(existingNamespace.Name).Get(app.config.Name, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, updatedRA)
		assert.Equal(t, "test", updatedRA.Name)
		assert.Equal(t, false, updatedRA.Spec.Components[0].Public)
	})

}
