package steps_test

import (
	"context"
	"os"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	internaltest "github.com/equinor/radix-operator/pipeline-runner/internal/test"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const (
	sampleApp = "./testdata/radixconfig.yaml"
)

func Test_RunApplyConfigTestSuite(t *testing.T) {
	suite.Run(t, new(applyConfigTestSuite))
}

type applyConfigTestSuite struct {
	suite.Suite
	kubeClient  *kubefake.Clientset
	radixClient *radixfake.Clientset
	promClient  *prometheusfake.Clientset
	kubeUtil    *kube.Kube
}

func (s *applyConfigTestSuite) SetupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, nil)
}

func (s *applyConfigTestSuite) Test_Deploy_BuildComponentInDeployPiplineShouldFail() {
	appName := "anyapp"
	prepareConfigMapName := "preparecm"
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment("dev", "anybranch").
		WithComponents(
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("buildcomp"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("deploycomp").WithImage("any:latest"),
		).
		BuildRA()
	s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, appName, ra, nil))

	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			PipelineType:  string(radixv1.Deploy),
			ToEnvironment: "dev",
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	applyStep := steps.NewApplyConfigStep()
	applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	err := applyStep.Run(&pipeline)
	s.ErrorIs(err, steps.ErrDeployOnlyPipelineDoesNotSupportBuild)
}

func (s *applyConfigTestSuite) Test_Deploy_BuildJobInDeployPiplineShouldFail() {
	appName := "anyapp"
	prepareConfigMapName := "preparecm"
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithJobComponents(
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(pointers.Ptr[int32](9999)).WithName("buildjob"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(pointers.Ptr[int32](9999)).WithName("deployjob").WithImage("any:latest"),
		).
		WithEnvironment("dev", "anybranch").
		BuildRA()
	s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, appName, ra, nil))

	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			PipelineType:  string(radixv1.Deploy),
			ToEnvironment: "dev",
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	applyStep := steps.NewApplyConfigStep()
	applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	err := applyStep.Run(&pipeline)
	s.ErrorIs(err, steps.ErrDeployOnlyPipelineDoesNotSupportBuild)
}

func (s *applyConfigTestSuite) Test_Deploy_ComponentImageTagName() {
	appName := "anyapp"
	prepareConfigMapName := "preparecm"
	type scenario struct {
		name                 string
		componentTagName     string
		hasEnvironmentConfig bool
		environmentTagName   string
		expectedError        error
	}
	scenarios := []scenario{
		{name: "no imageTagName in a component or an environment", expectedError: steps.ErrMissingRequiredImageTagName},
		{name: "imageTagName is in a component", componentTagName: "some-component-tag"},
		{name: "imageTagName is not set in an environment", hasEnvironmentConfig: true, expectedError: steps.ErrMissingRequiredImageTagName},
		{name: "imageTagName is in an environment", hasEnvironmentConfig: true, environmentTagName: "some-env-tag"},
		{name: "imageTagName is in a component, not in an environment", componentTagName: "some-component-tag", hasEnvironmentConfig: true},
		{name: "imageTagName is in a component and in an environment", componentTagName: "some-component-tag", hasEnvironmentConfig: true, environmentTagName: "some-env-tag"},
	}
	for _, ts := range scenarios {
		s.SetupTest()
		s.T().Run(ts.name, func(t *testing.T) {
			rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
			_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})

			componentBuilder := utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("deploycomp").WithImage("any:{imageTagName}").WithImageTagName(ts.componentTagName)
			if ts.hasEnvironmentConfig || ts.environmentTagName != "" {
				componentBuilder = componentBuilder.WithEnvironmentConfig(utils.AnEnvironmentConfig().WithEnvironment("dev").WithImageTagName(ts.environmentTagName))
			}
			ra := utils.NewRadixApplicationBuilder().
				WithAppName(appName).
				WithEnvironment("dev", "anybranch").
				WithComponents(componentBuilder).BuildRA()
			s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, appName, ra, nil))

			pipeline := model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					PipelineType:  string(radixv1.Deploy),
					ToEnvironment: "dev",
				},
				RadixConfigMapName: prepareConfigMapName,
			}

			applyStep := steps.NewApplyConfigStep()
			applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
			err := applyStep.Run(&pipeline)
			if ts.expectedError == nil {
				s.NoError(err)
			} else {
				s.ErrorIs(err, steps.ErrMissingRequiredImageTagName)
			}
		})
	}
}

func (s *applyConfigTestSuite) Test_Deploy_ComponentWithImageTagNameInRAShouldSucceed() {
	appName := "anyapp"
	prepareConfigMapName := "preparecm"
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment("dev", "anybranch").
		WithComponents(
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("deploycomp").WithImage("any:{imageTagName}").
				WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment("dev").WithImageTagName("anytag")),
		).
		BuildRA()
	s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, appName, ra, nil))

	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			PipelineType:  string(radixv1.Deploy),
			ToEnvironment: "dev",
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	applyStep := steps.NewApplyConfigStep()
	applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	s.NoError(applyStep.Run(&pipeline))
}

func (s *applyConfigTestSuite) Test_Deploy_ComponentWithImageTagNameInPipelineArgShouldSucceed() {
	appName := "anyapp"
	prepareConfigMapName := "preparecm"
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment("dev", "anybranch").
		WithComponents(
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("deploycomp").WithImage("any:{imageTagName}"),
		).
		BuildRA()
	s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, appName, ra, nil))

	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			PipelineType:  string(radixv1.Deploy),
			ToEnvironment: "dev",
			ImageTagNames: map[string]string{"deploycomp": "tag"},
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	applyStep := steps.NewApplyConfigStep()
	applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	s.NoError(applyStep.Run(&pipeline))
}

func (s *applyConfigTestSuite) Test_Deploy_JobWithMissingImageTagNameShouldFail() {
	appName := "anyapp"
	prepareConfigMapName := "preparecm"
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment("dev", "anybranch").
		WithJobComponents(
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(pointers.Ptr[int32](9999)).WithName("deployjob").WithImage("any:{imageTagName}"),
		).
		BuildRA()
	s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, appName, ra, nil))

	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			PipelineType:  string(radixv1.Deploy),
			ToEnvironment: "dev",
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	applyStep := steps.NewApplyConfigStep()
	applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	err := applyStep.Run(&pipeline)
	s.ErrorIs(err, steps.ErrMissingRequiredImageTagName)
	s.ErrorContains(err, "deployjob")
	s.ErrorContains(err, "dev")
}

func (s *applyConfigTestSuite) Test_Deploy_JobWithImageTagNameInRAShouldSucceed() {
	appName := "anyapp"
	prepareConfigMapName := "preparecm"
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment("dev", "anybranch").
		WithJobComponents(
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(pointers.Ptr[int32](9999)).WithName("deployjob").WithImage("any:{imageTagName}").
				WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment("dev").WithImageTagName("anytag")),
		).
		BuildRA()
	s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, appName, ra, nil))

	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			PipelineType:  string(radixv1.Deploy),
			ToEnvironment: "dev",
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	applyStep := steps.NewApplyConfigStep()
	applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	s.NoError(applyStep.Run(&pipeline))
}

func (s *applyConfigTestSuite) Test_DeployComponentWitImageTagNameInPipelineArgShouldSucceed() {
	appName := "anyapp"
	prepareConfigMapName := "preparecm"
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment("dev", "anybranch").
		WithJobComponents(
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(pointers.Ptr[int32](9999)).WithName("deployjob").WithImage("any:{imageTagName}"),
		).
		BuildRA()
	s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, appName, ra, nil))

	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			PipelineType:  string(radixv1.Deploy),
			ToEnvironment: "dev",
			ImageTagNames: map[string]string{"deployjob": "anytag"},
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	applyStep := steps.NewApplyConfigStep()
	applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	s.NoError(applyStep.Run(&pipeline))
}

func (s *applyConfigTestSuite) Test_CreateRadixApplication_LimitMemoryIsTakenFromRequestsMemory() {
	rr := utils.NewRegistrationBuilder().WithName("testapp").BuildRR()
	_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	s.Require().NoError(err)
	configFileContent, err := os.ReadFile(sampleApp)
	s.Require().NoError(err)
	ra, err := steps.CreateRadixApplication(s.radixClient, &dnsalias.DNSConfig{}, string(configFileContent))
	s.Require().NoError(err)
	s.Equal("100Mi", ra.Spec.Components[0].Resources.Limits["memory"], "server1 invalid resource limits memory")
	s.Equal("100Mi", ra.Spec.Components[1].Resources.Limits["memory"], "server2 invalid resource limits memory")
	s.Empty(ra.Spec.Components[2].Resources.Limits["memory"], "server3 not expected resource limits memory")
	s.Empty(ra.Spec.Components[3].Resources.Limits["memory"], "server4 not expected resource limits memory")
}

func (s *applyConfigTestSuite) Test_Deploy_ComponentsToDeployValidation() {
	schedulerPort := pointers.Ptr(int32(8080))
	raBuilder := utils.ARadixApplication().
		WithEnvironment("dev", "main").
		WithComponents(
			utils.AnApplicationComponent().WithName("comp1").WithImage("some-image"),
			utils.AnApplicationComponent().WithName("comp2").WithImage("some-image"),
		).
		WithJobComponents(
			utils.AnApplicationJobComponent().WithName("job1").WithImage("some-image").WithSchedulerPort(schedulerPort),
			utils.AnApplicationJobComponent().WithName("job2").WithImage("some-image").WithSchedulerPort(schedulerPort),
		)
	const appName = "anyapp"
	rdBuilder := utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("dev").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1").WithImage("some-image"),
			utils.NewDeployComponentBuilder().WithName("comp2").WithImage("some-image")).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().WithName("job1").WithImage("some-image").WithSchedulerPort(schedulerPort),
			utils.NewDeployJobComponentBuilder().WithName("job2").WithImage("some-image").WithSchedulerPort(schedulerPort))
	activeRadixDeployment := rdBuilder.BuildRD()
	ra := raBuilder.BuildRA()
	rr := raBuilder.GetRegistrationBuilder().BuildRR()
	const prepareConfigMapName = "preparecm"

	scenarios := []struct {
		name               string
		pipelineType       radixv1.RadixPipelineType
		componentsToDeploy []string
		expectedError      string
	}{
		{name: "Deploy No componentToDeploy", pipelineType: radixv1.Deploy, componentsToDeploy: nil, expectedError: ""},
		{name: "BuildDeploy No componentToDeploy", pipelineType: radixv1.BuildDeploy, componentsToDeploy: nil, expectedError: ""},
		{name: "Promote No componentToDeploy", pipelineType: radixv1.Promote, componentsToDeploy: nil, expectedError: ""},
		{name: "Deploy ComponentToDeploy exist in RA", pipelineType: radixv1.Deploy, componentsToDeploy: []string{"comp1", "job1"}, expectedError: ""},
		{name: "BuildDeploy ComponentToDeploy exist in RA", pipelineType: radixv1.BuildDeploy, componentsToDeploy: []string{"comp1", "job1"}, expectedError: ""},
		{name: "Promote ComponentToDeploy exist in RA", pipelineType: radixv1.Promote, componentsToDeploy: []string{"comp1", "job1"}, expectedError: ""},
		{name: "Deploy ComponentToDeploy with spaces exist in RA", pipelineType: radixv1.Deploy, componentsToDeploy: []string{"comp1  ", "  job1  "}, expectedError: ""},
		{name: "Deploy All ComponentToDeploy exist in RA", pipelineType: radixv1.Deploy, componentsToDeploy: []string{"comp1", "comp2", "job1", "job2"}, expectedError: ""},
		{name: "Deploy Some ComponentToDeploy do not exist in RA", pipelineType: radixv1.Deploy, componentsToDeploy: []string{"comp1", "not-existing-comp", "job1"}, expectedError: "requested component not-existing-comp does not exist"},
		{name: "Deploy None of ComponentToDeploy exist in RA", pipelineType: radixv1.Deploy, componentsToDeploy: []string{"not-existing-comp", "not-existing-job"}, expectedError: "requested component not-existing-comp does not exist\nrequested component not-existing-job does not exist"},
		{name: "Deploy Empty line as ComponentToDeploy no error", pipelineType: radixv1.Deploy, componentsToDeploy: []string{"", "  "}, expectedError: ""},
	}

	for _, ts := range scenarios {
		s.T().Run(ts.name, func(t *testing.T) {
			s.SetupTest()
			_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
			s.Require().NoError(err)
			_, err = s.radixClient.RadixV1().RadixDeployments("anyapp-dev").Create(context.Background(), activeRadixDeployment, metav1.CreateOptions{})
			s.Require().NoError(err)
			s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, appName, ra, nil))
			for _, env := range []string{"anyapp-app", "anyapp-dev"} {
				_, err = s.kubeClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: env}}, metav1.CreateOptions{})
				s.Require().NoError(err, "create env %s", env)
			}

			pipeline := model.PipelineInfo{
				RadixConfigMapName: prepareConfigMapName,
				PipelineArguments: model.PipelineArguments{
					PipelineType:       string(ts.pipelineType),
					ComponentsToDeploy: ts.componentsToDeploy,
				},
			}
			switch ts.pipelineType {
			case radixv1.Deploy:
				pipeline.PipelineArguments.ToEnvironment = "dev"
			case radixv1.BuildDeploy:
				pipeline.PipelineArguments.Branch = "main"
			case radixv1.Promote:
				pipeline.PipelineArguments.FromEnvironment = "dev"
				pipeline.PipelineArguments.ToEnvironment = "dev"
				pipeline.PipelineArguments.DeploymentName = "depl"
			}

			applyStep := steps.NewApplyConfigStep()
			applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
			err = applyStep.Run(&pipeline)
			if len(ts.expectedError) > 0 {
				s.Assert().EqualError(err, ts.expectedError, "missing error '%s'", ts.expectedError)
			} else {
				s.Assert().NoError(err)
			}
		})
	}
}
