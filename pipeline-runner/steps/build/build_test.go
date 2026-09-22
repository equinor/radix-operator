package build_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/equinor/radix-common/utils/slice"
	buildjobmock "github.com/equinor/radix-operator/pipeline-runner/internal/jobs/build/mock"
	internalwait "github.com/equinor/radix-operator/pipeline-runner/internal/wait"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/build"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	jobutil "github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_RunBuildTestSuite(t *testing.T) {
	suite.Run(t, new(buildTestSuite))
}

type buildTestSuite struct {
	suite.Suite
	kubeClient    *kubefake.Clientset
	radixClient   *radixfake.Clientset
	dynamicClient client.Client
	kubeUtil      *kube.Kube
	ctrl          *gomock.Controller
	kedaClient    *kedafake.Clientset
}

func (s *buildTestSuite) SetupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset() // nolint:staticcheck // SA1019: Ignore linting deprecated fields
	s.kedaClient = kedafake.NewSimpleClientset()
	s.dynamicClient = test.CreateClient()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.kedaClient, nil)
	s.ctrl = gomock.NewController(s.T())
}

func (s *buildTestSuite) SetupSubTest() {
	s.setupTest()
}

func (s *buildTestSuite) setupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset() // nolint:staticcheck // SA1019: Ignore linting deprecated fields
	s.kedaClient = kedafake.NewSimpleClientset()
	s.dynamicClient = test.CreateClient()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.kedaClient, nil)
	s.ctrl = gomock.NewController(s.T())
}

func (s *buildTestSuite) Test_TargetEnvironmentsEmpty_ShouldSkip() {
	rr := utils.ARadixRegistration().WithName("any").BuildRR()
	jobWaiter := internalwait.NewMockJobCompletionWaiter(s.ctrl)
	jobWaiter.EXPECT().Wait(gomock.Any()).Return(nil).Times(0)
	jobsBuilder := buildjobmock.NewMockJobsBuilder(gomock.NewController(s.T()))
	cli := build.NewBuildStep(jobWaiter, jobsBuilder)
	cli.Init(s.T().Context(), s.kubeClient, s.radixClient, s.dynamicClient, nil, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments:    model.PipelineArguments{},
		TargetEnvironments:   []model.TargetEnvironment{},
		BuildComponentImages: pipeline.EnvironmentBuildComponentImages{"anyenv": {{ComponentName: "anycomp"}}},
	}

	err := cli.Run(s.T().Context(), pipelineInfo)
	s.Require().NoError(err)
}

func (s *buildTestSuite) Test_BuildComponentImagesEmpty_ShouldSkip() {
	rr := utils.ARadixRegistration().WithName("any").BuildRR()
	jobWaiter := internalwait.NewMockJobCompletionWaiter(s.ctrl)
	jobWaiter.EXPECT().Wait(gomock.Any()).Return(nil).Times(0)
	jobsBuilder := buildjobmock.NewMockJobsBuilder(gomock.NewController(s.T()))
	cli := build.NewBuildStep(jobWaiter, jobsBuilder)
	cli.Init(s.T().Context(), s.kubeClient, s.radixClient, s.dynamicClient, nil, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments:    model.PipelineArguments{},
		TargetEnvironments:   []model.TargetEnvironment{{Environment: "anyenv"}},
		BuildComponentImages: pipeline.EnvironmentBuildComponentImages{},
	}

	err := cli.Run(s.T().Context(), pipelineInfo)
	s.Require().NoError(err)
}

func (s *buildTestSuite) Test_WithBuildSecrets_Validation() {
	const (
		appName    = "anyapp"
		jobName    = "anyjob"
		secretName = "thesecret"
	)
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	ra := utils.ARadixApplication().WithBuildSecrets(secretName).BuildRA()
	rj := utils.ARadixBuildDeployJob().WithJobName(jobName).WithAppName(appName).BuildRJ()
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(s.T().Context(), rj, metav1.CreateOptions{})
	jobWaiter := internalwait.NewMockJobCompletionWaiter(s.ctrl)
	jobWaiter.EXPECT().Wait(gomock.Any()).Return(nil).AnyTimes()
	jobsBuilder := buildjobmock.NewMockJobsBuilder(gomock.NewController(s.T()))
	cli := build.NewBuildStep(jobWaiter, jobsBuilder)
	cli.Init(s.T().Context(), s.kubeClient, s.radixClient, s.dynamicClient, nil, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName: jobName,
		},
		TargetEnvironments:   []model.TargetEnvironment{{Environment: "anyenv"}},
		BuildComponentImages: pipeline.EnvironmentBuildComponentImages{"anyenv": {}},
		RadixApplication:     ra,
	}

	err := cli.Run(s.T().Context(), pipelineInfo)
	s.ErrorContains(err, "build secrets has not been set")

	// secret key missing
	pipelineInfo.BuildSecret = &corev1.Secret{Data: map[string][]byte{}}
	err = cli.Run(s.T().Context(), pipelineInfo)
	s.ErrorContains(err, fmt.Sprintf("build secret %s has not been set", secretName))

	// secret set to default value
	pipelineInfo.BuildSecret = &corev1.Secret{Data: map[string][]byte{secretName: []byte(defaults.BuildSecretDefaultData)}}
	err = cli.Run(s.T().Context(), pipelineInfo)
	s.ErrorContains(err, fmt.Sprintf("build secret %s has not been set", secretName))

	// secret correctly set
	jobsBuilder.EXPECT().BuildJobs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	pipelineInfo.BuildSecret = &corev1.Secret{Data: map[string][]byte{secretName: []byte("anyvalue")}}
	err = cli.Run(s.T().Context(), pipelineInfo)
	s.NoError(err)
}

func (s *buildTestSuite) Test_AppWithoutBuildSecrets_Validation() {
	const (
		appName = "anyapp"
		jobName = "anyjob"
	)
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	ra := utils.ARadixApplication().WithBuildSecrets().BuildRA()
	rj := utils.ARadixBuildDeployJob().WithJobName(jobName).WithAppName(appName).BuildRJ()
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(s.T().Context(), rj, metav1.CreateOptions{})
	jobWaiter := internalwait.NewMockJobCompletionWaiter(s.ctrl)
	jobWaiter.EXPECT().Wait(gomock.Any()).Return(nil).AnyTimes()
	jobsBuilder := buildjobmock.NewMockJobsBuilder(gomock.NewController(s.T()))
	jobsBuilder.EXPECT().BuildJobs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	cli := build.NewBuildStep(jobWaiter, jobsBuilder)
	cli.Init(s.T().Context(), s.kubeClient, s.radixClient, s.dynamicClient, nil, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName: jobName,
		},
		TargetEnvironments:   []model.TargetEnvironment{{Environment: "anyenv"}},
		BuildComponentImages: pipeline.EnvironmentBuildComponentImages{"anyenv": {}},
		RadixApplication:     ra,
	}

	err := cli.Run(s.T().Context(), pipelineInfo)
	s.NoError(err)
}

func (s *buildTestSuite) Test_JobsBuilderCalledAndJobsCreated() {
	const (
		appName = "anyapp"
		jobName = "anyjob"
	)
	var (
		buildSecrets   []string                       = []string{"secret1", "secret2"}
		env1Components []pipeline.BuildComponentImage = []pipeline.BuildComponentImage{{ComponentName: "env1comp1"}, {ComponentName: "env1comp2"}}
		env2Components []pipeline.BuildComponentImage = []pipeline.BuildComponentImage{{ComponentName: "env2comp1"}, {ComponentName: "env2comp2"}}
	)
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	ra := utils.ARadixApplication().WithBuildSecrets(buildSecrets...).BuildRA()
	rj := utils.ARadixBuildDeployJob().WithJobName(jobName).WithAppName(appName).BuildRJ()
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(s.T().Context(), rj, metav1.CreateOptions{})
	jobWaiter := internalwait.NewMockJobCompletionWaiter(s.ctrl)
	jobWaiter.EXPECT().Wait(gomock.Any()).Return(nil).AnyTimes()
	jobsBuilder := buildjobmock.NewMockJobsBuilder(gomock.NewController(s.T()))
	cli := build.NewBuildStep(jobWaiter, jobsBuilder)
	cli.Init(s.T().Context(), s.kubeClient, s.radixClient, s.dynamicClient, nil, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName: jobName,
		},
		TargetEnvironments:   []model.TargetEnvironment{{Environment: "env1"}, {Environment: "env2"}},
		BuildComponentImages: pipeline.EnvironmentBuildComponentImages{"env1": env1Components, "env2": env2Components},
		RadixApplication:     ra,
		GitCommitHash:        "anycommithash",
		GitTags:              "anygittags",
		BuildSecret: &corev1.Secret{Data: map[string][]byte{
			buildSecrets[0]: []byte("secretdata"),
			buildSecrets[1]: []byte("secretdata"),
		}},
	}

	jobsToReturn := []batchv1.Job{
		{ObjectMeta: metav1.ObjectMeta{Name: "job1", Namespace: utils.GetAppNamespace(appName)}},
		{ObjectMeta: metav1.ObjectMeta{Name: "job2", Namespace: utils.GetAppNamespace(appName)}},
	}
	jobsBuilder.EXPECT().BuildJobs(
		gomock.Any(),
		gomock.Any(),
		pipelineInfo.GitCommitHash,
		pipelineInfo.GitTags,
		gomock.InAnyOrder(slices.Concat(env1Components, env2Components)),
		gomock.InAnyOrder(buildSecrets),
	).Return(jobsToReturn).Times(1)

	err := cli.Run(s.T().Context(), pipelineInfo)
	s.NoError(err)
	expectedOwnerRef, _ := jobutil.GetOwnerReferenceOfJob(s.T().Context(), s.radixClient, utils.GetAppNamespace(appName), pipelineInfo.PipelineArguments.JobName)
	expectedJobs := slice.Map(jobsToReturn, func(j batchv1.Job) batchv1.Job {
		newjob := j.DeepCopy()
		newjob.OwnerReferences = expectedOwnerRef
		return *newjob
	})
	actualJobs, _ := s.kubeClient.BatchV1().Jobs(utils.GetAppNamespace(appName)).List(s.T().Context(), metav1.ListOptions{})
	s.ElementsMatch(expectedJobs, actualJobs.Items)
}
