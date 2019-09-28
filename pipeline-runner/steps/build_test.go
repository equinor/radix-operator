package steps

import (
	"fmt"
	"testing"

	"github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/test"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
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
	kubeUtil, _ := kube.New(kubeclient)

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

	applicationConfig, _ := application.NewApplicationConfig(kubeclient, radixclient, nil, rr, ra)
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

func Test_dockerfile_from_build_folder(t *testing.T) {
	dockerfile := getDockerfile(".", "")

	assert.Equal(t, fmt.Sprintf("%s/Dockerfile", git.Workspace), dockerfile)
}

func Test_dockerfile_from_folder(t *testing.T) {
	dockerfile := getDockerfile("/afolder/", "")

	assert.Equal(t, fmt.Sprintf("%s/afolder/Dockerfile", git.Workspace), dockerfile)
}

func Test_dockerfile_from_folder_2(t *testing.T) {
	dockerfile := getDockerfile("afolder", "")

	assert.Equal(t, fmt.Sprintf("%s/afolder/Dockerfile", git.Workspace), dockerfile)
}

func Test_dockerfile_from_folder_special_char(t *testing.T) {
	dockerfile := getDockerfile("./afolder/", "")

	assert.Equal(t, fmt.Sprintf("%s/afolder/Dockerfile", git.Workspace), dockerfile)
}

func Test_dockerfile_from_folder_and_file(t *testing.T) {
	dockerfile := getDockerfile("/afolder/", "Dockerfile.adockerfile")

	assert.Equal(t, fmt.Sprintf("%s/afolder/Dockerfile.adockerfile", git.Workspace), dockerfile)
}

func Test_getGitCloneCommand(t *testing.T) {
	gitCloneCommand := getGitCloneCommand("git@github.com:equinor/radix-github-webhook.git", "master")

	assert.Equal(t, "git clone git@github.com:equinor/radix-github-webhook.git -b master .", gitCloneCommand)
}

func Test_getInitContainerArgString_NoCommitID(t *testing.T) {
	gitCloneCommand := getGitCloneCommand("git@github.com:equinor/radix-github-webhook.git", "master")
	argString := getInitContainerArgString("/workspace", gitCloneCommand, "")

	assert.Equal(t, "apk add --no-cache bash openssh-client git && ls /root/.ssh && cd /workspace && git clone git@github.com:equinor/radix-github-webhook.git -b master .", argString)
}

func Test_getInitContainerArgString_WithCommitID(t *testing.T) {
	gitCloneCommand := getGitCloneCommand("git@github.com:equinor/radix-github-webhook.git", "master")
	argString := getInitContainerArgString("/workspace", gitCloneCommand, "762235917d04dd541715df25a428b5f62394834c")

	assert.Equal(t, "apk add --no-cache bash openssh-client git && ls /root/.ssh && cd /workspace && git clone git@github.com:equinor/radix-github-webhook.git -b master . && git checkout 762235917d04dd541715df25a428b5f62394834c", argString)
}
