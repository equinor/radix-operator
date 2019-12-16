package steps

import (
	"testing"

	monitoring "github.com/coreos/prometheus-operator/pkg/client/versioned"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/test"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kubernetes "k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
)

func setupTest() (*kubernetes.Clientset, *kube.Kube, *radix.Clientset, test.Utils) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	testUtils := commonTest.NewTestUtils(kubeclient, radixclient)
	testUtils.CreateClusterPrerequisites(anyClusterName, anyContainerRegistry)
	kubeUtil, _ := kube.New(kubeclient, radixclient)

	return kubeclient, kubeUtil, radixclient, testUtils
}

func TestBuild_BranchIsNotMapped_ShouldSkip(t *testing.T) {
	kubeclient, kube, radixclient, _ := setupTest()

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
	cli.Init(kubeclient, radixclient, kube, &monitoring.Clientset{}, rr, ra)

	applicationConfig, _ := application.NewApplicationConfig(kubeclient, kube, radixclient, rr, ra)
	branchIsMapped, targetEnvs := applicationConfig.IsBranchMappedToEnvironment(anyNoMappedBranch)

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
