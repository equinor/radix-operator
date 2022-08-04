package onpush

import (
	"github.com/equinor/radix-operator/pipeline-runner/model/mock"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/model/env"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/stretchr/testify/assert"
	kubernetes "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	deployTestFilePath = "./testdata/radixconfig.variable.yaml"
	clusterName        = "AnyClusterName"
	containerRegistry  = "any.container.registry"
	egressIps          = "0.0.0.0"
)

func setupTest(t *testing.T) (*kubernetes.Clientset, *radix.Clientset, *secretproviderfake.Clientset, commonTest.Utils, env.Env) {
	mockCtrl := gomock.NewController(t)
	mockEnv := mock.NewMockEnv(mockCtrl)
	mockEnv.EXPECT().GetLogLevel().Return(string(env.LogLevelInfo)).AnyTimes()
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	testUtils := commonTest.NewTestUtils(kubeclient, radixclient, secretproviderclient)
	testUtils.CreateClusterPrerequisites(clusterName, containerRegistry, egressIps)
	return kubeclient, radixclient, secretproviderclient, testUtils, mockEnv
}

func TestPrepare_NoRegistration_NotValid(t *testing.T) {
	kubeclient, radixclient, secretproviderclient, _, env := setupTest(t)
	pipelineDefinition, _ := pipeline.GetPipelineFromName(string(v1.BuildDeploy))
	cli := InitRunner(kubeclient, radixclient, &monitoring.Clientset{}, secretproviderclient, pipelineDefinition, "any-app", env)

	err := cli.PrepareRun(model.PipelineArguments{})
	assert.Error(t, err)
}
