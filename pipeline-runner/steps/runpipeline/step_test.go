package runpipeline_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/labels"
	internalTest "github.com/equinor/radix-operator/pipeline-runner/steps/internal/test"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/wait"
	"github.com/equinor/radix-operator/pipeline-runner/steps/runpipeline"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	tektonfake "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
}

func (s *stepTestSuite) SetupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.kedaClient = kedafake.NewSimpleClientset()
	s.tknClient = tektonfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.kedaClient, nil)
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
	step.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, s.tknClient, rr)

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

func (s *stepTestSuite) Test_RunPipeline_ApplyEnvVars() {
	type scenario struct {
		name                          string
		pipelineSpec                  pipelinev1.PipelineSpec
		appEnvBuilder                 []utils.ApplicationEnvironmentBuilder
		buildVariables                radixv1.EnvVarsMap
		buildSubPipeline              utils.SubPipelineBuilder
		expectedPipelineRunParamNames map[string]string
	}

	scenarios := []scenario{
		{name: "no env vars",
			pipelineSpec:  pipelinev1.PipelineSpec{},
			appEnvBuilder: []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain)},
		},
		{name: "task uses common env vars",
			pipelineSpec: pipelinev1.PipelineSpec{
				Params: []pipelinev1.ParamSpec{
					{Name: "var1", Type: pipelinev1.ParamTypeString},
					{Name: "var3", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value3default"}},
					{Name: "var4", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value4default"}},
				},
			},
			appEnvBuilder:                 []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain)},
			buildVariables:                map[string]string{"var1": "value1common", "var2": "value2common", "var3": "value3common"},
			expectedPipelineRunParamNames: map[string]string{"var1": "value1common", "var3": "value3common", "var4": "value4default"},
		},
		{name: "task do not use common env vars because of an empty sub-pipeline",
			pipelineSpec: pipelinev1.PipelineSpec{
				Params: []pipelinev1.ParamSpec{
					{Name: "var1", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value1default"}},
					{Name: "var3", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value3default"}},
					{Name: "var4", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value4default"}},
				},
			},
			appEnvBuilder:                 []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain)},
			buildSubPipeline:              utils.NewSubPipelineBuilder(),
			buildVariables:                map[string]string{"var1": "value1common", "var2": "value2common", "var3": "value3common"},
			expectedPipelineRunParamNames: map[string]string{"var1": "value1default", "var3": "value3default", "var4": "value4default"},
		},
		{name: "task uses common env vars from sub-pipeline, ignores build variables",
			pipelineSpec: pipelinev1.PipelineSpec{
				Params: []pipelinev1.ParamSpec{
					{Name: "var1", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value1default"}},
					{Name: "var3", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value3default"}},
					{Name: "var4", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value4default"}},
					{Name: "var5", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value5default"}},
				},
			},
			buildSubPipeline:              utils.NewSubPipelineBuilder().WithEnvVars(map[string]string{"var3": "value3sp", "var4": "value4sp"}),
			buildVariables:                map[string]string{"var1": "value1common", "var2": "value2common", "var3": "value3common"},
			expectedPipelineRunParamNames: map[string]string{"var1": "value1default", "var3": "value3sp", "var4": "value4sp", "var5": "value5default"},
		},
		{name: "task uses environment env vars",
			pipelineSpec: pipelinev1.PipelineSpec{
				Params: []pipelinev1.ParamSpec{
					{Name: "var1", Type: pipelinev1.ParamTypeString},
					{Name: "var3", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value3default"}},
					{Name: "var4", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value4default"}},
					{Name: "var5", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value5default"}},
				},
			},
			appEnvBuilder: []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain).
				WithEnvVars(map[string]string{"var3": "value3env", "var4": "value4env"})},
			buildVariables:                map[string]string{"var1": "value1common", "var2": "value2common"},
			expectedPipelineRunParamNames: map[string]string{"var1": "value1common", "var3": "value3env", "var4": "value4env", "var5": "value5default"},
		},
		{name: "task uses environment env vars, not other environment env-vars",
			pipelineSpec: pipelinev1.PipelineSpec{
				Params: []pipelinev1.ParamSpec{
					{Name: "var1", Type: pipelinev1.ParamTypeString},
					{Name: "var3", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value3default"}},
					{Name: "var4", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value4default"}},
					{Name: "var5", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value5default"}},
				},
			},
			appEnvBuilder: []utils.ApplicationEnvironmentBuilder{
				utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain).
					WithEnvVars(map[string]string{"var3": "value3env", "var4": "value4env"}),
				utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env2).
					WithEnvVars(map[string]string{"var3": "value3env2", "var4": "value4env2"})},
			buildVariables:                map[string]string{"var1": "value1common", "var2": "value2common"},
			expectedPipelineRunParamNames: map[string]string{"var1": "value1common", "var3": "value3env", "var4": "value4env", "var5": "value5default"},
		},
		{name: "task uses environment sub-pipeline env vars, ignores environment build variables",
			pipelineSpec: pipelinev1.PipelineSpec{
				Params: []pipelinev1.ParamSpec{
					{Name: "var1", Type: pipelinev1.ParamTypeString},
					{Name: "var3", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value3default"}},
					{Name: "var4", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value4default"}},
					{Name: "var5", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value5default"}},
				},
			},
			appEnvBuilder: []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain).
				WithSubPipeline(utils.NewSubPipelineBuilder().WithEnvVars(map[string]string{"var3": "value3sp-env", "var4": "value4sp-env"}))},
			buildVariables:                map[string]string{"var1": "value1common", "var2": "value2common"},
			expectedPipelineRunParamNames: map[string]string{"var1": "value1common", "var3": "value3sp-env", "var4": "value4sp-env", "var5": "value5default"},
		},
		{name: "task do not use environment sub-pipeline env vars, because of empty env sub-pipeline",
			pipelineSpec: pipelinev1.PipelineSpec{
				Params: []pipelinev1.ParamSpec{
					{Name: "var1", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value1default"}},
					{Name: "var3", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value3default"}},
					{Name: "var4", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value4default"}},
					{Name: "var5", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value5default"}},
				},
			},
			appEnvBuilder: []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain).
				WithSubPipeline(utils.NewSubPipelineBuilder())},
			buildVariables:                map[string]string{"var1": "value1common", "var2": "value2common"},
			expectedPipelineRunParamNames: map[string]string{"var1": "value1common", "var3": "value3default", "var4": "value4default", "var5": "value5default"},
		},
		{name: "task uses environment sub-pipeline env vars, build sub-pipeline env vars, ignores environment build variables and build env vars",
			pipelineSpec: pipelinev1.PipelineSpec{
				Params: []pipelinev1.ParamSpec{
					{Name: "var1", Type: pipelinev1.ParamTypeString},
					{Name: "var3", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value3default"}},
					{Name: "var4", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value4default"}},
					{Name: "var5", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value5default"}},
				},
			},
			appEnvBuilder: []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain).
				WithSubPipeline(utils.NewSubPipelineBuilder().WithEnvVars(map[string]string{"var3": "value3sp-env", "var4": "value4sp-env"}))},
			buildVariables:                map[string]string{"var1": "value1common", "var2": "value2common", "var4": "value4common"},
			buildSubPipeline:              utils.NewSubPipelineBuilder().WithEnvVars(map[string]string{"var1": "value1sp", "var4": "value4sp"}),
			expectedPipelineRunParamNames: map[string]string{"var1": "value1sp", "var3": "value3sp-env", "var4": "value4sp-env", "var5": "value5default"},
		},
		{name: "task uses environment env vars, build sub-pipeline env vars, ignores environment build variables and build env vars",
			pipelineSpec: pipelinev1.PipelineSpec{
				Params: []pipelinev1.ParamSpec{
					{Name: "var1", Type: pipelinev1.ParamTypeString},
					{Name: "var3", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value3default"}},
					{Name: "var4", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value4default"}},
					{Name: "var5", Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "value5default"}},
				},
			},
			appEnvBuilder: []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain).
				WithEnvVars(map[string]string{"var3": "value3env", "var4": "value4env"})},
			buildVariables:                map[string]string{"var1": "value1common", "var2": "value2common", "var4": "value4common"},
			buildSubPipeline:              utils.NewSubPipelineBuilder().WithEnvVars(map[string]string{"var1": "value1sp", "var4": "value4sp"}),
			expectedPipelineRunParamNames: map[string]string{"var1": "value1sp", "var3": "value3env", "var4": "value4env", "var5": "value5default"},
		},
	}

	for _, ts := range scenarios {
		s.T().Run(ts.name, func(t *testing.T) {
			s.SetupTest()
			mockCtrl := gomock.NewController(t)
			completionWaiter := wait.NewMockPipelineRunsCompletionWaiter(mockCtrl)
			completionWaiter.EXPECT().Wait(gomock.Any(), gomock.Any()).AnyTimes()
			rrBuilder := utils.NewRegistrationBuilder().WithName(internalTest.AppName)
			ra := utils.NewRadixApplicationBuilder().
				WithRadixRegistration(rrBuilder).
				WithAppName(internalTest.AppName).
				WithBuildVariables(ts.buildVariables).
				WithSubPipeline(ts.buildSubPipeline).
				WithApplicationEnvironmentBuilders(ts.appEnvBuilder...).BuildRA()
			rr := rrBuilder.BuildRR()
			_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.TODO(), rr, metav1.CreateOptions{})
			s.Require().NoError(err)
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
				RadixApplication:             ra,
				TargetEnvironments:           []model.TargetEnvironment{{Environment: internalTest.Env1}},
				EnvironmentSubPipelinesToRun: []model.EnvironmentSubPipelineToRun{{Environment: "any", PipelineFile: "any"}},
			}
			step := runpipeline.NewRunPipelinesStep(runpipeline.WithPipelineRunsWaiter(completionWaiter))
			step.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, s.tknClient, rr)

			_, err = s.tknClient.TektonV1().Pipelines(pipelineInfo.GetAppNamespace()).Create(context.TODO(), &pipelinev1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{Name: internalTest.RadixPipelineJobName, Labels: labels.GetSubPipelineLabelsForEnvironment(pipelineInfo, internalTest.Env1, rr.Spec.AppID)},
				Spec:       ts.pipelineSpec}, metav1.CreateOptions{})
			s.Require().NoError(err)

			err = step.Run(context.TODO(), pipelineInfo)
			s.Require().NoError(err)

			pipelineRunList, err := s.tknClient.TektonV1().PipelineRuns(pipelineInfo.GetAppNamespace()).List(context.TODO(), metav1.ListOptions{})
			s.Require().NoError(err)
			assert.Len(t, pipelineRunList.Items, 1)
			pr := pipelineRunList.Items[0]
			assert.Len(t, pr.Spec.Params, len(ts.expectedPipelineRunParamNames), "mismatching pipelineRun.Spec.Params element count")
			for _, param := range pr.Spec.Params {
				expectedValue, ok := ts.expectedPipelineRunParamNames[param.Name]
				assert.True(t, ok, "unexpected param %s", param.Name)
				s.Assert().Equal(expectedValue, param.Value.StringVal, "mismatching value in the param %s", param.Name)
				s.Assert().Equal(pipelinev1.ParamTypeString, param.Value.Type, "mismatching type of the param %s", param.Name)
			}
		})
	}
}

func (s *stepTestSuite) Test_RunPipeline_ApplyIdentity() {
	type scenario struct {
		name                          string
		pipelineSpec                  pipelinev1.PipelineSpec
		appEnvBuilder                 []utils.ApplicationEnvironmentBuilder
		buildVariables                radixv1.EnvVarsMap
		buildIdentity                 *radixv1.Identity
		buildSubPipeline              utils.SubPipelineBuilder
		expectedPipelineRunParamNames map[string]string
	}

	scenarios := []scenario{
		{name: "no identity",
			pipelineSpec:  pipelinev1.PipelineSpec{},
			appEnvBuilder: []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain)},
		},
		{name: "task overrides param with common identity clientId",
			pipelineSpec: pipelinev1.PipelineSpec{
				Params: []pipelinev1.ParamSpec{
					{Name: defaults.AzureClientIdEnvironmentVariable, Type: pipelinev1.ParamTypeString, Default: &pipelinev1.ParamValue{StringVal: "not-set"}},
				},
			},
			appEnvBuilder:                 []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain)},
			buildIdentity:                 &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: internalTest.SomeAzureClientId}},
			expectedPipelineRunParamNames: map[string]string{defaults.AzureClientIdEnvironmentVariable: internalTest.SomeAzureClientId},
		},
		{name: "task sets param with common identity clientId",
			pipelineSpec:                  pipelinev1.PipelineSpec{},
			appEnvBuilder:                 []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain)},
			buildIdentity:                 &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: internalTest.SomeAzureClientId}},
			expectedPipelineRunParamNames: map[string]string{defaults.AzureClientIdEnvironmentVariable: internalTest.SomeAzureClientId},
		},
		{name: "task uses build environment env-var instead of common identity clientId",
			pipelineSpec: pipelinev1.PipelineSpec{},
			appEnvBuilder: []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain).
				WithEnvVars(map[string]string{defaults.AzureClientIdEnvironmentVariable: "build-env-var-client-id"})},
			buildIdentity:                 &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "build-identity-client-id"}},
			expectedPipelineRunParamNames: map[string]string{defaults.AzureClientIdEnvironmentVariable: "build-env-var-client-id"},
		},
		{name: "task uses build environment sub-pipeline env-var instead of common identity clientId",
			pipelineSpec: pipelinev1.PipelineSpec{},
			appEnvBuilder: []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain).
				WithSubPipeline(utils.NewSubPipelineBuilder().WithEnvVars(map[string]string{defaults.AzureClientIdEnvironmentVariable: "build-env-var-client-id"}))},
			buildIdentity:                 &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "build-identity-client-id"}},
			expectedPipelineRunParamNames: map[string]string{defaults.AzureClientIdEnvironmentVariable: "build-env-var-client-id"},
		},
		{name: "task uses build environment sub-pipeline env-var instead of common identity clientId",
			pipelineSpec: pipelinev1.PipelineSpec{},
			appEnvBuilder: []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain).
				WithSubPipeline(utils.NewSubPipelineBuilder().WithEnvVars(map[string]string{defaults.AzureClientIdEnvironmentVariable: "build-env-var-client-id"}))},
			buildIdentity:                 &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "build-identity-client-id"}},
			expectedPipelineRunParamNames: map[string]string{defaults.AzureClientIdEnvironmentVariable: "build-env-var-client-id"},
		},
		{name: "task uses build environment sub-pipeline env-var instead of environment sub-pipeline identity clientId",
			pipelineSpec: pipelinev1.PipelineSpec{},
			appEnvBuilder: []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain).
				WithSubPipeline(utils.NewSubPipelineBuilder().
					WithEnvVars(map[string]string{defaults.AzureClientIdEnvironmentVariable: "build-env-var-client-id"}).
					WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "build-identity-client-id"}}))},
			expectedPipelineRunParamNames: map[string]string{defaults.AzureClientIdEnvironmentVariable: "build-env-var-client-id"},
		},
		{name: "task uses build environment sub-pipeline identity clientId instead of build env-var",
			pipelineSpec: pipelinev1.PipelineSpec{},
			appEnvBuilder: []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain).
				WithSubPipeline(utils.NewSubPipelineBuilder().WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "build-identity-client-id"}}))},
			buildVariables:                map[string]string{defaults.AzureClientIdEnvironmentVariable: "build-env-var-client-id"},
			expectedPipelineRunParamNames: map[string]string{defaults.AzureClientIdEnvironmentVariable: "build-identity-client-id"},
		},
		{name: "task uses build environment sub-pipeline identity clientId instead of build sub-pipeline env-var",
			pipelineSpec: pipelinev1.PipelineSpec{},
			appEnvBuilder: []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain).
				WithSubPipeline(utils.NewSubPipelineBuilder().WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "build-identity-client-id"}}))},
			buildSubPipeline:              utils.NewSubPipelineBuilder().WithEnvVars(map[string]string{defaults.AzureClientIdEnvironmentVariable: "build-env-var-client-id"}),
			expectedPipelineRunParamNames: map[string]string{defaults.AzureClientIdEnvironmentVariable: "build-identity-client-id"},
		},
		{name: "task removed identity clientId env-var set by build env-var when build environment sub-pipeline has no identity clientId",
			pipelineSpec: pipelinev1.PipelineSpec{},
			appEnvBuilder: []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain).
				WithSubPipeline(utils.NewSubPipelineBuilder().WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: ""}}))},
			buildVariables:                map[string]string{defaults.AzureClientIdEnvironmentVariable: "build-env-var-client-id"},
			expectedPipelineRunParamNames: map[string]string{},
		},
		{name: "task uses build environment sub-pipeline identity clientId instead of build sub-pipeline env-var",
			pipelineSpec: pipelinev1.PipelineSpec{},
			appEnvBuilder: []utils.ApplicationEnvironmentBuilder{utils.NewApplicationEnvironmentBuilder().WithName(internalTest.Env1).WithBuildFrom(internalTest.BranchMain).
				WithSubPipeline(utils.NewSubPipelineBuilder().WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: ""}}))},
			buildSubPipeline:              utils.NewSubPipelineBuilder().WithEnvVars(map[string]string{defaults.AzureClientIdEnvironmentVariable: "build-env-var-client-id"}),
			expectedPipelineRunParamNames: map[string]string{},
		},
	}

	for _, ts := range scenarios {
		s.T().Run(ts.name, func(t *testing.T) {
			s.SetupTest()
			mockCtrl := gomock.NewController(t)
			completionWaiter := wait.NewMockPipelineRunsCompletionWaiter(mockCtrl)
			completionWaiter.EXPECT().Wait(gomock.Any(), gomock.Any()).AnyTimes()

			rrBuilder := utils.NewRegistrationBuilder().WithName(internalTest.AppName)
			raBuilder := utils.NewRadixApplicationBuilder().
				WithRadixRegistration(rrBuilder).
				WithAppName(internalTest.AppName).
				WithBuildVariables(ts.buildVariables).
				WithApplicationEnvironmentBuilders(ts.appEnvBuilder...)
			if ts.buildIdentity != nil {
				raBuilder = raBuilder.WithSubPipeline(utils.NewSubPipelineBuilder().WithIdentity(ts.buildIdentity))
			}
			rr := rrBuilder.BuildRR()
			_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.TODO(), rr, metav1.CreateOptions{})
			s.Require().NoError(err)

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
				TargetEnvironments:           []model.TargetEnvironment{{Environment: internalTest.Env1}},
				EnvironmentSubPipelinesToRun: []model.EnvironmentSubPipelineToRun{{Environment: "any", PipelineFile: "any"}},
			}
			step := runpipeline.NewRunPipelinesStep(runpipeline.WithPipelineRunsWaiter(completionWaiter))
			step.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, s.tknClient, rr)

			appNamespace := pipelineInfo.GetAppNamespace()
			_, err = s.tknClient.TektonV1().Pipelines(appNamespace).Create(context.TODO(), &pipelinev1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{Name: internalTest.RadixPipelineJobName, Namespace: appNamespace,
					Labels: labels.GetSubPipelineLabelsForEnvironment(pipelineInfo, internalTest.Env1, rr.Spec.AppID)},
				Spec: ts.pipelineSpec}, metav1.CreateOptions{})
			s.Require().NoError(err)

			err = step.Run(context.Background(), pipelineInfo)
			s.Require().NoError(err)

			pipelineRunList, err := s.tknClient.TektonV1().PipelineRuns(appNamespace).List(context.TODO(), metav1.ListOptions{})
			s.Require().NoError(err)
			assert.Len(t, pipelineRunList.Items, 1, "mismatching pipelineRun count")
			pr := pipelineRunList.Items[0]
			assert.Len(t, pr.Spec.Params, len(ts.expectedPipelineRunParamNames), "mismatching pipelineRun.Spec.Params element count")
			for _, param := range pr.Spec.Params {
				expectedValue, ok := ts.expectedPipelineRunParamNames[param.Name]
				assert.True(t, ok, "unexpected param %s", param.Name)
				s.Assert().Equal(expectedValue, param.Value.StringVal, "mismatching value in the param %s", param.Name)
				s.Assert().Equal(pipelinev1.ParamTypeString, param.Value.Type, "mismatching type of the param %s", param.Name)
			}
		})
	}
}
