package pipelines_test

import (
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/pipelines"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/stretchr/testify/assert"
	kubernetes "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func setupTest(t *testing.T) (*kubernetes.Clientset, *radix.Clientset, *secretproviderfake.Clientset, commonTest.Utils) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	testUtils := commonTest.NewTestUtils(kubeclient, radixclient, secretproviderclient)
	testUtils.CreateClusterPrerequisites("AnyClusterName", "0.0.0.0", "anysubid")
	return kubeclient, radixclient, secretproviderclient, testUtils
}

func TestPrepare_NoRegistration_NotValid(t *testing.T) {
	kubeclient, radixclient, secretproviderclient, _ := setupTest(t)
	pipelineDefinition, _ := pipeline.GetPipelineFromName(string(v1.BuildDeploy))
	cli := pipelines.InitRunner(kubeclient, radixclient, &monitoring.Clientset{}, secretproviderclient, pipelineDefinition, "any-app")

	err := cli.PrepareRun(&model.PipelineArguments{})
	assert.Error(t, err)
}
