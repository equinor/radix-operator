package build_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/equinor/radix-common/utils/slice"
	internalbuild "github.com/equinor/radix-operator/pipeline-runner/internal/jobs/build"
	buildjobmock "github.com/equinor/radix-operator/pipeline-runner/internal/jobs/build/mock"
	internalwait "github.com/equinor/radix-operator/pipeline-runner/internal/wait"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/build"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	jobutil "github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	_ "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const (
	buildJobFactoryMockMethodName = "BuildJobFactory"
)

func createbuildJobFactoryMock(m *mock.Mock) build.BuildJobFactory {
	return func(useBuildKit bool) internalbuild.JobsBuilder {
		return m.MethodCalled(buildJobFactoryMockMethodName, useBuildKit).Get(0).(internalbuild.JobsBuilder)
	}
}

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
	kedaClient  *kedafake.Clientset
}

func (s *buildTestSuite) SetupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.kedaClient = kedafake.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.kedaClient, nil)
	s.ctrl = gomock.NewController(s.T())
}

func (s *buildTestSuite) SetupSubTest() {
	s.setupTest()
}

func (s *buildTestSuite) setupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.kedaClient = kedafake.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.kedaClient, nil)
	s.ctrl = gomock.NewController(s.T())
}

func (s *buildTestSuite) Test_TargetEnvironmentsEmpty_ShouldSkip() {
	rr := utils.ARadixRegistration().WithName("any").BuildRR()
	jobWaiter := internalwait.NewMockJobCompletionWaiter(s.ctrl)
	jobWaiter.EXPECT().Wait(gomock.Any()).Return(nil).Times(0)
	var m mock.Mock
	cli := build.NewBuildStep(jobWaiter, build.WithBuildJobFactory(createbuildJobFactoryMock(&m)))
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments:    model.PipelineArguments{},
		TargetEnvironments:   []string{},
		BuildComponentImages: pipeline.EnvironmentBuildComponentImages{"anyenv": {{ComponentName: "anycomp"}}},
	}

	err := cli.Run(context.Background(), pipelineInfo)
	s.Require().NoError(err)
	m.AssertNotCalled(s.T(), buildJobFactoryMockMethodName, mock.Anything)
}

func (s *buildTestSuite) Test_BuildComponentImagesEmpty_ShouldSkip() {
	rr := utils.ARadixRegistration().WithName("any").BuildRR()
	jobWaiter := internalwait.NewMockJobCompletionWaiter(s.ctrl)
	jobWaiter.EXPECT().Wait(gomock.Any()).Return(nil).Times(0)
	var m mock.Mock
	cli := build.NewBuildStep(jobWaiter, build.WithBuildJobFactory(createbuildJobFactoryMock(&m)))
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments:    model.PipelineArguments{},
		TargetEnvironments:   []string{"anyenv"},
		BuildComponentImages: pipeline.EnvironmentBuildComponentImages{},
	}

	err := cli.Run(context.Background(), pipelineInfo)
	s.Require().NoError(err)
	m.AssertNotCalled(s.T(), buildJobFactoryMockMethodName, mock.Anything)
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
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(context.Background(), rj, metav1.CreateOptions{})
	jobWaiter := internalwait.NewMockJobCompletionWaiter(s.ctrl)
	jobWaiter.EXPECT().Wait(gomock.Any()).Return(nil).AnyTimes()
	var m mock.Mock
	cli := build.NewBuildStep(jobWaiter, build.WithBuildJobFactory(createbuildJobFactoryMock(&m)))
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName: jobName,
		},
		TargetEnvironments:   []string{"anyenv"},
		BuildComponentImages: pipeline.EnvironmentBuildComponentImages{"anyenv": {}},
		RadixApplication:     ra,
	}

	err := cli.Run(context.Background(), pipelineInfo)
	s.ErrorContains(err, "build secrets has not been set")
	m.AssertNotCalled(s.T(), buildJobFactoryMockMethodName, mock.Anything)

	// secret key missing
	pipelineInfo.BuildSecret = &corev1.Secret{Data: map[string][]byte{}}
	err = cli.Run(context.Background(), pipelineInfo)
	s.ErrorContains(err, fmt.Sprintf("build secret %s has not been set", secretName))
	m.AssertNotCalled(s.T(), buildJobFactoryMockMethodName, mock.Anything)

	// secret set to default value
	pipelineInfo.BuildSecret = &corev1.Secret{Data: map[string][]byte{secretName: []byte(defaults.BuildSecretDefaultData)}}
	err = cli.Run(context.Background(), pipelineInfo)
	s.ErrorContains(err, fmt.Sprintf("build secret %s has not been set", secretName))
	m.AssertNotCalled(s.T(), buildJobFactoryMockMethodName, mock.Anything)

	// secret correctly set
	jobBuilder := buildjobmock.NewMockJobsBuilder(gomock.NewController(s.T()))
	jobBuilder.EXPECT().BuildJobs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	m.On(buildJobFactoryMockMethodName, mock.Anything).Return(jobBuilder)
	pipelineInfo.BuildSecret = &corev1.Secret{Data: map[string][]byte{secretName: []byte("anyvalue")}}
	err = cli.Run(context.Background(), pipelineInfo)
	s.NoError(err)
	m.AssertExpectations(s.T())
}

func (s *buildTestSuite) Test_AppWithoutBuildSecrets_Validation() {
	const (
		appName = "anyapp"
		jobName = "anyjob"
	)
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	ra := utils.ARadixApplication().WithBuildSecrets().BuildRA()
	rj := utils.ARadixBuildDeployJob().WithJobName(jobName).WithAppName(appName).BuildRJ()
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(context.Background(), rj, metav1.CreateOptions{})
	jobWaiter := internalwait.NewMockJobCompletionWaiter(s.ctrl)
	jobWaiter.EXPECT().Wait(gomock.Any()).Return(nil).AnyTimes()
	var m mock.Mock
	cli := build.NewBuildStep(jobWaiter, build.WithBuildJobFactory(createbuildJobFactoryMock(&m)))
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName: jobName,
		},
		TargetEnvironments:   []string{"anyenv"},
		BuildComponentImages: pipeline.EnvironmentBuildComponentImages{"anyenv": {}},
		RadixApplication:     ra,
	}

	jobBuilder := buildjobmock.NewMockJobsBuilder(gomock.NewController(s.T()))
	jobBuilder.EXPECT().BuildJobs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	m.On(buildJobFactoryMockMethodName, mock.Anything).Return(jobBuilder)
	err := cli.Run(context.Background(), pipelineInfo)
	s.NoError(err)
	m.AssertExpectations(s.T())
}

func (s *buildTestSuite) Test_JobsBuilderCalledAndJobsCreated() {
	const (
		appName  = "anyapp"
		jobName  = "anyjob"
		cloneUrl = "anycloneurl"
	)
	var (
		buildSecrets   []string                       = []string{"secret1", "secret2"}
		env1Components []pipeline.BuildComponentImage = []pipeline.BuildComponentImage{{ComponentName: "env1comp1"}, {ComponentName: "env1comp2"}}
		env2Components []pipeline.BuildComponentImage = []pipeline.BuildComponentImage{{ComponentName: "env2comp1"}, {ComponentName: "env2comp2"}}
	)
	rr := utils.ARadixRegistration().WithName(appName).WithCloneURL(cloneUrl).BuildRR()
	ra := utils.ARadixApplication().WithBuildSecrets(buildSecrets...).BuildRA()
	rj := utils.ARadixBuildDeployJob().WithJobName(jobName).WithAppName(appName).BuildRJ()
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(context.Background(), rj, metav1.CreateOptions{})
	jobWaiter := internalwait.NewMockJobCompletionWaiter(s.ctrl)
	jobWaiter.EXPECT().Wait(gomock.Any()).Return(nil).AnyTimes()
	var m mock.Mock
	cli := build.NewBuildStep(jobWaiter, build.WithBuildJobFactory(createbuildJobFactoryMock(&m)))
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName: jobName,
		},
		TargetEnvironments:   []string{"env1", "env2"},
		BuildComponentImages: pipeline.EnvironmentBuildComponentImages{"env1": env1Components, "env2": env2Components},
		RadixApplication:     ra,
		GitCommitHash:        "anycommithash",
		GitTags:              "anygittags",
		BuildSecret: &corev1.Secret{Data: map[string][]byte{
			buildSecrets[0]: []byte("secretdata"),
			buildSecrets[1]: []byte("secretdata"),
		}},
	}

	jobBuilder := buildjobmock.NewMockJobsBuilder(gomock.NewController(s.T()))
	jobsToReturn := []batchv1.Job{
		{ObjectMeta: metav1.ObjectMeta{Name: "job1", Namespace: utils.GetAppNamespace(appName)}},
		{ObjectMeta: metav1.ObjectMeta{Name: "job2", Namespace: utils.GetAppNamespace(appName)}},
	}
	jobBuilder.EXPECT().BuildJobs(
		gomock.Any(),
		pipelineInfo.PipelineArguments,
		cloneUrl,
		pipelineInfo.GitCommitHash,
		pipelineInfo.GitTags,
		gomock.InAnyOrder(slices.Concat(env1Components, env2Components)),
		gomock.InAnyOrder(buildSecrets),
	).Return(jobsToReturn).Times(1)
	m.On(buildJobFactoryMockMethodName, mock.Anything).Return(jobBuilder)
	err := cli.Run(context.Background(), pipelineInfo)
	s.NoError(err)
	m.AssertExpectations(s.T())
	expectedOwnerRef, _ := jobutil.GetOwnerReferenceOfJob(context.Background(), s.radixClient, utils.GetAppNamespace(appName), pipelineInfo.PipelineArguments.JobName)
	expectedJobs := slice.Map(jobsToReturn, func(j batchv1.Job) batchv1.Job {
		newjob := j.DeepCopy()
		newjob.OwnerReferences = expectedOwnerRef
		return *newjob
	})
	actualJobs, _ := s.kubeClient.BatchV1().Jobs(utils.GetAppNamespace(appName)).List(context.Background(), metav1.ListOptions{})
	s.ElementsMatch(expectedJobs, actualJobs.Items)
}
