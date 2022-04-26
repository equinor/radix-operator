package steps

import (
	"github.com/equinor/radix-operator/pipeline-runner/model/env"
	"github.com/equinor/radix-operator/pipeline-runner/model/mock"
	"github.com/golang/mock/gomock"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/stretchr/testify/assert"
	kubernetes "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func setupTest(t *testing.T) (*kubernetes.Clientset, *kube.Kube, *radix.Clientset, commonTest.Utils, env.Env) {
	mockCtrl := gomock.NewController(t)
	mockEnv := mock.NewMockEnv(mockCtrl)
	mockEnv.EXPECT().GetLogLevel().Return(string(env.LogLevelInfo)).AnyTimes()
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	testUtils := commonTest.NewTestUtils(kubeclient, radixclient, secretproviderclient)
	testUtils.CreateClusterPrerequisites(anyClusterName, anyContainerRegistry, egressIps)
	kubeUtil, _ := kube.New(kubeclient, radixclient, secretproviderclient)

	return kubeclient, kubeUtil, radixclient, testUtils, mockEnv
}

func TestBuild_BranchIsNotMapped_ShouldSkip(t *testing.T) {
	kubeclient, kube, radixclient, _, env := setupTest(t)

	anyBranch := "master"
	anyEnvironment := "dev"
	anyComponentName := "app"
	anyNoMappedBranch := "feature"

	rr := utils.ARadixRegistration().
		WithName(anyAppName).
		BuildRR()

	ra := utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, anyBranch).
		WithComponents(
			utils.AnApplicationComponent().
				WithName(anyComponentName)).
		BuildRA()

	// Prometheus doesn´t contain any fake
	cli := NewBuildStep()
	cli.Init(kubeclient, radixclient, kube, &monitoring.Clientset{}, rr, env)

	applicationConfig, _ := application.NewApplicationConfig(kubeclient, kube, radixclient, rr, ra)
	branchIsMapped, targetEnvs := applicationConfig.IsThereAnythingToDeploy(anyNoMappedBranch)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName:  anyJobName,
			ImageTag: anyImageTag,
			Branch:   anyNoMappedBranch,
			CommitID: anyCommitID,
		},
		TargetEnvironments: targetEnvs,
		BranchIsMapped:     branchIsMapped,
	}

	err := cli.Run(pipelineInfo)
	assert.Error(t, err)

}
