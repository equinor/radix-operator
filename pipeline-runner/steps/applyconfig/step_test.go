package applyconfig_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	internaltest "github.com/equinor/radix-operator/pipeline-runner/internal/test"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/applyconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
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
	kedaClient  *kedafake.Clientset
	promClient  *prometheusfake.Clientset
	kubeUtil    *kube.Kube
}

func (s *applyConfigTestSuite) SetupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.kedaClient = kedafake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.kedaClient, nil)
}

func (s *applyConfigTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *applyConfigTestSuite) Test_RadixConfigMap_Missing() {
	appName := "anyapp"
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			AppName: appName,
		},
		RadixConfigMapName: "anyconfigmap",
	}
	cli := applyconfig.NewApplyConfigStep()
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	err := cli.Run(context.Background(), pipelineInfo)
	s.True(k8sErrors.IsNotFound(err))
}

func (s *applyConfigTestSuite) Test_RadixConfigMap_WithPrepareBuildCtx_Processed() {
	appName, radixConfigMapName := "anyapp", "preparecm"
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	expectedRa := utils.ARadixApplication().WithAppName(appName).BuildRA()
	expectedPrepareBuildCtx := &model.PrepareBuildContext{
		EnvironmentsToBuild:          []model.EnvironmentToBuild{{Environment: "any", Components: []string{"comp1", "comp2"}}},
		ChangedRadixConfig:           true,
		EnvironmentSubPipelinesToRun: []model.EnvironmentSubPipelineToRun{{Environment: "any", PipelineFile: "file1"}},
	}
	s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, radixConfigMapName, appName, expectedRa, expectedPrepareBuildCtx))
	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			AppName: appName,
		},
		RadixConfigMapName: radixConfigMapName,
	}
	cli := applyconfig.NewApplyConfigStep()
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	err := cli.Run(context.Background(), pipelineInfo)
	s.Require().NoError(err)
	s.Equal(expectedPrepareBuildCtx, pipelineInfo.PrepareBuildContext)
	// We need marshal expected and actual to JSON and compare, since Equal asserts an empty array is different for a nil array
	expectedRaJson, _ := json.Marshal(expectedRa)
	pipelineRaJson, _ := json.Marshal(pipelineInfo.RadixApplication)
	s.Equal(expectedRaJson, pipelineRaJson)
	actualRa, err := s.radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Get(context.Background(), appName, metav1.GetOptions{})
	s.Require().NoError(err)
	actualRaJson, _ := json.Marshal(actualRa)
	s.Equal(expectedRaJson, actualRaJson)
}

func (s *applyConfigTestSuite) Test_RadixConfigMap_WithoutPrepareBuildCtx_Processed() {
	appName, radixConfigMapName := "anyapp", "preparecm"
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	expectedRa := utils.ARadixApplication().WithAppName(appName).BuildRA()
	s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, radixConfigMapName, appName, expectedRa, nil))
	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			AppName: appName,
		},
		RadixConfigMapName: radixConfigMapName,
	}
	cli := applyconfig.NewApplyConfigStep()
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	err := cli.Run(context.Background(), pipelineInfo)
	s.Require().NoError(err)
	s.Nil(pipelineInfo.PrepareBuildContext)
}

func (s *applyConfigTestSuite) Test_TargetEnvironments_BranchIsNotMapped() {
	const (
		anyAppName           = "any-app"
		mappedBranch         = "master"
		nonMappedBranch      = "feature"
		prepareConfigMapName = "preparecm"
	)

	rr := utils.ARadixRegistration().WithName(anyAppName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("anyenv", mappedBranch).
		WithComponents(
			utils.AnApplicationComponent().
				WithName("anyname")).
		BuildRA()
	s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, anyAppName, ra, nil))

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			AppName:      anyAppName,
			PipelineType: string(radixv1.BuildDeploy),
			Branch:       nonMappedBranch,
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	cli := applyconfig.NewApplyConfigStep()
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	err := cli.Run(context.Background(), pipelineInfo)
	s.Require().NoError(err)
	s.Empty(pipelineInfo.TargetEnvironments)
}

func (s *applyConfigTestSuite) Test_TargetEnvironments_BranchIsMapped() {
	const (
		anyAppName           = "any-app"
		mappedBranch         = "master"
		nonMappedBranch      = "release"
		prepareConfigMapName = "preparecm"
	)

	rr := utils.ARadixRegistration().WithName(anyAppName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})

	ra := utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("mappedenv1", mappedBranch).
		WithEnvironment("mappedenv2", mappedBranch).
		WithEnvironment("nonmappedenv", nonMappedBranch).
		WithComponents(
			utils.AnApplicationComponent().
				WithName("anyname")).
		BuildRA()
	s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, anyAppName, ra, nil))

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			AppName:      anyAppName,
			PipelineType: string(radixv1.BuildDeploy),
			Branch:       mappedBranch,
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	cli := applyconfig.NewApplyConfigStep()
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	err := cli.Run(context.Background(), pipelineInfo)
	s.Require().NoError(err)
	s.ElementsMatch([]string{"mappedenv1", "mappedenv2"}, pipelineInfo.TargetEnvironments)
}

func (s *applyConfigTestSuite) Test_TargetEnvironments_DeployOnly() {
	const (
		anyAppName           = "any-app"
		prepareConfigMapName = "preparecm"
		toEnvironment        = "anyenv"
	)

	rr := utils.ARadixRegistration().WithName(anyAppName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().WithAppName(anyAppName).BuildRA()
	s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, anyAppName, ra, nil))

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			AppName:       anyAppName,
			PipelineType:  string(radixv1.Deploy),
			ToEnvironment: toEnvironment,
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	cli := applyconfig.NewApplyConfigStep()
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	err := cli.Run(context.Background(), pipelineInfo)
	s.Require().NoError(err)
	s.ElementsMatch([]string{toEnvironment}, pipelineInfo.TargetEnvironments)
}

func (s *applyConfigTestSuite) Test_BuildSecrets_SecretMissing() {
	const (
		anyAppName           = "any-app"
		mappedBranch         = "master"
		nonMappedBranch      = "release"
		prepareConfigMapName = "preparecm"
	)

	rr := utils.ARadixRegistration().WithName(anyAppName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})

	ra := utils.NewRadixApplicationBuilder().WithAppName(anyAppName).WithBuildSecrets("secret1", "secret2").BuildRA()
	s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, anyAppName, ra, nil))

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			AppName:      anyAppName,
			PipelineType: string(radixv1.BuildDeploy),
			Branch:       mappedBranch,
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	cli := applyconfig.NewApplyConfigStep()
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	err := cli.Run(context.Background(), pipelineInfo)
	s.Require().NoError(err)
	s.Empty(pipelineInfo.BuildSecret)
}

func (s *applyConfigTestSuite) Test_BuildSecrets_SecretExist() {
	const (
		anyAppName           = "any-app"
		mappedBranch         = "master"
		nonMappedBranch      = "release"
		prepareConfigMapName = "preparecm"
	)

	rr := utils.ARadixRegistration().WithName(anyAppName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().WithAppName(anyAppName).WithBuildSecrets("secret1", "secret2").BuildRA()
	s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, anyAppName, ra, nil))
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: defaults.BuildSecretsName, Namespace: utils.GetAppNamespace(anyAppName)},
		Data:       map[string][]byte{"any": []byte("data")},
	}
	secret, _ = s.kubeClient.CoreV1().Secrets(utils.GetAppNamespace(anyAppName)).Create(context.Background(), secret, metav1.CreateOptions{})

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			AppName:      anyAppName,
			PipelineType: string(radixv1.BuildDeploy),
			Branch:       mappedBranch,
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	cli := applyconfig.NewApplyConfigStep()
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	err := cli.Run(context.Background(), pipelineInfo)
	s.Require().NoError(err)
	s.Equal(secret, pipelineInfo.BuildSecret)
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

	applyStep := applyconfig.NewApplyConfigStep()
	applyStep.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	err := applyStep.Run(context.Background(), &pipeline)
	s.ErrorIs(err, applyconfig.ErrDeployOnlyPipelineDoesNotSupportBuild)
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

	applyStep := applyconfig.NewApplyConfigStep()
	applyStep.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	err := applyStep.Run(context.Background(), &pipeline)
	s.ErrorIs(err, applyconfig.ErrDeployOnlyPipelineDoesNotSupportBuild)
}

func (s *applyConfigTestSuite) Test_BuildAndDeployComponentImages() {
	appName, envName1, envName2, envName3, envName4, rjName, buildBranch, jobPort := "anyapp", "dev1", "dev2", "dev3", "dev4", "anyrj", "anybranch", pointers.Ptr[int32](9999)
	prepareConfigMapName := "preparecm"

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment(envName1, buildBranch).
		WithEnvironment(envName2, buildBranch).
		WithEnvironment(envName3, buildBranch).
		WithEnvironment(envName4, buildBranch).
		WithComponents(
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("component-1").WithSourceFolder("./client/").WithDockerfileName("client.Dockerfile").
				WithEnvironmentConfigs(
					utils.NewComponentEnvironmentBuilder().WithEnvironment(envName1),
					utils.NewComponentEnvironmentBuilder().WithEnvironment(envName2).WithSourceFolder("./client2/"),
					utils.NewComponentEnvironmentBuilder().WithEnvironment(envName3).WithDockerfileName("client2.Dockerfile"),
					utils.NewComponentEnvironmentBuilder().WithEnvironment(envName4).WithImage("some-image2:some-tag"),
				),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("component-2").WithDockerfileName("client.Dockerfile").
				WithEnvironmentConfigs(
					utils.NewComponentEnvironmentBuilder().WithEnvironment(envName1),
					utils.NewComponentEnvironmentBuilder().WithEnvironment(envName2).WithSourceFolder("./client2/"),
					utils.NewComponentEnvironmentBuilder().WithEnvironment(envName3).WithDockerfileName("client2.Dockerfile"),
					utils.NewComponentEnvironmentBuilder().WithEnvironment(envName4).WithImage("some-image3:some-tag"),
				),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("component-3").WithSourceFolder("./client/").
				WithEnvironmentConfigs(
					utils.NewComponentEnvironmentBuilder().WithEnvironment(envName1),
					utils.NewComponentEnvironmentBuilder().WithEnvironment(envName2).WithSourceFolder("./client2/"),
					utils.NewComponentEnvironmentBuilder().WithEnvironment(envName3).WithDockerfileName("client2.Dockerfile"),
					utils.NewComponentEnvironmentBuilder().WithEnvironment(envName4).WithImage("some-image4:some-tag"),
				),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("component-4").WithImage("some-image1:some-tag").
				WithEnvironmentConfigs(
					utils.NewComponentEnvironmentBuilder().WithEnvironment(envName1),
					utils.NewComponentEnvironmentBuilder().WithEnvironment(envName2).WithSourceFolder("./client2/"),
					utils.NewComponentEnvironmentBuilder().WithEnvironment(envName3).WithDockerfileName("client2.Dockerfile"),
					utils.NewComponentEnvironmentBuilder().WithEnvironment(envName4).WithImage("some-image5:some-tag"),
				),
		).
		WithJobComponents(
			utils.NewApplicationJobComponentBuilder().WithPort("any", 8080).WithName("job-1").WithSourceFolder("./client/").WithDockerfileName("client.Dockerfile").WithSchedulerPort(jobPort).
				WithEnvironmentConfigs(
					utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName1),
					utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName2).WithSourceFolder("./client2/"),
					utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName3).WithDockerfileName("client2.Dockerfile"),
					utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName4).WithImage("some-image2:some-tag"),
				),
			utils.NewApplicationJobComponentBuilder().WithPort("any", 8080).WithName("job-2").WithDockerfileName("client.Dockerfile").WithSchedulerPort(jobPort).
				WithEnvironmentConfigs(
					utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName1),
					utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName2).WithSourceFolder("./client2/"),
					utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName3).WithDockerfileName("client2.Dockerfile"),
					utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName4).WithImage("some-image3:some-tag"),
				),
			utils.NewApplicationJobComponentBuilder().WithPort("any", 8080).WithName("job-3").WithSourceFolder("./client/").WithSchedulerPort(jobPort).
				WithEnvironmentConfigs(
					utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName1),
					utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName2).WithSourceFolder("./client2/"),
					utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName3).WithDockerfileName("client2.Dockerfile"),
					utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName4).WithImage("some-image4:some-tag"),
				),
			utils.NewApplicationJobComponentBuilder().WithPort("any", 8080).WithName("job-4").WithImage("some-image1:some-tag").WithSchedulerPort(jobPort).
				WithEnvironmentConfigs(
					utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName1),
					utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName2).WithSourceFolder("./client2/"),
					utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName3).WithDockerfileName("client2.Dockerfile"),
					utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName4).WithImage("some-image5:some-tag"),
				),
		).
		BuildRA()
	s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, appName, ra, nil))
	pipelineInfo := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			PipelineType:      string(radixv1.BuildDeploy),
			Branch:            buildBranch,
			JobName:           rjName,
			ImageTag:          "imgtag",
			ContainerRegistry: "registry",
			Clustertype:       "clustertype",
			Clustername:       "clustername",
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	applyStep := applyconfig.NewApplyConfigStep()
	applyStep.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	s.Require().NoError(applyStep.Run(context.Background(), &pipelineInfo))
	imageNameFunc := func(env, comp string) string {
		return fmt.Sprintf("%s-%s", env, comp)
	}
	imagePathFunc := func(env, comp string) string {
		return fmt.Sprintf("%s/%s-%s:%s", pipelineInfo.PipelineArguments.ContainerRegistry, appName, imageNameFunc(env, comp), pipelineInfo.PipelineArguments.ImageTag)
	}
	imagePathClusterTypeFunc := func(env, comp string) string {
		return fmt.Sprintf("%s/%s-%s:%s-%s", pipelineInfo.PipelineArguments.ContainerRegistry, appName, imageNameFunc(env, comp), pipelineInfo.PipelineArguments.Clustertype, pipelineInfo.PipelineArguments.ImageTag)
	}
	imagePathClusterNameFunc := func(env, comp string) string {
		return fmt.Sprintf("%s/%s-%s:%s-%s", pipelineInfo.PipelineArguments.ContainerRegistry, appName, imageNameFunc(env, comp), pipelineInfo.PipelineArguments.Clustername, pipelineInfo.PipelineArguments.ImageTag)
	}
	buildComponentImageFunc := func(env, component, context, dockerfile string) pipeline.BuildComponentImage {
		return pipeline.BuildComponentImage{
			ComponentName:        component,
			EnvName:              env,
			ContainerName:        fmt.Sprintf("build-%s-%s", component, env),
			Context:              context,
			Dockerfile:           dockerfile,
			ImageName:            imageNameFunc(env, component),
			ImagePath:            imagePathFunc(env, component),
			ClusterTypeImagePath: imagePathClusterTypeFunc(env, component),
			ClusterNameImagePath: imagePathClusterNameFunc(env, component),
		}
	}
	expectedBuildComponentImages := pipeline.EnvironmentBuildComponentImages{
		envName1: []pipeline.BuildComponentImage{
			buildComponentImageFunc(envName1, "component-1", "/workspace/client/", "client.Dockerfile"),
			buildComponentImageFunc(envName1, "component-2", "/workspace/", "client.Dockerfile"),
			buildComponentImageFunc(envName1, "component-3", "/workspace/client/", "Dockerfile"),
			buildComponentImageFunc(envName1, "job-1", "/workspace/client/", "client.Dockerfile"),
			buildComponentImageFunc(envName1, "job-2", "/workspace/", "client.Dockerfile"),
			buildComponentImageFunc(envName1, "job-3", "/workspace/client/", "Dockerfile"),
		},
		envName2: []pipeline.BuildComponentImage{
			buildComponentImageFunc(envName2, "component-1", "/workspace/client2/", "client.Dockerfile"),
			buildComponentImageFunc(envName2, "component-2", "/workspace/client2/", "client.Dockerfile"),
			buildComponentImageFunc(envName2, "component-3", "/workspace/client2/", "Dockerfile"),
			buildComponentImageFunc(envName2, "component-4", "/workspace/client2/", "Dockerfile"),
			buildComponentImageFunc(envName2, "job-1", "/workspace/client2/", "client.Dockerfile"),
			buildComponentImageFunc(envName2, "job-2", "/workspace/client2/", "client.Dockerfile"),
			buildComponentImageFunc(envName2, "job-3", "/workspace/client2/", "Dockerfile"),
			buildComponentImageFunc(envName2, "job-4", "/workspace/client2/", "Dockerfile"),
		},
		envName3: []pipeline.BuildComponentImage{
			buildComponentImageFunc(envName3, "component-1", "/workspace/client/", "client2.Dockerfile"),
			buildComponentImageFunc(envName3, "component-2", "/workspace/", "client2.Dockerfile"),
			buildComponentImageFunc(envName3, "component-3", "/workspace/client/", "client2.Dockerfile"),
			buildComponentImageFunc(envName3, "component-4", "/workspace/", "client2.Dockerfile"),
			buildComponentImageFunc(envName3, "job-1", "/workspace/client/", "client2.Dockerfile"),
			buildComponentImageFunc(envName3, "job-2", "/workspace/", "client2.Dockerfile"),
			buildComponentImageFunc(envName3, "job-3", "/workspace/client/", "client2.Dockerfile"),
			buildComponentImageFunc(envName3, "job-4", "/workspace/", "client2.Dockerfile"),
		},
	}

	s.ElementsMatch(maps.Keys(expectedBuildComponentImages), maps.Keys(pipelineInfo.BuildComponentImages))

	for env, images := range pipelineInfo.BuildComponentImages {
		s.ElementsMatch(expectedBuildComponentImages[env], images)
	}

	expectedDeployEnvironmentComponentImages := pipeline.DeployEnvironmentComponentImages{
		envName1: pipeline.DeployComponentImages{
			"component-1": pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName1, "component-1"), Build: true},
			"component-2": pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName1, "component-2"), Build: true},
			"component-3": pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName1, "component-3"), Build: true},
			"component-4": pipeline.DeployComponentImage{ImagePath: "some-image1:some-tag", Build: false},
			"job-1":       pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName1, "job-1"), Build: true},
			"job-2":       pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName1, "job-2"), Build: true},
			"job-3":       pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName1, "job-3"), Build: true},
			"job-4":       pipeline.DeployComponentImage{ImagePath: "some-image1:some-tag", Build: false},
		},
		envName2: pipeline.DeployComponentImages{
			"component-1": pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName2, "component-1"), Build: true},
			"component-2": pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName2, "component-2"), Build: true},
			"component-3": pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName2, "component-3"), Build: true},
			"component-4": pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName2, "component-4"), Build: true},
			"job-1":       pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName2, "job-1"), Build: true},
			"job-2":       pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName2, "job-2"), Build: true},
			"job-3":       pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName2, "job-3"), Build: true},
			"job-4":       pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName2, "job-4"), Build: true},
		},
		envName3: pipeline.DeployComponentImages{
			"component-1": pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName3, "component-1"), Build: true},
			"component-2": pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName3, "component-2"), Build: true},
			"component-3": pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName3, "component-3"), Build: true},
			"component-4": pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName3, "component-4"), Build: true},
			"job-1":       pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName3, "job-1"), Build: true},
			"job-2":       pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName3, "job-2"), Build: true},
			"job-3":       pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName3, "job-3"), Build: true},
			"job-4":       pipeline.DeployComponentImage{ImagePath: imagePathFunc(envName3, "job-4"), Build: true},
		},
		envName4: pipeline.DeployComponentImages{
			"component-1": pipeline.DeployComponentImage{ImagePath: "some-image2:some-tag", Build: false},
			"component-2": pipeline.DeployComponentImage{ImagePath: "some-image3:some-tag", Build: false},
			"component-3": pipeline.DeployComponentImage{ImagePath: "some-image4:some-tag", Build: false},
			"component-4": pipeline.DeployComponentImage{ImagePath: "some-image5:some-tag", Build: false},
			"job-1":       pipeline.DeployComponentImage{ImagePath: "some-image2:some-tag", Build: false},
			"job-2":       pipeline.DeployComponentImage{ImagePath: "some-image3:some-tag", Build: false},
			"job-3":       pipeline.DeployComponentImage{ImagePath: "some-image4:some-tag", Build: false},
			"job-4":       pipeline.DeployComponentImage{ImagePath: "some-image5:some-tag", Build: false},
		},
	}
	s.Equal(expectedDeployEnvironmentComponentImages, pipelineInfo.DeployEnvironmentComponentImages)
}

func (s *applyConfigTestSuite) Test_BuildAndDeployComponentImages_ExpectedRuntime() {
	appName, envName, buildBranch, jobPort := "anyapp", "dev", "anybranch", pointers.Ptr[int32](9999)
	prepareConfigMapName := "preparecm"

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithBuildKit(pointers.Ptr(true)).
		WithEnvironment(envName, buildBranch).
		WithEnvironmentNoBranch("otherenv").
		WithComponents(
			utils.NewApplicationComponentBuilder().WithName("comp1-build"),
			utils.NewApplicationComponentBuilder().WithName("comp2-build").WithRuntime(&radixv1.Runtime{Architecture: ""}),
			utils.NewApplicationComponentBuilder().WithName("comp3-build").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}),
			utils.NewApplicationComponentBuilder().WithName("comp4-build").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}),
			utils.NewApplicationComponentBuilder().WithName("comp5-build").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}).
				WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment(envName).WithRuntime(&radixv1.Runtime{})),
			utils.NewApplicationComponentBuilder().WithName("comp6-build").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}).
				WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment(envName).WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64})),
			utils.NewApplicationComponentBuilder().WithName("comp7-build").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}).
				WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment("otherenv").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64})),
			utils.NewApplicationComponentBuilder().WithName("comp1-deploy").WithImage("any"),
			utils.NewApplicationComponentBuilder().WithName("comp2-deploy").WithImage("any").WithRuntime(&radixv1.Runtime{Architecture: ""}),
			utils.NewApplicationComponentBuilder().WithName("comp3-deploy").WithImage("any").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}),
			utils.NewApplicationComponentBuilder().WithName("comp4-deploy").WithImage("any").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}),
			utils.NewApplicationComponentBuilder().WithName("comp5-deploy").WithImage("any").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}).
				WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment(envName).WithRuntime(&radixv1.Runtime{})),
			utils.NewApplicationComponentBuilder().WithName("comp6-deploy").WithImage("any").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}).
				WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment(envName).WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64})),
			utils.NewApplicationComponentBuilder().WithName("comp7-deploy").WithImage("any").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}).
				WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment("otherenv").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64})),
		).
		WithJobComponents(
			utils.NewApplicationJobComponentBuilder().WithName("job1-build").WithSchedulerPort(jobPort),
			utils.NewApplicationJobComponentBuilder().WithName("job2-build").WithRuntime(&radixv1.Runtime{Architecture: ""}).WithSchedulerPort(jobPort),
			utils.NewApplicationJobComponentBuilder().WithName("job3-build").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}).WithSchedulerPort(jobPort),
			utils.NewApplicationJobComponentBuilder().WithName("job4-build").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}).WithSchedulerPort(jobPort),
			utils.NewApplicationJobComponentBuilder().WithName("job5-build").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}).WithSchedulerPort(jobPort).
				WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName).WithRuntime(&radixv1.Runtime{})),
			utils.NewApplicationJobComponentBuilder().WithName("job6-build").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}).WithSchedulerPort(jobPort).
				WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName).WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64})),
			utils.NewApplicationJobComponentBuilder().WithName("job7-build").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}).WithSchedulerPort(jobPort).
				WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment("otherenv").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64})),
			utils.NewApplicationJobComponentBuilder().WithName("job1-deploy").WithImage("any").WithSchedulerPort(jobPort),
			utils.NewApplicationJobComponentBuilder().WithName("job2-deploy").WithImage("any").WithRuntime(&radixv1.Runtime{Architecture: ""}).WithSchedulerPort(jobPort),
			utils.NewApplicationJobComponentBuilder().WithName("job3-deploy").WithImage("any").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}).WithSchedulerPort(jobPort),
			utils.NewApplicationJobComponentBuilder().WithName("job4-deploy").WithImage("any").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}).WithSchedulerPort(jobPort),
			utils.NewApplicationJobComponentBuilder().WithName("job5-deploy").WithImage("any").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}).WithSchedulerPort(jobPort).
				WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName).WithRuntime(&radixv1.Runtime{})),
			utils.NewApplicationJobComponentBuilder().WithName("job6-deploy").WithImage("any").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}).WithSchedulerPort(jobPort).
				WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName).WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64})),
			utils.NewApplicationJobComponentBuilder().WithName("job7-deploy").WithImage("any").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}).WithSchedulerPort(jobPort).
				WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment("otherenv").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64})),
		).
		BuildRA()
	s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, appName, ra, nil))
	pipelineInfo := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			PipelineType:      string(radixv1.BuildDeploy),
			Branch:            buildBranch,
			ImageTag:          "anytag",
			ContainerRegistry: "anyregistry",
			Clustertype:       "anyclustertype",
			Clustername:       "anyclustername",
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	applyStep := applyconfig.NewApplyConfigStep()
	applyStep.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	s.Require().NoError(applyStep.Run(context.Background(), &pipelineInfo))
	imageNameFunc := func(comp string) string {
		return fmt.Sprintf("%s-%s", envName, comp)
	}
	imagePathFunc := func(comp string) string {
		return fmt.Sprintf("%s/%s-%s:%s", pipelineInfo.PipelineArguments.ContainerRegistry, appName, imageNameFunc(comp), pipelineInfo.PipelineArguments.ImageTag)
	}
	imagePathClusterTypeFunc := func(comp string) string {
		return fmt.Sprintf("%s/%s-%s:%s-%s", pipelineInfo.PipelineArguments.ContainerRegistry, appName, imageNameFunc(comp), pipelineInfo.PipelineArguments.Clustertype, pipelineInfo.PipelineArguments.ImageTag)
	}
	imagePathClusterNameFunc := func(comp string) string {
		return fmt.Sprintf("%s/%s-%s:%s-%s", pipelineInfo.PipelineArguments.ContainerRegistry, appName, imageNameFunc(comp), pipelineInfo.PipelineArguments.Clustername, pipelineInfo.PipelineArguments.ImageTag)
	}
	buildComponentImageFunc := func(component string, runtime *radixv1.Runtime) pipeline.BuildComponentImage {
		return pipeline.BuildComponentImage{
			ComponentName:        component,
			EnvName:              envName,
			ContainerName:        fmt.Sprintf("build-%s-%s", component, envName),
			Context:              "/workspace/",
			Dockerfile:           "Dockerfile",
			ImageName:            imageNameFunc(component),
			ImagePath:            imagePathFunc(component),
			ClusterTypeImagePath: imagePathClusterTypeFunc(component),
			ClusterNameImagePath: imagePathClusterNameFunc(component),
			Runtime:              runtime,
		}
	}
	expectedBuildComponentImages := pipeline.EnvironmentBuildComponentImages{
		"dev": []pipeline.BuildComponentImage{
			buildComponentImageFunc("comp1-build", nil),
			buildComponentImageFunc("comp2-build", nil),
			buildComponentImageFunc("comp3-build", &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}),
			buildComponentImageFunc("comp4-build", &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}),
			buildComponentImageFunc("comp5-build", &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}),
			buildComponentImageFunc("comp6-build", &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}),
			buildComponentImageFunc("comp7-build", &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}),
			buildComponentImageFunc("job1-build", nil),
			buildComponentImageFunc("job2-build", nil),
			buildComponentImageFunc("job3-build", &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}),
			buildComponentImageFunc("job4-build", &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}),
			buildComponentImageFunc("job5-build", &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}),
			buildComponentImageFunc("job6-build", &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}),
			buildComponentImageFunc("job7-build", &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}),
		},
	}
	s.ElementsMatch(maps.Keys(expectedBuildComponentImages), maps.Keys(pipelineInfo.BuildComponentImages))
	for env, images := range expectedBuildComponentImages {
		s.ElementsMatch(images, pipelineInfo.BuildComponentImages[env])
	}

	expectedDeployEnvironmentComponentImages := pipeline.DeployEnvironmentComponentImages{
		"dev": pipeline.DeployComponentImages{
			"comp1-build":  pipeline.DeployComponentImage{ImagePath: imagePathFunc("comp1-build"), ImageTagName: "", Runtime: nil, Build: true},
			"comp2-build":  pipeline.DeployComponentImage{ImagePath: imagePathFunc("comp2-build"), ImageTagName: "", Runtime: nil, Build: true},
			"comp3-build":  pipeline.DeployComponentImage{ImagePath: imagePathFunc("comp3-build"), ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}, Build: true},
			"comp4-build":  pipeline.DeployComponentImage{ImagePath: imagePathFunc("comp4-build"), ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}, Build: true},
			"comp5-build":  pipeline.DeployComponentImage{ImagePath: imagePathFunc("comp5-build"), ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}, Build: true},
			"comp6-build":  pipeline.DeployComponentImage{ImagePath: imagePathFunc("comp6-build"), ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}, Build: true},
			"comp7-build":  pipeline.DeployComponentImage{ImagePath: imagePathFunc("comp7-build"), ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}, Build: true},
			"comp1-deploy": pipeline.DeployComponentImage{ImagePath: "any", ImageTagName: "", Runtime: nil, Build: false},
			"comp2-deploy": pipeline.DeployComponentImage{ImagePath: "any", ImageTagName: "", Runtime: nil, Build: false},
			"comp3-deploy": pipeline.DeployComponentImage{ImagePath: "any", ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}, Build: false},
			"comp4-deploy": pipeline.DeployComponentImage{ImagePath: "any", ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}, Build: false},
			"comp5-deploy": pipeline.DeployComponentImage{ImagePath: "any", ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}, Build: false},
			"comp6-deploy": pipeline.DeployComponentImage{ImagePath: "any", ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}, Build: false},
			"comp7-deploy": pipeline.DeployComponentImage{ImagePath: "any", ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}, Build: false},
			"job1-build":   pipeline.DeployComponentImage{ImagePath: imagePathFunc("job1-build"), ImageTagName: "", Runtime: nil, Build: true},
			"job2-build":   pipeline.DeployComponentImage{ImagePath: imagePathFunc("job2-build"), ImageTagName: "", Runtime: nil, Build: true},
			"job3-build":   pipeline.DeployComponentImage{ImagePath: imagePathFunc("job3-build"), ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}, Build: true},
			"job4-build":   pipeline.DeployComponentImage{ImagePath: imagePathFunc("job4-build"), ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}, Build: true},
			"job5-build":   pipeline.DeployComponentImage{ImagePath: imagePathFunc("job5-build"), ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}, Build: true},
			"job6-build":   pipeline.DeployComponentImage{ImagePath: imagePathFunc("job6-build"), ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}, Build: true},
			"job7-build":   pipeline.DeployComponentImage{ImagePath: imagePathFunc("job7-build"), ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}, Build: true},
			"job1-deploy":  pipeline.DeployComponentImage{ImagePath: "any", ImageTagName: "", Runtime: nil, Build: false},
			"job2-deploy":  pipeline.DeployComponentImage{ImagePath: "any", ImageTagName: "", Runtime: nil, Build: false},
			"job3-deploy":  pipeline.DeployComponentImage{ImagePath: "any", ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}, Build: false},
			"job4-deploy":  pipeline.DeployComponentImage{ImagePath: "any", ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}, Build: false},
			"job5-deploy":  pipeline.DeployComponentImage{ImagePath: "any", ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}, Build: false},
			"job6-deploy":  pipeline.DeployComponentImage{ImagePath: "any", ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}, Build: false},
			"job7-deploy":  pipeline.DeployComponentImage{ImagePath: "any", ImageTagName: "", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}, Build: false},
		},
	}
	s.Equal(expectedDeployEnvironmentComponentImages, pipelineInfo.DeployEnvironmentComponentImages)
}

/*******************************************************/
// func (s *applyConfigTestSuite) Test_ApplyConfig_ShouldNotFail() {
// 	appName := "anyapp"
// 	prepareConfigMapName := "preparecm"
// 	type scenario struct {
// 		name                string
// 		componentBuilder    *utils.RadixApplicationComponentBuilder
// 		jobComponentBuilder *utils.RadixApplicationJobComponentBuilder
// 	}
// 	scenarios := []scenario{
// 		{name: "no components"},
// 		{name: "with a component", componentBuilder: pointers.Ptr(utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("comp1"))},
// 		{name: "with a job-component", jobComponentBuilder: pointers.Ptr(utils.NewApplicationJobComponentBuilder().WithSchedulerPort(pointers.Ptr[int32](8080)).WithName("job1"))},
// 	}

// 	for _, ts := range scenarios {
// 		s.T().Run(ts.name, func(t *testing.T) {
// 			s.SetupTest()
// 			rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
// 			_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
// 			raBuilder := utils.NewRadixApplicationBuilder().
// 				WithAppName(appName).
// 				WithEnvironment("dev", "anybranch")
// 			if ts.componentBuilder != nil {
// 				raBuilder = raBuilder.WithComponent(*ts.componentBuilder)
// 			}
// 			if ts.jobComponentBuilder != nil {
// 				raBuilder = raBuilder.WithJobComponent(*ts.jobComponentBuilder)
// 			}
// 			ra := raBuilder.BuildRA()
// 			s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, appName, ra, nil))

// 			pipeline := model.PipelineInfo{
// 				PipelineArguments:  model.PipelineArguments{},
// 				RadixConfigMapName: prepareConfigMapName,
// 			}

// 			applyStep := applyconfig.NewApplyConfigStep()
// 			applyStep.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
// 			err := applyStep.Run(context.Background(), &pipeline)
// 			s.NoError(err)
// 		})
// 	}
// }

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
		{name: "no imageTagName in a component or an environment", expectedError: applyconfig.ErrMissingRequiredImageTagName},
		{name: "imageTagName is in a component", componentTagName: "some-component-tag"},
		{name: "imageTagName is not set in an environment", hasEnvironmentConfig: true, expectedError: applyconfig.ErrMissingRequiredImageTagName},
		{name: "imageTagName is in an environment", hasEnvironmentConfig: true, environmentTagName: "some-env-tag"},
		{name: "imageTagName is in a component, not in an environment", componentTagName: "some-component-tag", hasEnvironmentConfig: true},
		{name: "imageTagName is in a component and in an environment", componentTagName: "some-component-tag", hasEnvironmentConfig: true, environmentTagName: "some-env-tag"},
	}
	for _, ts := range scenarios {
		s.Run(ts.name, func() {
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

			applyStep := applyconfig.NewApplyConfigStep()
			applyStep.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
			err := applyStep.Run(context.Background(), &pipeline)
			if ts.expectedError == nil {
				s.NoError(err)
			} else {
				s.ErrorIs(err, applyconfig.ErrMissingRequiredImageTagName)
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

	applyStep := applyconfig.NewApplyConfigStep()
	applyStep.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	s.NoError(applyStep.Run(context.Background(), &pipeline))
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

	applyStep := applyconfig.NewApplyConfigStep()
	applyStep.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	s.NoError(applyStep.Run(context.Background(), &pipeline))
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

	applyStep := applyconfig.NewApplyConfigStep()
	applyStep.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	err := applyStep.Run(context.Background(), &pipeline)
	s.ErrorIs(err, applyconfig.ErrMissingRequiredImageTagName)
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

	applyStep := applyconfig.NewApplyConfigStep()
	applyStep.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	s.NoError(applyStep.Run(context.Background(), &pipeline))
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

	applyStep := applyconfig.NewApplyConfigStep()
	applyStep.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	s.NoError(applyStep.Run(context.Background(), &pipeline))
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

			applyStep := applyconfig.NewApplyConfigStep()
			applyStep.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
			err = applyStep.Run(context.Background(), &pipeline)
			if len(ts.expectedError) > 0 {
				s.Assert().EqualError(err, ts.expectedError, "missing error '%s'", ts.expectedError)
			} else {
				s.Assert().NoError(err)
			}
		})
	}
}

func (s *applyConfigTestSuite) Test_BuildDeploy_RuntimeValidation() {
	appName, branchName, schedulerPort := "anyapp", "anybranch", int32(9999)
	prepareConfigMapName := "preparecm"

	tests := map[string]struct {
		useBuildKit bool
		components  []utils.RadixApplicationComponentBuilder
		jobs        []utils.RadixApplicationJobComponentBuilder
		expectError bool
	}{
		"buildkit: support non-amd64 build architectures": {
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}),
				utils.NewApplicationComponentBuilder().WithName("comp2").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}),
			},
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithSchedulerPort(&schedulerPort).WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}),
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithSchedulerPort(&schedulerPort).WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}),
			},
			useBuildKit: true,
			expectError: false,
		},
		"non-buildkit: succeed if all components are amd64": {
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}),
				utils.NewApplicationComponentBuilder().WithName("comp2").WithRuntime(&radixv1.Runtime{Architecture: ""}),
				utils.NewApplicationComponentBuilder().WithName("comp3"),
			},
			useBuildKit: false,
			expectError: false,
		},
		"non-buildkit: fail if any component is non-amd64": {
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("comp1").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}),
				utils.NewApplicationComponentBuilder().WithName("comp2").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}),
			},
			useBuildKit: false,
			expectError: true,
		},
		"non-buildkit: succeed if all jobs are amd64": {
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}).WithSchedulerPort(&schedulerPort),
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithRuntime(&radixv1.Runtime{Architecture: ""}).WithSchedulerPort(&schedulerPort),
				utils.NewApplicationJobComponentBuilder().WithName("job3").WithSchedulerPort(&schedulerPort),
			},
			useBuildKit: false,
			expectError: false,
		},
		"non-buildkit: fail if any job is non-amd64": {
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("job1").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}).WithSchedulerPort(&schedulerPort),
				utils.NewApplicationJobComponentBuilder().WithName("job2").WithRuntime(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}).WithSchedulerPort(&schedulerPort),
			},
			useBuildKit: false,
			expectError: true,
		},
	}

	for name, test := range tests {
		s.Run(name, func() {
			rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
			_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
			ra := utils.NewRadixApplicationBuilder().
				WithAppName(appName).
				WithBuildKit(&test.useBuildKit).
				WithEnvironment("dev", branchName).
				WithComponents(test.components...).
				WithJobComponents(test.jobs...).
				BuildRA()
			s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, appName, ra, nil))

			pipeline := model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					PipelineType: string(radixv1.BuildDeploy),
					Branch:       branchName,
				},
				RadixConfigMapName: prepareConfigMapName,
			}

			applyStep := applyconfig.NewApplyConfigStep()
			applyStep.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
			err := applyStep.Run(context.Background(), &pipeline)
			if test.expectError {
				s.ErrorIs(err, applyconfig.ErrBuildNonDefaultRuntimeArchitectureWithoutBuildKitError)
			} else {
				s.NoError(err)
			}
		})
	}
}
