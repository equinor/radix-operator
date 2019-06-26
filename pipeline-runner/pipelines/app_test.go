package onpush

import (
	"testing"

	"github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pkg/apis/test"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
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
	cli := Init(kubeclient, radixclient, &monitoring.Clientset{})

	ra := utils.NewRadixApplicationBuilder().
		WithAppName("any-app").
		WithEnvironment("dev", "dev").
		WithEnvironment("prod", "").
		WithComponents(utils.AnApplicationComponent().WithPort("http", 8080)).
		BuildRA()

	_, err := cli.Prepare(ra)
	assert.Error(t, err)
}
