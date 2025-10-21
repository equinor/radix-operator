package runner_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/internal/runner"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kubernetes "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func setupTest(t *testing.T) (*kubernetes.Clientset, *radix.Clientset, *kedafake.Clientset, *secretproviderfake.Clientset, commonTest.Utils) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	testUtils := commonTest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient)
	err := testUtils.CreateClusterPrerequisites("AnyClusterName", "anysubid")
	require.NoError(t, err)
	return kubeclient, radixclient, kedaClient, secretproviderclient, testUtils
}

func TestPrepare_NoRegistration_NotValid(t *testing.T) {
	kubeclient, radixclient, kedaClient, secretproviderclient, _ := setupTest(t)
	pipelineDefinition, _ := pipeline.GetPipelineFromName(string(v1.BuildDeploy))
	cli := runner.NewRunner(kubeclient, radixclient, kedaClient, &monitoring.Clientset{}, secretproviderclient, nil, pipelineDefinition, "any-app")

	err := cli.PrepareRun(context.Background(), &model.PipelineArguments{})
	assert.Error(t, err)
}
