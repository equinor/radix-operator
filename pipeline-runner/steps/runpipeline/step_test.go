package runpipeline_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/labels"
	internalTest "github.com/equinor/radix-operator/pipeline-runner/steps/internal/test"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/wait"
	"github.com/equinor/radix-operator/pipeline-runner/steps/runpipeline"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/suite"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	tektonfake "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_RunTestSuite(t *testing.T) {
	suite.Run(t, new(stepTestSuite))
}

type stepTestSuite struct {
	suite.Suite
	kubeClient    *kubefake.Clientset
	radixClient   *radixfake.Clientset
	kedaClient    *kedafake.Clientset
	dynamicClient client.Client
	tknClient     tektonclient.Interface
}

func (s *stepTestSuite) SetupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset() // nolint:staticcheck // SA1019: Ignore linting deprecated fields
	s.dynamicClient = test.CreateClient()
	s.kedaClient = kedafake.NewSimpleClientset()
	s.tknClient = tektonfake.NewSimpleClientset() // nolint:staticcheck // SA1019: Ignore linting deprecated fields
}

func (s *stepTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *stepTestSuite) Test_RunPipeline_TaskRunTemplate() {
	mockCtrl := gomock.NewController(s.T())
	completionWaiter := wait.NewMockPipelineRunsCompletionWaiter(mockCtrl)
	completionWaiter.EXPECT().Wait(gomock.Any(), gomock.Any()).AnyTimes()
	rrBuilder := utils.NewRegistrationBuilder().WithName(internalTest.AppName)
	raBuilder := utils.NewRadixApplicationBuilder().WithRadixRegistration(rrBuilder).WithAppName(internalTest.AppName).
		WithEnvironment(internalTest.Env1, internalTest.BranchMain)
	rr := rrBuilder.BuildRR()
	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			AppName:       internalTest.AppName,
			ImageTag:      internalTest.RadixImageTag,
			JobName:       internalTest.RadixPipelineJobName,
			GitRef:        internalTest.BranchMain,
			GitRefType:    string(radixv1.GitRefBranch),
			PipelineType:  string(radixv1.BuildDeploy),
			ToEnvironment: internalTest.Env1,
		},
		RadixApplication:             raBuilder.BuildRA(),
		EnvironmentSubPipelinesToRun: []model.EnvironmentSubPipelineToRun{{Environment: "any", PipelineFile: "any"}},
	}
	ctx := context.Background()
	_, err := s.radixClient.RadixV1().RadixRegistrations().Create(ctx, rr, metav1.CreateOptions{})
	s.Require().NoError(err)
	step := runpipeline.NewRunPipelinesStep(runpipeline.WithPipelineRunsWaiter(completionWaiter))
	step.Init(context.Background(), s.kubeClient, s.radixClient, s.dynamicClient, s.tknClient, rr)

	_, err = s.tknClient.TektonV1().Pipelines(pipelineInfo.GetAppNamespace()).Create(ctx, &pipelinev1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:   internalTest.RadixPipelineJobName,
			Labels: labels.GetSubPipelineLabelsForEnvironment(pipelineInfo, internalTest.Env1, rr.Spec.AppID),
		},
		Spec: pipelinev1.PipelineSpec{},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)

	err = step.Run(ctx, pipelineInfo)
	s.Require().NoError(err)

	l, err := s.tknClient.TektonV1().PipelineRuns(pipelineInfo.GetAppNamespace()).List(ctx, metav1.ListOptions{})
	s.Require().NoError(err)
	s.Assert().NotEmpty(l.Items)

	expected := pipelinev1.PipelineTaskRunTemplate{
		ServiceAccountName: utils.GetSubPipelineServiceAccountName(internalTest.Env1),
		PodTemplate: &pod.Template{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: pointers.Ptr(true),
			},
			NodeSelector: map[string]string{
				corev1.LabelArchStable: "amd64",
				corev1.LabelOSStable:   "linux",
			},
		},
	}
	s.Assert().Equal(expected, l.Items[0].Spec.TaskRunTemplate)
	s.Assert().Equal(l.Items[0].Labels[kube.RadixAppIDLabel], rr.Spec.AppID.String(), "mismatching app ID label in PipelineRun")
}

func (s *stepTestSuite) Test_RunPipeline_RadixParam() {
	mockCtrl := gomock.NewController(s.T())
	completionWaiter := wait.NewMockPipelineRunsCompletionWaiter(mockCtrl)
	completionWaiter.EXPECT().Wait(gomock.Any(), gomock.Any()).AnyTimes()

	rrBuilder := utils.NewRegistrationBuilder().WithName(internalTest.AppName)
	raBuilder := utils.NewRadixApplicationBuilder().WithRadixRegistration(rrBuilder).WithAppName(internalTest.AppName).
		WithEnvironment(internalTest.Env1, internalTest.BranchMain).
		WithEnvironment(internalTest.Env2, internalTest.BranchMain)
	rr := rrBuilder.BuildRR()

	env1SubPipelineParam := model.SubPipelineParams{
		PipelineType: string(radixv1.BuildDeploy),
		Environment:  internalTest.Env1,
		GitSSHUrl:    "git@github.com:equinor/radix-operator.git",
		GitRef:       internalTest.BranchMain,
		GitRefType:   string(radixv1.GitRefBranch),
		GitCommit:    "commit-for-dev1",
		GitTags:      "",
	}
	env2SubPipelineParam := model.SubPipelineParams{
		PipelineType: string(radixv1.BuildDeploy),
		Environment:  internalTest.Env2,
		GitSSHUrl:    "git@github.com:equinor/radix-operator.git",
		GitRef:       "release/v1",
		GitRefType:   string(radixv1.GitRefBranch),
		GitCommit:    "commit-for-dev2",
		GitTags:      "v1",
	}

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			AppName: internalTest.AppName,
		},
		RadixApplication: raBuilder.BuildRA(),
		EnvironmentSubPipelinesToRun: []model.EnvironmentSubPipelineToRun{
			{Environment: internalTest.Env1, PipelineFile: "subpipeline-dev1.yaml"},
			{Environment: internalTest.Env2, PipelineFile: "subpipeline-dev2.yaml"},
		},
		EnvironmentSubPipelineParams: map[string]model.SubPipelineParams{
			internalTest.Env1: env1SubPipelineParam,
			internalTest.Env2: env2SubPipelineParam,
		},
	}

	ctx := context.Background()
	_, err := s.radixClient.RadixV1().RadixRegistrations().Create(ctx, rr, metav1.CreateOptions{})
	s.Require().NoError(err)

	step := runpipeline.NewRunPipelinesStep(runpipeline.WithPipelineRunsWaiter(completionWaiter))
	step.Init(context.Background(), s.kubeClient, s.radixClient, s.dynamicClient, s.tknClient, rr)

	paramSpec, err := model.SubPipelineParams{}.AsObjectParamSpec()
	s.Require().NoError(err)

	for _, targetEnv := range []string{internalTest.Env1, internalTest.Env2} {
		_, err = s.tknClient.TektonV1().Pipelines(pipelineInfo.GetAppNamespace()).Create(ctx, &pipelinev1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:   internalTest.RadixPipelineJobName + "-" + targetEnv,
				Labels: labels.GetSubPipelineLabelsForEnvironment(pipelineInfo, targetEnv, rr.Spec.AppID),
			},
			Spec: pipelinev1.PipelineSpec{
				Params: []pipelinev1.ParamSpec{paramSpec},
			},
		}, metav1.CreateOptions{})
		s.Require().NoError(err)
	}

	err = step.Run(ctx, pipelineInfo)
	s.Require().NoError(err)

	pipelineRuns, err := s.tknClient.TektonV1().PipelineRuns(pipelineInfo.GetAppNamespace()).List(ctx, metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().Len(pipelineRuns.Items, 2)

	expectedParams := map[string]pipelinev1.Param{}
	expectedEnv1Param, err := env1SubPipelineParam.AsObjectParam()
	s.Require().NoError(err)
	expectedParams[internalTest.Env1] = expectedEnv1Param

	expectedEnv2Param, err := env2SubPipelineParam.AsObjectParam()
	s.Require().NoError(err)
	expectedParams[internalTest.Env2] = expectedEnv2Param

	for _, pipelineRun := range pipelineRuns.Items {
		targetEnv := pipelineRun.Labels[kube.RadixEnvLabel]
		expectedParam, ok := expectedParams[targetEnv]
		s.Require().True(ok, "unexpected target environment on PipelineRun: %s", targetEnv)

		var actualRadixParam *pipelinev1.Param
		for i := range pipelineRun.Spec.Params {
			if pipelineRun.Spec.Params[i].Name == expectedParam.Name {
				actualRadixParam = &pipelineRun.Spec.Params[i]
				break
			}
		}

		s.Require().NotNil(actualRadixParam, "missing radix parameter in PipelineRun for environment %s", targetEnv)
		s.Assert().Equal(expectedParam, *actualRadixParam)
	}
}

func (s *stepTestSuite) Test_RunPipeline_ParamsFromBuildAndEnvironment() {
	testCases := map[string]struct {
		buildVariables             radixv1.EnvVarsMap
		buildSubPipelineVars       radixv1.EnvVarsMap
		environmentBuildVars       radixv1.EnvVarsMap
		environmentSubPipelineVars radixv1.EnvVarsMap
		pipelineParamSpecs         []pipelinev1.ParamSpec
		expectedParams             []pipelinev1.Param
	}{
		"build and environment build variables are both included": {
			buildVariables: radixv1.EnvVarsMap{
				"SHARED":     "from-build",
				"BUILD_ONLY": "build-value",
			},
			environmentBuildVars: radixv1.EnvVarsMap{
				"ENV_ONLY": "env-value",
			},
			pipelineParamSpecs: []pipelinev1.ParamSpec{
				{Name: "SHARED", Type: pipelinev1.ParamTypeString},
				{Name: "BUILD_ONLY", Type: pipelinev1.ParamTypeString},
				{Name: "ENV_ONLY", Type: pipelinev1.ParamTypeString},
			},
			expectedParams: []pipelinev1.Param{
				{Name: "SHARED", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "from-build"}},
				{Name: "BUILD_ONLY", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "build-value"}},
				{Name: "ENV_ONLY", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "env-value"}},
			},
		},
		"environment build overrides build variables": {
			buildVariables: radixv1.EnvVarsMap{
				"SHARED":     "from-build",
				"BUILD_ONLY": "build-value",
			},
			environmentBuildVars: radixv1.EnvVarsMap{
				"SHARED": "from-environment",
			},
			pipelineParamSpecs: []pipelinev1.ParamSpec{
				{Name: "SHARED", Type: pipelinev1.ParamTypeString},
				{Name: "BUILD_ONLY", Type: pipelinev1.ParamTypeString},
			},
			expectedParams: []pipelinev1.Param{
				{Name: "SHARED", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "from-environment"}},
				{Name: "BUILD_ONLY", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "build-value"}},
			},
		},
		"environment sub-pipeline variables override and preserve build sub-pipeline values": {
			buildVariables: radixv1.EnvVarsMap{
				"LEGACY": "legacy-build-variable",
			},
			buildSubPipelineVars: radixv1.EnvVarsMap{
				"SHARED":         "from-build-subpipeline",
				"BUILD_SUB_ONLY": "build-sub-only",
				"LIST":           "a,b,c",
			},
			environmentBuildVars: radixv1.EnvVarsMap{
				"SHARED": "from-environment-build",
			},
			environmentSubPipelineVars: radixv1.EnvVarsMap{
				"SHARED":       "from-environment-subpipeline",
				"ENV_SUB_ONLY": "env-sub-only",
			},
			pipelineParamSpecs: []pipelinev1.ParamSpec{
				{Name: "SHARED", Type: pipelinev1.ParamTypeString},
				{Name: "BUILD_SUB_ONLY", Type: pipelinev1.ParamTypeString},
				{Name: "ENV_SUB_ONLY", Type: pipelinev1.ParamTypeString},
				{Name: "LIST", Type: pipelinev1.ParamTypeArray},
				{Name: "LEGACY", Type: pipelinev1.ParamTypeString},
			},
			expectedParams: []pipelinev1.Param{
				{Name: "SHARED", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "from-environment-subpipeline"}},
				{Name: "BUILD_SUB_ONLY", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "build-sub-only"}},
				{Name: "ENV_SUB_ONLY", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "env-sub-only"}},
				{Name: "LIST", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeArray, ArrayVal: []string{"a", "b", "c"}}},
			},
		},
	}

	for testName, testCase := range testCases {
		s.Run(testName, func() {
			mockCtrl := gomock.NewController(s.T())
			completionWaiter := wait.NewMockPipelineRunsCompletionWaiter(mockCtrl)
			completionWaiter.EXPECT().Wait(gomock.Any(), gomock.Any()).AnyTimes()

			rrBuilder := utils.NewRegistrationBuilder().WithName(internalTest.AppName)
			raBuilder := utils.NewRadixApplicationBuilder().WithRadixRegistration(rrBuilder).WithAppName(internalTest.AppName).
				WithBuildVariables(testCase.buildVariables)
			if len(testCase.buildSubPipelineVars) > 0 {
				raBuilder.WithSubPipeline(utils.NewSubPipelineBuilder().WithEnvVars(testCase.buildSubPipelineVars))
			}

			envBuilder := utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain).WithEnvVars(testCase.environmentBuildVars)
			if len(testCase.environmentSubPipelineVars) > 0 {
				envBuilder.WithSubPipeline(utils.NewSubPipelineBuilder().WithEnvVars(testCase.environmentSubPipelineVars))
			}
			raBuilder.WithApplicationEnvironmentBuilders(envBuilder)

			rr := rrBuilder.BuildRR()
			pipelineInfo := &model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					AppName: internalTest.AppName,
				},
				RadixApplication: raBuilder.BuildRA(),
				EnvironmentSubPipelinesToRun: []model.EnvironmentSubPipelineToRun{
					{Environment: internalTest.Env1, PipelineFile: "subpipeline-dev1.yaml"},
				},
			}

			ctx := context.Background()
			_, err := s.radixClient.RadixV1().RadixRegistrations().Create(ctx, rr, metav1.CreateOptions{})
			s.Require().NoError(err)

			step := runpipeline.NewRunPipelinesStep(runpipeline.WithPipelineRunsWaiter(completionWaiter))
			step.Init(ctx, s.kubeClient, s.radixClient, s.dynamicClient, s.tknClient, rr)

			_, err = s.tknClient.TektonV1().Pipelines(pipelineInfo.GetAppNamespace()).Create(ctx, &pipelinev1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:   internalTest.RadixPipelineJobName,
					Labels: labels.GetSubPipelineLabelsForEnvironment(pipelineInfo, internalTest.Env1, rr.Spec.AppID),
				},
				Spec: pipelinev1.PipelineSpec{
					Params: testCase.pipelineParamSpecs,
				},
			}, metav1.CreateOptions{})
			s.Require().NoError(err)

			err = step.Run(ctx, pipelineInfo)
			s.Require().NoError(err)

			pipelineRuns, err := s.tknClient.TektonV1().PipelineRuns(pipelineInfo.GetAppNamespace()).List(ctx, metav1.ListOptions{})
			s.Require().NoError(err)
			s.Require().Len(pipelineRuns.Items, 1)

			var actualParams []pipelinev1.Param
			for _, param := range pipelineRuns.Items[0].Spec.Params {
				if param.Name == "radix" || param.Name == "AZURE_CLIENT_ID" {
					continue
				}
				actualParams = append(actualParams, param)
			}

			s.Assert().ElementsMatch(testCase.expectedParams, actualParams)
		})
	}
}

func (s *stepTestSuite) Test_RunPipeline_AzureIdentityClientID() {
	testScenarios := map[string]struct {
		buildIdentityClientID         string
		buildAZURECLIENTIDParam       string
		environmentIdentityClientID   string
		environmentAZURECLIENTIDParam string
		pipelineParamSpecs            []pipelinev1.ParamSpec
		expectedAZURECLIENTID         string
		expectAzureParam              bool
	}{
		"adds AZURE_CLIENT_ID from build identity when pipeline has no param spec": {
			buildIdentityClientID: "build-identity-client-id",
			expectedAZURECLIENTID: "build-identity-client-id",
			expectAzureParam:      true,
		},
		"environment identity overrides build identity": {
			buildIdentityClientID:       "build-identity-client-id",
			environmentIdentityClientID: "environment-identity-client-id",
			pipelineParamSpecs:          []pipelinev1.ParamSpec{{Name: "AZURE_CLIENT_ID", Type: pipelinev1.ParamTypeString}},
			expectedAZURECLIENTID:       "environment-identity-client-id",
			expectAzureParam:            true,
		},
		"AZURE_CLIENT_ID param in build sub-pipeline overrides build identity": {
			buildIdentityClientID:   "build-identity-client-id",
			buildAZURECLIENTIDParam: "build-param-client-id",
			expectedAZURECLIENTID:   "build-param-client-id",
			expectAzureParam:        true,
		},
		"AZURE_CLIENT_ID param in environment sub-pipeline overrides all lower priority sources": {
			buildIdentityClientID:         "build-identity-client-id",
			buildAZURECLIENTIDParam:       "build-param-client-id",
			environmentIdentityClientID:   "environment-identity-client-id",
			environmentAZURECLIENTIDParam: "environment-param-client-id",
			expectedAZURECLIENTID:         "environment-param-client-id",
			expectAzureParam:              true,
		},
		"AZURE_CLIENT_ID is not added when no client id is configured": {
			expectAzureParam: false,
		},
	}

	for testScenarioName, testScenario := range testScenarios {
		s.Run(testScenarioName, func() {
			mockCtrl := gomock.NewController(s.T())
			completionWaiter := wait.NewMockPipelineRunsCompletionWaiter(mockCtrl)
			completionWaiter.EXPECT().Wait(gomock.Any(), gomock.Any()).AnyTimes()

			rrBuilder := utils.NewRegistrationBuilder().WithName(internalTest.AppName)
			raBuilder := utils.NewRadixApplicationBuilder().WithRadixRegistration(rrBuilder).WithAppName(internalTest.AppName)

			buildSubPipelineBuilder := utils.NewSubPipelineBuilder()
			if testScenario.buildIdentityClientID != "" {
				buildSubPipelineBuilder.WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: testScenario.buildIdentityClientID}})
			}
			if testScenario.buildAZURECLIENTIDParam != "" {
				buildSubPipelineBuilder.WithEnvVars(radixv1.EnvVarsMap{"AZURE_CLIENT_ID": testScenario.buildAZURECLIENTIDParam})
			}
			raBuilder.WithSubPipeline(buildSubPipelineBuilder)

			environmentBuilder := utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain)
			environmentSubPipelineBuilder := utils.NewSubPipelineBuilder()
			if testScenario.environmentIdentityClientID != "" {
				environmentSubPipelineBuilder.WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: testScenario.environmentIdentityClientID}})
			}
			if testScenario.environmentAZURECLIENTIDParam != "" {
				environmentSubPipelineBuilder.WithEnvVars(radixv1.EnvVarsMap{"AZURE_CLIENT_ID": testScenario.environmentAZURECLIENTIDParam})
			}
			environmentBuilder.WithSubPipeline(environmentSubPipelineBuilder)
			raBuilder.WithApplicationEnvironmentBuilders(environmentBuilder)

			rr := rrBuilder.BuildRR()
			pipelineInfo := &model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					AppName: internalTest.AppName,
				},
				RadixApplication: raBuilder.BuildRA(),
				EnvironmentSubPipelinesToRun: []model.EnvironmentSubPipelineToRun{
					{Environment: internalTest.Env1, PipelineFile: "subpipeline-dev1.yaml"},
				},
			}

			ctx := context.Background()
			_, err := s.radixClient.RadixV1().RadixRegistrations().Create(ctx, rr, metav1.CreateOptions{})
			s.Require().NoError(err)

			step := runpipeline.NewRunPipelinesStep(runpipeline.WithPipelineRunsWaiter(completionWaiter))
			step.Init(ctx, s.kubeClient, s.radixClient, s.dynamicClient, s.tknClient, rr)

			_, err = s.tknClient.TektonV1().Pipelines(pipelineInfo.GetAppNamespace()).Create(ctx, &pipelinev1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:   internalTest.RadixPipelineJobName,
					Labels: labels.GetSubPipelineLabelsForEnvironment(pipelineInfo, internalTest.Env1, rr.Spec.AppID),
				},
				Spec: pipelinev1.PipelineSpec{
					Params: testScenario.pipelineParamSpecs,
				},
			}, metav1.CreateOptions{})
			s.Require().NoError(err)

			err = step.Run(ctx, pipelineInfo)
			s.Require().NoError(err)

			pipelineRuns, err := s.tknClient.TektonV1().PipelineRuns(pipelineInfo.GetAppNamespace()).List(ctx, metav1.ListOptions{})
			s.Require().NoError(err)
			s.Require().Len(pipelineRuns.Items, 1)

			var azureParam *pipelinev1.Param
			for i := range pipelineRuns.Items[0].Spec.Params {
				if pipelineRuns.Items[0].Spec.Params[i].Name == "radix" {
					continue
				}
				if pipelineRuns.Items[0].Spec.Params[i].Name == "AZURE_CLIENT_ID" {
					azureParam = &pipelineRuns.Items[0].Spec.Params[i]
				}
			}

			if testScenario.expectAzureParam {
				s.Require().NotNil(azureParam)
				s.Assert().Equal(testScenario.expectedAZURECLIENTID, azureParam.Value.StringVal)
			} else {
				s.Assert().Nil(azureParam)
			}
		})
	}
}

func (s *stepTestSuite) Test_RunPipeline_ImageParam() {
	mockCtrl := gomock.NewController(s.T())
	completionWaiter := wait.NewMockPipelineRunsCompletionWaiter(mockCtrl)
	completionWaiter.EXPECT().Wait(gomock.Any(), gomock.Any()).AnyTimes()

	rrBuilder := utils.NewRegistrationBuilder().WithName(internalTest.AppName)
	raBuilder := utils.NewRadixApplicationBuilder().WithRadixRegistration(rrBuilder).WithAppName(internalTest.AppName).
		WithEnvironment(internalTest.Env1, internalTest.BranchMain)
	rr := rrBuilder.BuildRR()

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			AppName: internalTest.AppName,
		},
		RadixApplication: raBuilder.BuildRA(),
		EnvironmentSubPipelinesToRun: []model.EnvironmentSubPipelineToRun{
			{Environment: internalTest.Env1, PipelineFile: "pipeline.yaml"},
		},
		EnvironmentSubPipelineParams: map[string]model.SubPipelineParams{
			internalTest.Env1: {Environment: internalTest.Env1},
		},
		EnvironmentSubPipelineImageParams: model.EnvironmentComponentImages{
			internalTest.Env1: {"api-server": "", "web-app": ""},
		},
		DeployEnvironmentComponentImages: pipeline.DeployEnvironmentComponentImages{
			internalTest.Env1: {
				"api-server": {ImagePath: "registry.azurecr.io/myapp-dev-api-server:abc123"},
				"web-app":    {ImagePath: "registry.azurecr.io/myapp-dev-web-app:abc123"},
			},
		},
	}

	ctx := context.Background()
	_, err := s.radixClient.RadixV1().RadixRegistrations().Create(ctx, rr, metav1.CreateOptions{})
	s.Require().NoError(err)

	step := runpipeline.NewRunPipelinesStep(runpipeline.WithPipelineRunsWaiter(completionWaiter))
	step.Init(context.Background(), s.kubeClient, s.radixClient, s.dynamicClient, s.tknClient, rr)

	_, err = s.tknClient.TektonV1().Pipelines(pipelineInfo.GetAppNamespace()).Create(ctx, &pipelinev1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:   internalTest.RadixPipelineJobName,
			Labels: labels.GetSubPipelineLabelsForEnvironment(pipelineInfo, internalTest.Env1, rr.Spec.AppID),
		},
		Spec: pipelinev1.PipelineSpec{},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)

	err = step.Run(ctx, pipelineInfo)
	s.Require().NoError(err)

	pipelineRuns, err := s.tknClient.TektonV1().PipelineRuns(pipelineInfo.GetAppNamespace()).List(ctx, metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().Len(pipelineRuns.Items, 1)

	var imageParam *pipelinev1.Param
	for i := range pipelineRuns.Items[0].Spec.Params {
		if pipelineRuns.Items[0].Spec.Params[i].Name == model.SubPipelineImageParamName {
			imageParam = &pipelineRuns.Items[0].Spec.Params[i]
			break
		}
	}

	s.Require().NotNil(imageParam, "missing radix-image parameter in PipelineRun")
	s.Assert().Equal(pipelinev1.ParamTypeObject, imageParam.Value.Type)
	s.Assert().Equal("registry.azurecr.io/myapp-dev-api-server:abc123", imageParam.Value.ObjectVal["api-server"])
	s.Assert().Equal("registry.azurecr.io/myapp-dev-web-app:abc123", imageParam.Value.ObjectVal["web-app"])
}

func (s *stepTestSuite) Test_RunPipeline_ImageParam_NotInjectedWhenNoImages() {
	mockCtrl := gomock.NewController(s.T())
	completionWaiter := wait.NewMockPipelineRunsCompletionWaiter(mockCtrl)
	completionWaiter.EXPECT().Wait(gomock.Any(), gomock.Any()).AnyTimes()

	rrBuilder := utils.NewRegistrationBuilder().WithName(internalTest.AppName)
	raBuilder := utils.NewRadixApplicationBuilder().WithRadixRegistration(rrBuilder).WithAppName(internalTest.AppName).
		WithEnvironment(internalTest.Env1, internalTest.BranchMain)
	rr := rrBuilder.BuildRR()

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			AppName: internalTest.AppName,
		},
		RadixApplication: raBuilder.BuildRA(),
		EnvironmentSubPipelinesToRun: []model.EnvironmentSubPipelineToRun{
			{Environment: internalTest.Env1, PipelineFile: "pipeline.yaml"},
		},
		EnvironmentSubPipelineParams: map[string]model.SubPipelineParams{
			internalTest.Env1: {Environment: internalTest.Env1},
		},
	}

	ctx := context.Background()
	_, err := s.radixClient.RadixV1().RadixRegistrations().Create(ctx, rr, metav1.CreateOptions{})
	s.Require().NoError(err)

	step := runpipeline.NewRunPipelinesStep(runpipeline.WithPipelineRunsWaiter(completionWaiter))
	step.Init(context.Background(), s.kubeClient, s.radixClient, s.dynamicClient, s.tknClient, rr)

	_, err = s.tknClient.TektonV1().Pipelines(pipelineInfo.GetAppNamespace()).Create(ctx, &pipelinev1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:   internalTest.RadixPipelineJobName,
			Labels: labels.GetSubPipelineLabelsForEnvironment(pipelineInfo, internalTest.Env1, rr.Spec.AppID),
		},
		Spec: pipelinev1.PipelineSpec{},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)

	err = step.Run(ctx, pipelineInfo)
	s.Require().NoError(err)

	pipelineRuns, err := s.tknClient.TektonV1().PipelineRuns(pipelineInfo.GetAppNamespace()).List(ctx, metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().Len(pipelineRuns.Items, 1)

	for _, param := range pipelineRuns.Items[0].Spec.Params {
		s.Assert().NotEqual(model.SubPipelineImageParamName, param.Name, "radix-image param should not be present when no images configured")
	}
}

func (s *stepTestSuite) Test_RunPipeline_ImageParam_Promote_AllImagesPassedOn() {
	mockCtrl := gomock.NewController(s.T())
	completionWaiter := wait.NewMockPipelineRunsCompletionWaiter(mockCtrl)
	completionWaiter.EXPECT().Wait(gomock.Any(), gomock.Any()).AnyTimes()

	rrBuilder := utils.NewRegistrationBuilder().WithName(internalTest.AppName)
	raBuilder := utils.NewRadixApplicationBuilder().WithRadixRegistration(rrBuilder).WithAppName(internalTest.AppName).
		WithEnvironment(internalTest.Env1, internalTest.BranchMain).
		WithEnvironment(internalTest.Env2, internalTest.BranchMain)
	rr := rrBuilder.BuildRR()

	// Simulate a promote pipeline: images from the source deployment are in DeployEnvironmentComponentImages for the target env
	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			AppName:      internalTest.AppName,
			PipelineType: string(radixv1.Promote),
		},
		RadixApplication: raBuilder.BuildRA(),
		EnvironmentSubPipelinesToRun: []model.EnvironmentSubPipelineToRun{
			{Environment: internalTest.Env2, PipelineFile: "pipeline.yaml"},
		},
		EnvironmentSubPipelineParams: map[string]model.SubPipelineParams{
			internalTest.Env2: {Environment: internalTest.Env2},
		},
		EnvironmentSubPipelineImageParams: model.EnvironmentComponentImages{
			internalTest.Env2: {"api-server": "", "web-app": "", "db-migrator": ""},
		},
		DeployEnvironmentComponentImages: pipeline.DeployEnvironmentComponentImages{
			internalTest.Env2: {
				"api-server":  {ImagePath: "registry.azurecr.io/myapp-dev1-api-server:promoted-tag"},
				"web-app":     {ImagePath: "registry.azurecr.io/myapp-dev1-web-app:promoted-tag"},
				"db-migrator": {ImagePath: "registry.azurecr.io/myapp-dev1-db-migrator:promoted-tag"},
			},
		},
	}

	ctx := context.Background()
	_, err := s.radixClient.RadixV1().RadixRegistrations().Create(ctx, rr, metav1.CreateOptions{})
	s.Require().NoError(err)

	step := runpipeline.NewRunPipelinesStep(runpipeline.WithPipelineRunsWaiter(completionWaiter))
	step.Init(context.Background(), s.kubeClient, s.radixClient, s.dynamicClient, s.tknClient, rr)

	_, err = s.tknClient.TektonV1().Pipelines(pipelineInfo.GetAppNamespace()).Create(ctx, &pipelinev1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:   internalTest.RadixPipelineJobName,
			Labels: labels.GetSubPipelineLabelsForEnvironment(pipelineInfo, internalTest.Env2, rr.Spec.AppID),
		},
		Spec: pipelinev1.PipelineSpec{},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)

	err = step.Run(ctx, pipelineInfo)
	s.Require().NoError(err)

	pipelineRuns, err := s.tknClient.TektonV1().PipelineRuns(pipelineInfo.GetAppNamespace()).List(ctx, metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().Len(pipelineRuns.Items, 1)

	var imageParam *pipelinev1.Param
	for i := range pipelineRuns.Items[0].Spec.Params {
		if pipelineRuns.Items[0].Spec.Params[i].Name == model.SubPipelineImageParamName {
			imageParam = &pipelineRuns.Items[0].Spec.Params[i]
			break
		}
	}

	s.Require().NotNil(imageParam, "missing radix-image parameter in PipelineRun for promote")
	s.Assert().Equal(pipelinev1.ParamTypeObject, imageParam.Value.Type)
	s.Assert().Equal("registry.azurecr.io/myapp-dev1-api-server:promoted-tag", imageParam.Value.ObjectVal["api-server"])
	s.Assert().Equal("registry.azurecr.io/myapp-dev1-web-app:promoted-tag", imageParam.Value.ObjectVal["web-app"])
	s.Assert().Equal("registry.azurecr.io/myapp-dev1-db-migrator:promoted-tag", imageParam.Value.ObjectVal["db-migrator"])
}

func (s *stepTestSuite) Test_RunPipeline_ImageParam_BuildDeploy_OnlyEnabledComponents() {
	mockCtrl := gomock.NewController(s.T())
	completionWaiter := wait.NewMockPipelineRunsCompletionWaiter(mockCtrl)
	completionWaiter.EXPECT().Wait(gomock.Any(), gomock.Any()).AnyTimes()

	rrBuilder := utils.NewRegistrationBuilder().WithName(internalTest.AppName)
	raBuilder := utils.NewRadixApplicationBuilder().WithRadixRegistration(rrBuilder).WithAppName(internalTest.AppName).
		WithEnvironment(internalTest.Env1, internalTest.BranchMain)
	rr := rrBuilder.BuildRR()

	// Simulate build-deploy: EnvironmentSubPipelineImageParams declares all components (from radixconfig),
	// but DeployEnvironmentComponentImages only has enabled ones (disabled-component is not present)
	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			AppName:      internalTest.AppName,
			PipelineType: string(radixv1.BuildDeploy),
		},
		RadixApplication: raBuilder.BuildRA(),
		EnvironmentSubPipelinesToRun: []model.EnvironmentSubPipelineToRun{
			{Environment: internalTest.Env1, PipelineFile: "pipeline.yaml"},
		},
		EnvironmentSubPipelineParams: map[string]model.SubPipelineParams{
			internalTest.Env1: {Environment: internalTest.Env1},
		},
		EnvironmentSubPipelineImageParams: model.EnvironmentComponentImages{
			internalTest.Env1: {"api-server": "", "web-app": "", "disabled-component": ""},
		},
		DeployEnvironmentComponentImages: pipeline.DeployEnvironmentComponentImages{
			internalTest.Env1: {
				// Only enabled components have entries here
				"api-server": {ImagePath: "registry.azurecr.io/myapp-dev1-api-server:abc123"},
				"web-app":    {ImagePath: "registry.azurecr.io/myapp-dev1-web-app:abc123"},
				// "disabled-component" is NOT present — it was disabled for this environment
			},
		},
	}

	ctx := context.Background()
	_, err := s.radixClient.RadixV1().RadixRegistrations().Create(ctx, rr, metav1.CreateOptions{})
	s.Require().NoError(err)

	step := runpipeline.NewRunPipelinesStep(runpipeline.WithPipelineRunsWaiter(completionWaiter))
	step.Init(context.Background(), s.kubeClient, s.radixClient, s.dynamicClient, s.tknClient, rr)

	_, err = s.tknClient.TektonV1().Pipelines(pipelineInfo.GetAppNamespace()).Create(ctx, &pipelinev1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:   internalTest.RadixPipelineJobName,
			Labels: labels.GetSubPipelineLabelsForEnvironment(pipelineInfo, internalTest.Env1, rr.Spec.AppID),
		},
		Spec: pipelinev1.PipelineSpec{},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)

	err = step.Run(ctx, pipelineInfo)
	s.Require().NoError(err)

	pipelineRuns, err := s.tknClient.TektonV1().PipelineRuns(pipelineInfo.GetAppNamespace()).List(ctx, metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().Len(pipelineRuns.Items, 1)

	var imageParam *pipelinev1.Param
	for i := range pipelineRuns.Items[0].Spec.Params {
		if pipelineRuns.Items[0].Spec.Params[i].Name == model.SubPipelineImageParamName {
			imageParam = &pipelineRuns.Items[0].Spec.Params[i]
			break
		}
	}

	s.Require().NotNil(imageParam, "missing radix-image parameter in PipelineRun")
	s.Assert().Equal(pipelinev1.ParamTypeObject, imageParam.Value.Type)
	// Enabled components have their image paths
	s.Assert().Equal("registry.azurecr.io/myapp-dev1-api-server:abc123", imageParam.Value.ObjectVal["api-server"])
	s.Assert().Equal("registry.azurecr.io/myapp-dev1-web-app:abc123", imageParam.Value.ObjectVal["web-app"])
	// Disabled component is present in the param (declared in ParamSpec) but has empty value
	s.Assert().NotContains(imageParam.Value.ObjectVal, "disabled-component")
}
