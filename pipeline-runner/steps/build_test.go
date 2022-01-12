package steps

import (
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	kubernetes "k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
)

func setupTest() (*kubernetes.Clientset, *kube.Kube, *radix.Clientset, commonTest.Utils) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	testUtils := commonTest.NewTestUtils(kubeclient, radixclient)
	testUtils.CreateClusterPrerequisites(anyClusterName, anyContainerRegistry, egressIps)
	kubeUtil, _ := kube.New(kubeclient, radixclient, secretproviderfake.NewSimpleClientset())

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
	cli.Init(kubeclient, radixclient, kube, &monitoring.Clientset{}, rr)

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
