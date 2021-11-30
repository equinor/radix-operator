package onpush

import (
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/stretchr/testify/assert"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

const (
	deployTestFilePath = "./testdata/radixconfig.variable.yaml"
	clusterName        = "AnyClusterName"
	containerRegistry  = "any.container.registry"
	egressIps          = "0.0.0.0"
)

func setupTest() (*kubernetes.Clientset, *radix.Clientset, test.Utils) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	testUtils := commonTest.NewTestUtils(kubeclient, radixclient)
	testUtils.CreateClusterPrerequisites(clusterName, containerRegistry, egressIps)
	return kubeclient, radixclient, testUtils
}

func TestPrepare_NoRegistration_NotValid(t *testing.T) {
	kubeclient, radixclient, _ := setupTest()
	pipelineDefinition, _ := pipeline.GetPipelineFromName(string(v1.BuildDeploy))
	cli := InitRunner(kubeclient, radixclient, &monitoring.Clientset{}, pipelineDefinition, "any-app")

	err := cli.PrepareRun(model.PipelineArguments{})
	assert.Error(t, err)
}
