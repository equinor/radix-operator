package onpush

import (
	"testing"

	"github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pkg/apis/test"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

const (
	deployTestFilePath = "./testdata/radixconfig.variable.yaml"
	clusterName        = "AnyClusterName"
	containerRegistry  = "any.container.registry"
)

func setupTest() (*kubernetes.Clientset, *radix.Clientset, test.Utils) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	testUtils := commonTest.NewTestUtils(kubeclient, radixclient)
	testUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	return kubeclient, radixclient, testUtils
}

func TestPrepare_NoRegistration_NotValid(t *testing.T) {
	kubeclient, radixclient, _ := setupTest()
	cli, _ := Init(kubeclient, radixclient, &monitoring.Clientset{})

	ra := utils.NewRadixApplicationBuilder().
		WithAppName("any-app").
		WithEnvironment("dev", "dev").
		WithEnvironment("prod", "").
		WithComponents(utils.AnApplicationComponent().WithPort("http", 8080)).
		BuildRA()

	_, _, err := cli.Prepare(ra, "master")
	assert.Error(t, err)
}

func TestPrepare_MasterIsNotMappedToEnvironment_StillItsApplied(t *testing.T) {
	kubeclient, radixclient, commonTestUtils := setupTest()
	cli, _ := Init(kubeclient, radixclient, &monitoring.Clientset{})

	commonTestUtils.ApplyRegistration(utils.ARadixRegistration().
		WithName("any-app"))

	ra := utils.NewRadixApplicationBuilder().
		WithAppName("any-app").
		WithEnvironment("dev", "dev").
		WithEnvironment("prod", "").
		WithComponents(utils.AnApplicationComponent().WithPort("http", 8080)).
		BuildRA()

	cli.Prepare(ra, "master")
	savedRadixApplication, _ := radixclient.RadixV1().RadixApplications(utils.GetAppNamespace("any-app")).Get(ra.Name, metav1.GetOptions{})
	assert.NotNil(t, savedRadixApplication)
}
