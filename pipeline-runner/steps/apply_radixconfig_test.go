package steps_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	internaltest "github.com/equinor/radix-operator/pipeline-runner/internal/test"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
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

func (s *applyConfigTestSuite) Test_Deploy_ComponentWithMissingImageTagNameShouldFail() {
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
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	applyStep := steps.NewApplyConfigStep()
	applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	err := applyStep.Run(&pipeline)
	s.ErrorContains(err, steps.ErrMissingRequiredImageTagName("deploycomp", "dev").Error())
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
	s.ErrorContains(err, steps.ErrMissingRequiredImageTagName("deployjob", "dev").Error())
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
