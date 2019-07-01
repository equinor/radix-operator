package onpush

import (
	"testing"

	"github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
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
	ra := utils.NewRadixApplicationBuilder().
		WithAppName("any-app").
		WithEnvironment("dev", "dev").
		WithEnvironment("prod", "").
		WithComponents(utils.AnApplicationComponent().WithPort("http", 8080)).
		BuildRA()

	pipelineDefinition, _ := pipeline.GetPipelineFromName(pipeline.BuildDeploy)
	cli := InitRunner(kubeclient, radixclient, &monitoring.Clientset{}, pipelineDefinition, ra)

	err := cli.PrepareRun(model.PipelineArguments{})
	assert.Error(t, err)
}
