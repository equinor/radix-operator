package internal_test

import (
	"errors"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/preparepipeline/internal"
	"github.com/equinor/radix-operator/pipeline-runner/utils/git"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
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
	_, err := sut.GetBuildContext(pipelineInfo, gitRepo)
	s.Require().NoError(err)
}

// func (s *stepTestSuite) Test_ComponentHasChangedSource() {
// 	const (
// 		envDev1 = "dev1"
// 		envDev2 = "dev2"
// 	)

// 	testScenarios := map[string]struct {
// 		changedFolders       []string
// 		sourceFolder         string
// 		checkChangesEnvName  string
// 		envName1SourceFolder *string
// 		envName1Enabled      *bool
// 		envName1Image        string
// 		expectedResult       bool
// 	}{
// 		"sourceFolder is dot": {
// 			changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:        ".",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      true,
// 		},
// 		"several dots and slashes": {
// 			changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:        "././",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      true,
// 		},
// 		"no changes in the sourceFolder folder with trailing slash": {
// 			changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:        "nonexistingdir/",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      false,
// 		},
// 		"no changes in the sourceFolder folder without slashes": {
// 			changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:        "nonexistingdir",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      false,
// 		},
// 		"real source dir with trailing slash": {
// 			changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:        "src/",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      true,
// 		},
// 		"sourceFolder has surrounding slashes and leading dot": {
// 			changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:        "./src/",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      true,
// 		},
// 		"real source dir without trailing slash": {
// 			changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:        "./src",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      true,
// 		},
// 		"changes in the sourceFolder folder subfolder": {
// 			changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:        "src",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      true,
// 		},
// 		"changes in the sourceFolder multiple element path subfolder": {
// 			changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:        "src/subdir",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      true,
// 		},
// 		"changes in the sourceFolder multiple element path subfolder with trailing slash": {
// 			changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:        "src/subdir/",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      true,
// 		},
// 		"no changes in the sourceFolder multiple element path subfolder": {
// 			changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:        "src/subdir/water",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      false,
// 		},
// 		"changes in the sourceFolder multiple element path subfolder with trailing slash, no match": {
// 			changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:        "src/subdir/water/",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      false,
// 		},
// 		"sourceFolder has name of changed folder": {
// 			changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:        "notebooks",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      true,
// 		},
// 		"empty sourceFolder is affected by any changes": {
// 			changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:        "",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      true,
// 		},
// 		"sourceFolder sub-folder in the root": {
// 			changedFolders:      []string{".", "app1"},
// 			sourceFolder:        "./app1",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      true,
// 		},
// 		"sourceFolder sub-folder in the root, no changed folders": {
// 			changedFolders:      []string{},
// 			sourceFolder:        ".",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      false,
// 		},
// 		"sourceFolder sub-folder in empty, no changed folders": {
// 			changedFolders:      []string{},
// 			sourceFolder:        "",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      false,
// 		},
// 		"sourceFolder sub-folder in slash, no changed folders": {
// 			changedFolders:      []string{},
// 			sourceFolder:        "/",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      false,
// 		},
// 		"sourceFolder sub-folder in slash with dot, no changed folders": {
// 			changedFolders:      []string{},
// 			sourceFolder:        "/.",
// 			checkChangesEnvName: envDev1,
// 			expectedResult:      false,
// 		},
// 		"sourceFolder is dot, env sourceFolder is empty": {
// 			changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:         ".",
// 			checkChangesEnvName:  envDev1,
// 			envName1SourceFolder: pointers.Ptr(""),
// 			expectedResult:       true,
// 		},
// 		"sourceFolder is dot, env sourceFolder has image": {
// 			changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:        ".",
// 			checkChangesEnvName: envDev1,
// 			envName1Image:       "some-image",
// 			expectedResult:      false,
// 		},
// 		"sourceFolder is dot, disabled env sourceFolder has image": {
// 			changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:        ".",
// 			checkChangesEnvName: envDev1,
// 			envName1Enabled:     pointers.Ptr(false),
// 			envName1Image:       "some-image",
// 			expectedResult:      false,
// 		},
// 		"sourceFolder is dot, enabled env sourceFolder has image": {
// 			changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:        ".",
// 			checkChangesEnvName: envDev1,
// 			envName1Enabled:     pointers.Ptr(true),
// 			envName1Image:       "some-image",
// 			expectedResult:      false,
// 		},
// 		"sourceFolder is dot, env sourceFolder has no image": {
// 			changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:        ".",
// 			checkChangesEnvName: envDev2,
// 			envName1Image:       "some-image",
// 			expectedResult:      true,
// 		},
// 		"sourceFolder is not changed, env sourceFolder is changed": {
// 			changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:         "noneexistingdir",
// 			checkChangesEnvName:  envDev1,
// 			envName1SourceFolder: pointers.Ptr("src"),
// 			expectedResult:       true,
// 		},
// 		"sourceFolder is not changed, enabled env sourceFolder is changed": {
// 			changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:         "noneexistingdir",
// 			checkChangesEnvName:  envDev1,
// 			envName1SourceFolder: pointers.Ptr("src"),
// 			envName1Enabled:      pointers.Ptr(true),
// 			expectedResult:       true,
// 		},
// 		"sourceFolder is not changed, disabled env sourceFolder is changed": {
// 			changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:         "noneexistingdir",
// 			checkChangesEnvName:  envDev1,
// 			envName1SourceFolder: pointers.Ptr("src"),
// 			envName1Enabled:      pointers.Ptr(false),
// 			expectedResult:       false,
// 		},
// 		"sourceFolder is not changed, disabled env sourceFolder is changed, checking another env": {
// 			changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:         "noneexistingdir",
// 			checkChangesEnvName:  envDev2,
// 			envName1SourceFolder: pointers.Ptr("src"),
// 			envName1Enabled:      pointers.Ptr(false),
// 			expectedResult:       false,
// 		},
// 		"sourceFolder is src, env sourceFolder is dot": {
// 			changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:         "src/",
// 			checkChangesEnvName:  envDev1,
// 			envName1SourceFolder: pointers.Ptr("."),
// 			expectedResult:       true,
// 		},
// 		"sourceFolder is dot, env sourceFolder is src": {
// 			changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:         ".",
// 			checkChangesEnvName:  envDev1,
// 			envName1SourceFolder: pointers.Ptr("src/"),
// 			expectedResult:       true,
// 		},
// 		"sourceFolder is dot, no env sourceFolder": {
// 			changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:         ".",
// 			checkChangesEnvName:  envDev2,
// 			envName1SourceFolder: pointers.Ptr("src/"),
// 			expectedResult:       true,
// 		},
// 		"no changes in the sourceFolder folder, env sourceFolder is empty": {
// 			changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:         "nonexistingdir",
// 			checkChangesEnvName:  envDev1,
// 			envName1SourceFolder: pointers.Ptr(""),
// 			expectedResult:       false,
// 		},
// 		"changes in the sourceFolder folder, env sourceFolder changed": {
// 			changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:         ".",
// 			checkChangesEnvName:  envDev2,
// 			envName1SourceFolder: pointers.Ptr("nonexistingdir"),
// 			expectedResult:       true,
// 		},
// 		"changes in the sourceFolder folder, disabled env sourceFolder not changed": {
// 			changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:         ".",
// 			checkChangesEnvName:  envDev1,
// 			envName1SourceFolder: pointers.Ptr("nonexistingdir"),
// 			envName1Enabled:      pointers.Ptr(false),
// 			expectedResult:       false,
// 		},
// 		"no changes in the sourceFolder folder, env sourceFolder is changed": {
// 			changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:         "nonexistingdir",
// 			checkChangesEnvName:  envDev1,
// 			envName1SourceFolder: pointers.Ptr("src"),
// 			expectedResult:       true,
// 		},
// 		"no changes in the sourceFolder folder, env sourceFolder is not changed": {
// 			changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:         "nonexistingdir",
// 			checkChangesEnvName:  envDev2,
// 			envName1SourceFolder: pointers.Ptr("src"),
// 			expectedResult:       false,
// 		},
// 		"changes in the sourceFolder folder subfolder, env sourceFolder is same folder": {
// 			changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:         "src",
// 			checkChangesEnvName:  envDev1,
// 			envName1SourceFolder: pointers.Ptr("src"),
// 			expectedResult:       true,
// 		},
// 		"sourceFolder is dot, env sourceFolder is disabled": {
// 			changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
// 			sourceFolder:         ".",
// 			checkChangesEnvName:  envDev1,
// 			envName1SourceFolder: pointers.Ptr(""),
// 			expectedResult:       true,
// 		},
// 	}

// 	var applicationComponent v1.RadixComponent

// 	for testName, testScenario := range testScenarios {
// 		s.Run(testName, func() {
// 			environmentConfigBuilder := utils.AnEnvironmentConfig().WithEnvironment(envDev1).WithImage(testScenario.envName1Image)
// 			if testScenario.envName1SourceFolder != nil {
// 				environmentConfigBuilder.WithSourceFolder(*testScenario.envName1SourceFolder)
// 			}
// 			if testScenario.envName1Enabled != nil {
// 				environmentConfigBuilder.WithEnabled(*testScenario.envName1Enabled)
// 			}
// 			applicationComponent =
// 				utils.AnApplicationComponent().
// 					WithName("client-component-1").
// 					WithEnvironmentConfigs(
// 						environmentConfigBuilder,
// 					).
// 					WithSourceFolder(testScenario.sourceFolder).
// 					BuildComponent()
// 			sourceHasChanged := prepareInternal.ComponentHasChangedSource(testScenario.checkChangesEnvName, &applicationComponent, testScenario.changedFolders)
// 			assert.Equal(t, testScenario.expectedResult, sourceHasChanged)
// 		})
// 	}
// }
