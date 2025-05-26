package internal_test

import (
	"errors"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/preparepipeline/internal"
	"github.com/equinor/radix-operator/pipeline-runner/utils/git"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	gomock "github.com/golang/mock/gomock"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	tektonfake "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func Test_RunTestSuite(t *testing.T) {
	suite.Run(t, new(stepTestSuite))
}

type stepTestSuite struct {
	suite.Suite
	kubeClient  *kubefake.Clientset
	radixClient *radixfake.Clientset
	kedaClient  *kedafake.Clientset
	promClient  *prometheusfake.Clientset
	kubeUtil    *kube.Kube
	tknClient   tektonclient.Interface
	ctrl        *gomock.Controller
}

func (s *stepTestSuite) SetupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.kedaClient = kedafake.NewSimpleClientset()
	s.tknClient = tektonfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.kedaClient, nil)
	s.ctrl = gomock.NewController(s.T())
}

func (s *stepTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *stepTestSuite) Test_GitCommitHashNoSet() {
	sut := internal.NewContextBuilder(s.kubeUtil)
	gitRepo := git.NewMockRepository(s.ctrl)
	_, err := sut.GetBuildContext(&model.PipelineInfo{}, gitRepo)
	s.ErrorIs(err, internal.ErrMissingGitCommitHash)
}

func (s *stepTestSuite) Test_EmptyTargetEnvironment() {
	sut := internal.NewContextBuilder(s.kubeUtil)
	gitRepo := git.NewMockRepository(s.ctrl)
	actual, err := sut.GetBuildContext(&model.PipelineInfo{GitCommitHash: "anyhash"}, gitRepo)
	s.NoError(err)
	s.Empty(actual)
}

func (s *stepTestSuite) Test_DiffCommitReturnsError() {
	sut := internal.NewContextBuilder(s.kubeUtil)
	gitError := errors.New("any error")
	gitRepo := git.NewMockRepository(s.ctrl)
	gitRepo.EXPECT().DiffCommits(gomock.Any(), gomock.Any()).Times(1).Return(nil, gitError)
	pipelineInfo := &model.PipelineInfo{
		GitCommitHash:      "anyhash",
		TargetEnvironments: []string{"env1", "env2"},
	}
	_, err := sut.GetBuildContext(pipelineInfo, gitRepo)
	s.ErrorIs(err, gitError)
}

func (s *stepTestSuite) Test_DiffCommitsCalledOncePerTargetEnvirononment() {
	sut := internal.NewContextBuilder(s.kubeUtil)
	gitRepo := git.NewMockRepository(s.ctrl)
	gitRepo.EXPECT().DiffCommits(gomock.Any(), gomock.Any()).Times(3).Return(nil, nil)
	pipelineInfo := &model.PipelineInfo{
		GitCommitHash:      "anyhash",
		TargetEnvironments: []string{"env1", "env2", "env3"},
		RadixApplication:   &v1.RadixApplication{},
	}
	actualBuildContext, err := sut.GetBuildContext(pipelineInfo, gitRepo)
	s.Require().NoError(err)
	s.ElementsMatch(actualBuildContext.EnvironmentsToBuild, []model.EnvironmentToBuild{{Environment: "env1"}, {Environment: "env2"}, {Environment: "env3"}})
}

func (s *stepTestSuite) Test_DetectComponentsWithChangedSource() {
	const (
		envName = "anyenv"
	)

	tests := map[string]struct {
		diffs              git.DiffEntries
		components         []utils.RadixApplicationComponentBuilder
		jobs               []utils.RadixApplicationJobComponentBuilder
		expectedComponents []string
	}{
		"changed root file matches all components with root folder": {
			diffs: git.DiffEntries{{Name: "anyfile.txt"}},
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithSourceFolder("/sub1"),
				utils.NewApplicationComponentBuilder().WithName("comp2").WithSourceFolder("/"),
				utils.NewApplicationComponentBuilder().WithName("comp3").WithSourceFolder("."),
				utils.NewApplicationComponentBuilder().WithName("comp4").WithSourceFolder(""),
			},
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithSourceFolder("/sub2"),
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithSourceFolder("/"),
				utils.NewApplicationJobComponentBuilder().WithName("job3").WithSourceFolder("."),
				utils.NewApplicationJobComponentBuilder().WithName("job4").WithSourceFolder(""),
			},
			expectedComponents: []string{"comp2", "comp3", "comp4", "job2", "job3", "job4"},
		},
		"changed root folder matches all components with root folder": {
			diffs: git.DiffEntries{{Name: ".", IsDir: true}},
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithSourceFolder("/sub1"),
				utils.NewApplicationComponentBuilder().WithName("comp2").WithSourceFolder("/"),
				utils.NewApplicationComponentBuilder().WithName("comp3").WithSourceFolder("."),
				utils.NewApplicationComponentBuilder().WithName("comp4").WithSourceFolder(""),
			},
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithSourceFolder("/sub2"),
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithSourceFolder("/"),
				utils.NewApplicationJobComponentBuilder().WithName("job3").WithSourceFolder("."),
				utils.NewApplicationJobComponentBuilder().WithName("job4").WithSourceFolder(""),
			},
			expectedComponents: []string{"comp2", "comp3", "comp4", "job2", "job3", "job4"},
		},
		"changed file in subfolder matches all components with root folder": {
			diffs: git.DiffEntries{{Name: "othersub/anyfile.txt"}},
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithSourceFolder("/sub1"),
				utils.NewApplicationComponentBuilder().WithName("comp2").WithSourceFolder("/"),
				utils.NewApplicationComponentBuilder().WithName("comp3").WithSourceFolder("."),
				utils.NewApplicationComponentBuilder().WithName("comp4").WithSourceFolder(""),
			},
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithSourceFolder("/sub2"),
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithSourceFolder("/"),
				utils.NewApplicationJobComponentBuilder().WithName("job3").WithSourceFolder("."),
				utils.NewApplicationJobComponentBuilder().WithName("job4").WithSourceFolder(""),
			},
			expectedComponents: []string{"comp2", "comp3", "comp4", "job2", "job3", "job4"},
		},
		"changed subfolder matches all components with root folder": {
			diffs: git.DiffEntries{{Name: "othersub", IsDir: true}},
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithSourceFolder("/sub1"),
				utils.NewApplicationComponentBuilder().WithName("comp2").WithSourceFolder("/"),
				utils.NewApplicationComponentBuilder().WithName("comp3").WithSourceFolder("."),
				utils.NewApplicationComponentBuilder().WithName("comp4").WithSourceFolder(""),
			},
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithSourceFolder("/sub2"),
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithSourceFolder("/"),
				utils.NewApplicationJobComponentBuilder().WithName("job3").WithSourceFolder("."),
				utils.NewApplicationJobComponentBuilder().WithName("job4").WithSourceFolder(""),
			},
			expectedComponents: []string{"comp2", "comp3", "comp4", "job2", "job3", "job4"},
		},
		"changed file in component subfolder hierarchy": {
			diffs: git.DiffEntries{{Name: "sub1/sub2/anyfile.txt"}},
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithSourceFolder("/sub1"),
				utils.NewApplicationComponentBuilder().WithName("comp2").WithSourceFolder("/sub2"),
			},
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithSourceFolder("/sub1"),
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithSourceFolder("/sub2"),
			},
			expectedComponents: []string{"comp1", "job1"},
		},
		"changed folder in component subfolder hierarchy": {
			diffs: git.DiffEntries{{Name: "sub1/sub_a", IsDir: true}},
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithSourceFolder("/sub1"),
				utils.NewApplicationComponentBuilder().WithName("comp2").WithSourceFolder("/sub2"),
			},
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithSourceFolder("/sub1"),
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithSourceFolder("/sub2"),
			},
			expectedComponents: []string{"comp1", "job1"},
		},
		"compopnent/job source folders are cleaned": {
			diffs: git.DiffEntries{{Name: "sub1/sub2/anyfile.txt"}},
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithSourceFolder("/sub_a/sub_b/../../sub1/sub_c/.."), // equals /sub1
				utils.NewApplicationComponentBuilder().WithName("comp2").WithSourceFolder("/sub2"),
			},
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithSourceFolder("/sub_a/sub_b/../../sub1/sub_c/.."), // equals /sub1
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithSourceFolder("/sub2"),
			},
			expectedComponents: []string{"comp1", "job1"},
		},
		"diff entries are cleaned": {
			diffs: git.DiffEntries{{Name: "sub_a/sub_b/../../sub1/anyfile.txt"}}, // equals /sub1
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithSourceFolder("/sub1"),
				utils.NewApplicationComponentBuilder().WithName("comp2").WithSourceFolder("/sub2"),
			},
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithSourceFolder("/sub1"),
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithSourceFolder("/sub2"),
			},
			expectedComponents: []string{"comp1", "job1"},
		},
		"all diff entries are checked": {
			diffs: git.DiffEntries{{Name: "sub1/file.txt"}, {Name: "sub3/file.txt"}},
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithSourceFolder("/sub1"),
				utils.NewApplicationComponentBuilder().WithName("comp2").WithSourceFolder("/sub2"),
				utils.NewApplicationComponentBuilder().WithName("comp3").WithSourceFolder("/sub3"),
			},
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithSourceFolder("/sub1"),
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithSourceFolder("/sub2"),
				utils.NewApplicationJobComponentBuilder().WithName("job3").WithSourceFolder("/sub3"),
			},
			expectedComponents: []string{"comp1", "comp3", "job1", "job3"},
		},
		"skip components/jobs with image": {
			diffs: git.DiffEntries{{Name: "."}},
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithSourceFolder(""),
				utils.NewApplicationComponentBuilder().WithName("comp2").WithSourceFolder("").WithImage("anyimage"),
			},
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithSourceFolder(""),
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithSourceFolder("").WithImage("anyimage"),
			},
			expectedComponents: []string{"comp1", "job1"},
		},
		"skip disabled components/jobs": {
			diffs: git.DiffEntries{{Name: "."}},
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithSourceFolder(""),
				utils.NewApplicationComponentBuilder().WithName("comp2").WithSourceFolder("").WithEnabled(false),
			},
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithSourceFolder(""),
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithSourceFolder("").WithEnabled(false),
			},
			expectedComponents: []string{"comp1", "job1"},
		},
		"matching environment config: use source": {
			diffs: git.DiffEntries{{Name: "sub1/file.txt"}},
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithSourceFolder("/sub1").WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment(envName).WithSourceFolder("/sub2")),
				utils.NewApplicationComponentBuilder().WithName("comp2").WithSourceFolder("/sub2").WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment(envName).WithSourceFolder("/sub1")),
			},
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithSourceFolder("/sub1").WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName).WithSourceFolder("/sub2")),
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithSourceFolder("/sub2").WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName).WithSourceFolder("/sub1")),
			},
			expectedComponents: []string{"comp2", "job2"},
		},
		"non-matching environment config: use source from common config": {
			diffs: git.DiffEntries{{Name: "sub1/file.txt"}},
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithSourceFolder("/sub1").WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment("otherenv").WithSourceFolder("/sub2")),
				utils.NewApplicationComponentBuilder().WithName("comp2").WithSourceFolder("/sub2").WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment("otherenv").WithSourceFolder("/sub1")),
			},
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithSourceFolder("/sub1").WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment("otherenv").WithSourceFolder("/sub2")),
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithSourceFolder("/sub2").WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment("otherenv").WithSourceFolder("/sub1")),
			},
			expectedComponents: []string{"comp1", "job1"},
		},
		"matching environment config: skip components/jobs with image set": {
			diffs: git.DiffEntries{{Name: "sub1/file.txt"}},
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithSourceFolder("/sub1").WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment(envName).WithImage("anyimage")),
				utils.NewApplicationComponentBuilder().WithName("comp2").WithSourceFolder("/sub1"),
			},
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithSourceFolder("/sub1").WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName).WithImage("anyimage")),
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithSourceFolder("/sub1"),
			},
			expectedComponents: []string{"comp2", "job2"},
		},
		"matching environment config: skip disabled components/jobs": {
			diffs: git.DiffEntries{{Name: "sub1/file.txt"}},
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithSourceFolder("/sub1").WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment(envName).WithEnabled(false)),
				utils.NewApplicationComponentBuilder().WithName("comp2").WithSourceFolder("/sub1"),
			},
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithSourceFolder("/sub1").WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName).WithEnabled(false)),
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithSourceFolder("/sub1"),
			},
			expectedComponents: []string{"comp2", "job2"},
		},
		"matching environment config: include enabled components/jobs": {
			diffs: git.DiffEntries{{Name: "sub1/file.txt"}},
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithSourceFolder("/sub1").WithEnabled(false).WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment(envName).WithEnabled(true)),
				utils.NewApplicationComponentBuilder().WithName("comp2").WithSourceFolder("/sub1").WithEnabled(false),
			},
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithSourceFolder("/sub1").WithEnabled(false).WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName).WithEnabled(true)),
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithSourceFolder("/sub1").WithEnabled(false),
			},
			expectedComponents: []string{"comp1", "job1"},
		},
	}

	for testName, testSpec := range tests {
		s.Run(testName, func() {
			sut := internal.NewContextBuilder(s.kubeUtil)
			gitRepo := git.NewMockRepository(s.ctrl)
			gitRepo.EXPECT().DiffCommits(gomock.Any(), gomock.Any()).Times(1).Return(testSpec.diffs, nil)
			ra := utils.NewRadixApplicationBuilder().WithAppName("anyname").
				WithComponents(testSpec.components...).
				WithJobComponents(testSpec.jobs...)

			pipelineInfo := &model.PipelineInfo{
				GitCommitHash:      "anyhash",
				TargetEnvironments: []string{envName},
				RadixApplication:   ra.BuildRA(),
			}
			actualBuildContext, err := sut.GetBuildContext(pipelineInfo, gitRepo)
			s.Require().NoError(err)
			s.Require().Len(actualBuildContext.EnvironmentsToBuild, 1)
			s.Require().Equal(envName, actualBuildContext.EnvironmentsToBuild[0].Environment)
			s.ElementsMatch(actualBuildContext.EnvironmentsToBuild[0].Components, testSpec.expectedComponents)
		})
	}
}
