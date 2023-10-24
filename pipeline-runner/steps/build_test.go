package steps_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/steps"
	pipelinewait "github.com/equinor/radix-operator/pipeline-runner/wait"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	yamlk8s "sigs.k8s.io/yaml"
)

func Test_RunBuildTestSuite(t *testing.T) {
	suite.Run(t, new(buildTestSuite))
}

type buildTestSuite struct {
	suite.Suite
	kubeClient  *kubefake.Clientset
	radixClient *radixfake.Clientset
	promClient  *prometheusfake.Clientset
	kubeUtil    *kube.Kube
	ctrl        *gomock.Controller
}

func (s *buildTestSuite) SetupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, nil)
	s.ctrl = gomock.NewController(s.T())
}

func (s *buildTestSuite) Test_BranchIsNotMapped_ShouldSkip() {
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

	cli := steps.NewBuildStep(nil)
	cli.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	applicationConfig, _ := application.NewApplicationConfig(s.kubeClient, s.kubeUtil, s.radixClient, rr, ra)
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
	s.Require().NoError(err)
	radixJobList, err := s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(anyAppName)).List(context.Background(), metav1.ListOptions{})
	s.NoError(err)
	s.Empty(radixJobList.Items)
}

func (s *buildTestSuite) Test_ACRBuildJobSpec() {
	appName, rjName := "anyapp", "anyrj"
	configMapName := "anycm"
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	rj := utils.ARadixBuildDeployJob().WithJobName(rjName).WithAppName(appName).BuildRJ()
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(context.Background(), rj, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment("dev", "main").
		WithComponents(
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("c1"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("c2").WithSourceFolder("c2src"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("c3").WithDockerfileName("c3docker"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("c4").WithSourceFolder("c4src").WithDockerfileName("c4docker"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("c5").WithDockerfileName("compcommondocker"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("c6").WithDockerfileName("compcommondocker"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("c7").WithImage("anyimage"),
		).
		BuildRA()
	raBytes, _ := yamlk8s.Marshal(ra)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: configMapName},
		Data: map[string]string{
			pipelineDefaults.PipelineConfigMapContent: string(raBytes),
		},
	}
	_, _ = s.kubeClient.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Create(context.Background(), cm, metav1.CreateOptions{})
	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			PipelineType: "build-deploy",
			Branch:       "main",
			JobName:      rjName,
		},
		RadixConfigMapName: configMapName,
	}
	jobWaiter := pipelinewait.NewMockJobCompletionWaiter(s.ctrl)
	jobWaiter.EXPECT().Wait(gomock.Any()).Return(nil).Times(1)

	applyStep := steps.NewApplyConfigStep()
	applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	buildStep := steps.NewBuildStep(jobWaiter)
	buildStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	s.Require().NoError(applyStep.Run(&pipeline))
	s.Require().NoError(buildStep.Run(&pipeline))
	jobs, _ := s.kubeClient.BatchV1().Jobs(utils.GetAppNamespace(appName)).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
}
