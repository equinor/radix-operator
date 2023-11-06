package steps_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/steps"
	pipelinewait "github.com/equinor/radix-operator/pipeline-runner/wait"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
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

	jobWaiter := pipelinewait.NewMockJobCompletionWaiter(s.ctrl)
	jobWaiter.EXPECT().Wait(gomock.Any()).Return(nil).Times(0)
	cli := steps.NewBuildStep(jobWaiter)
	cli.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	applicationConfig, _ := application.NewApplicationConfig(s.kubeClient, s.kubeUtil, s.radixClient, rr, ra, nil, nil)
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

func (s *buildTestSuite) Test_BuildDeploy_JobSpecAndDeploymentConsistent() {
	appName, envName, rjName, compName, cloneURL, buildBranch := "anyapp", "dev", "anyrj", "c1", "git@github.com:anyorg/anyrepo", "anybranch"
	prepareConfigMapName := "preparecm"
	gitConfigMapName, gitHash, gitTags := "gitcm", "githash", "gittags"
	rr := utils.NewRegistrationBuilder().WithCloneURL(cloneURL).WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	rj := utils.ARadixBuildDeployJob().WithJobName(rjName).WithAppName(appName).BuildRJ()
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(context.Background(), rj, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment(envName, buildBranch).
		WithEnvironment("prod", "release").
		WithComponent(utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName(compName)).
		BuildRA()
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra))
	s.Require().NoError(s.createGitInfoConfigMapResponse(gitConfigMapName, appName, gitHash, gitTags))
	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			PipelineType:      "build-deploy",
			Branch:            buildBranch,
			JobName:           rjName,
			ImageBuilder:      "builder:latest",
			CommitID:          "commit1234",
			ImageTag:          "imgtag",
			PushImage:         false,
			UseCache:          false,
			ContainerRegistry: "registry",
			Clustertype:       "clustertype",
			RadixZone:         "radixzone",
			Clustername:       "clustername",
			SubscriptionId:    "subscriptionid",
		},
		RadixConfigMapName: prepareConfigMapName,
		GitConfigMapName:   gitConfigMapName,
	}

	applyStep := steps.NewApplyConfigStep()
	applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	jobWaiter := pipelinewait.NewMockJobCompletionWaiter(s.ctrl)
	jobWaiter.EXPECT().Wait(gomock.Any()).Return(nil).Times(1)
	buildStep := steps.NewBuildStep(jobWaiter)
	buildStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	deployStep := steps.NewDeployStep(FakeNamespaceWatcher{})
	deployStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	s.Require().NoError(applyStep.Run(&pipeline))
	s.Require().NoError(buildStep.Run(&pipeline))
	s.Require().NoError(deployStep.Run(&pipeline))
	jobs, _ := s.kubeClient.BatchV1().Jobs(utils.GetAppNamespace(appName)).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
	job := jobs.Items[0]
	expectedPodLabels := map[string]string{kube.RadixJobNameLabel: rjName}
	s.Equal(expectedPodLabels, job.Spec.Template.Labels)
	expectedPodAnnotations := annotations.ForClusterAutoscalerSafeToEvict(false)
	s.Equal(expectedPodAnnotations, job.Spec.Template.Annotations)
	expectedVolumes := []corev1.Volume{
		{Name: git.BuildContextVolumeName},
		{Name: git.GitSSHKeyVolumeName, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: git.GitSSHKeyVolumeName, DefaultMode: pointers.Ptr[int32](256)}}},
		{Name: defaults.AzureACRServicePrincipleSecretName, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: defaults.AzureACRServicePrincipleSecretName}}},
	}
	s.ElementsMatch(expectedVolumes, job.Spec.Template.Spec.Volumes)

	// Check init containers
	s.Len(job.Spec.Template.Spec.InitContainers, 2)
	s.Require().ElementsMatch([]string{"internal-nslookup", "clone"}, slice.Map(job.Spec.Template.Spec.InitContainers, func(c corev1.Container) string { return c.Name }))
	cloneContainer, _ := slice.FindFirst(job.Spec.Template.Spec.InitContainers, func(c corev1.Container) bool { return c.Name == "clone" })
	s.Equal("alpine/git:user", cloneContainer.Image)
	s.Equal([]string{"/bin/sh", "-c"}, cloneContainer.Command)
	s.Equal([]string{fmt.Sprintf("git clone --recurse-submodules %s -b %s --verbose --progress /workspace", cloneURL, buildBranch)}, cloneContainer.Args)
	expectedCloneVolumeMounts := []corev1.VolumeMount{
		{Name: git.BuildContextVolumeName, MountPath: git.Workspace},
		{Name: git.GitSSHKeyVolumeName, MountPath: "/home/git-user/.ssh", ReadOnly: true},
	}
	s.ElementsMatch(expectedCloneVolumeMounts, cloneContainer.VolumeMounts)
	// Check containers
	s.Require().Len(job.Spec.Template.Spec.Containers, 1)
	s.Equal(fmt.Sprintf("build-%s", compName), job.Spec.Template.Spec.Containers[0].Name)
	s.Equal("registry/builder:latest", job.Spec.Template.Spec.Containers[0].Image)
	s.Len(job.Spec.Template.Spec.Containers[0].Args, 0)
	s.Len(job.Spec.Template.Spec.Containers[0].Command, 0)
	expectedBuildVolumeMounts := []corev1.VolumeMount{
		{Name: git.BuildContextVolumeName, MountPath: git.Workspace},
		{Name: defaults.AzureACRServicePrincipleSecretName, MountPath: "/radix-image-builder/.azure", ReadOnly: true},
	}
	s.ElementsMatch(expectedBuildVolumeMounts, job.Spec.Template.Spec.Containers[0].VolumeMounts)
	expectedEnv := []corev1.EnvVar{
		{Name: "DOCKER_FILE_NAME", Value: "Dockerfile"},
		{Name: "DOCKER_REGISTRY", Value: pipeline.PipelineArguments.ContainerRegistry},
		{Name: "IMAGE", Value: fmt.Sprintf("%s/%s-%s:%s", pipeline.PipelineArguments.ContainerRegistry, appName, compName, pipeline.PipelineArguments.ImageTag)},
		{Name: "CONTEXT", Value: "/workspace/"},
		{Name: "PUSH", Value: ""},
		{Name: "AZURE_CREDENTIALS", Value: "/radix-image-builder/.azure/sp_credentials.json"},
		{Name: "SUBSCRIPTION_ID", Value: pipeline.PipelineArguments.SubscriptionId},
		{Name: "CLUSTERTYPE_IMAGE", Value: fmt.Sprintf("%s/%s-%s:%s-%s", pipeline.PipelineArguments.ContainerRegistry, appName, compName, pipeline.PipelineArguments.Clustertype, pipeline.PipelineArguments.ImageTag)},
		{Name: "CLUSTERNAME_IMAGE", Value: fmt.Sprintf("%s/%s-%s:%s-%s", pipeline.PipelineArguments.ContainerRegistry, appName, compName, pipeline.PipelineArguments.Clustername, pipeline.PipelineArguments.ImageTag)},
		{Name: "REPOSITORY_NAME", Value: fmt.Sprintf("%s-%s", appName, compName)},
		{Name: "CACHE", Value: "--no-cache"},
		{Name: "RADIX_ZONE", Value: pipeline.PipelineArguments.RadixZone},
		{Name: "BRANCH", Value: pipeline.PipelineArguments.Branch},
		{Name: "TARGET_ENVIRONMENTS", Value: "dev"},
		{Name: "RADIX_GIT_COMMIT_HASH", Value: gitHash},
		{Name: "RADIX_GIT_TAGS", Value: gitTags},
		{Name: "BUILDAH_USERNAME", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: "username", LocalObjectReference: corev1.LocalObjectReference{Name: defaults.AzureACRServicePrincipleBuildahSecretName}}}},
		{Name: "BUILDAH_PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: "password", LocalObjectReference: corev1.LocalObjectReference{Name: defaults.AzureACRServicePrincipleBuildahSecretName}}}},
		{Name: "BUILDAH_CACHE_USERNAME", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: "username", LocalObjectReference: corev1.LocalObjectReference{Name: defaults.AzureACRTokenPasswordAppRegistrySecretName}}}},
		{Name: "BUILDAH_CACHE_PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: "password", LocalObjectReference: corev1.LocalObjectReference{Name: defaults.AzureACRTokenPasswordAppRegistrySecretName}}}},
	}
	s.ElementsMatch(expectedEnv, job.Spec.Template.Spec.Containers[0].Env)

	rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, envName)).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(rds.Items, 1)
	rd := rds.Items[0]
	s.Require().Len(rd.Spec.Components, 1)
	s.Equal(compName, rd.Spec.Components[0].Name)
	s.Equal(fmt.Sprintf("%s/%s-%s:%s", pipeline.PipelineArguments.ContainerRegistry, appName, compName, pipeline.PipelineArguments.ImageTag), rd.Spec.Components[0].Image)
}

func (s *buildTestSuite) Test_BuildJobSpec_MultipleComponents() {
	appName, envName, rjName, buildBranch, jobPort := "anyapp", "dev", "anyrj", "anybranch", pointers.Ptr[int32](9999)
	prepareConfigMapName := "preparecm"

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	rj := utils.ARadixBuildDeployJob().WithJobName(rjName).WithAppName(appName).BuildRJ()
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(context.Background(), rj, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment(envName, buildBranch).
		WithComponents(
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("client-component-1").WithSourceFolder("./client/").WithDockerfileName("client.Dockerfile"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("client-component-2").WithSourceFolder("./client/").WithDockerfileName("client.Dockerfile"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("server-component-1").WithSourceFolder("./server/").WithDockerfileName("server.Dockerfile"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("server-component-2").WithSourceFolder("./server/").WithDockerfileName("server.Dockerfile"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("single-component").WithSourceFolder("."),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("public-image-component").WithImage("swaggerapi/swagger-ui"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("private-hub-component").WithImage("radixcanary.azurecr.io/nginx:latest"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("compute-shared-1").WithSourceFolder("./compute/").WithDockerfileName("compute.Dockerfile"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("compute-shared-with-different-dockerfile-1").WithSourceFolder("./compute-with-different-dockerfile/").WithDockerfileName("compute-custom1.Dockerfile"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("compute-shared-with-different-dockerfile-2").WithSourceFolder("./compute-with-different-dockerfile/").WithDockerfileName("compute-custom2.Dockerfile"),
		).
		WithJobComponents(
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("compute-shared-2").WithDockerfileName("compute.Dockerfile").WithSourceFolder("./compute/"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("compute-shared-with-different-dockerfile-3").WithSourceFolder("./compute-with-different-dockerfile/").WithDockerfileName("compute-custom3.Dockerfile"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("single-job").WithDockerfileName("job.Dockerfile").WithSourceFolder("./job/"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("calc-1").WithDockerfileName("calc.Dockerfile").WithSourceFolder("./calc/"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("calc-2").WithDockerfileName("calc.Dockerfile").WithSourceFolder("./calc/"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("public-job-component").WithImage("job/job:latest"),
		).
		BuildRA()
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra))
	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			PipelineType:      "build-deploy",
			Branch:            buildBranch,
			JobName:           rjName,
			ImageTag:          "imgtag",
			ContainerRegistry: "registry",
			Clustertype:       "clustertype",
			Clustername:       "clustername",
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	applyStep := steps.NewApplyConfigStep()
	applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	jobWaiter := pipelinewait.NewMockJobCompletionWaiter(s.ctrl)
	jobWaiter.EXPECT().Wait(gomock.Any()).Return(nil).Times(1)
	buildStep := steps.NewBuildStep(jobWaiter)
	buildStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	deployStep := steps.NewDeployStep(FakeNamespaceWatcher{})
	deployStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	s.Require().NoError(applyStep.Run(&pipeline))
	s.Require().NoError(buildStep.Run(&pipeline))
	s.Require().NoError(deployStep.Run(&pipeline))
	jobs, _ := s.kubeClient.BatchV1().Jobs(utils.GetAppNamespace(appName)).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
	job := jobs.Items[0]

	// Check build containers
	type jobContainerSpec struct {
		Name    string
		Docker  string
		Image   string
		Context string
	}
	imageNameFunc := func(s string) string {
		return fmt.Sprintf("%s/%s-%s:%s", pipeline.PipelineArguments.ContainerRegistry, appName, s, pipeline.PipelineArguments.ImageTag)
	}
	expectedJobContainers := []jobContainerSpec{
		{Name: "build-multi-component", Docker: "client.Dockerfile", Context: "/workspace/client/", Image: imageNameFunc("multi-component")},
		{Name: "build-multi-component-1", Docker: "server.Dockerfile", Context: "/workspace/server/", Image: imageNameFunc("multi-component-1")},
		{Name: "build-single-component", Docker: "Dockerfile", Context: "/workspace/", Image: imageNameFunc("single-component")},
		{Name: "build-multi-component-2", Docker: "compute.Dockerfile", Context: "/workspace/compute/", Image: imageNameFunc("multi-component-2")},
		{Name: "build-compute-shared-with-different-dockerfile-1", Docker: "compute-custom1.Dockerfile", Context: "/workspace/compute-with-different-dockerfile/", Image: imageNameFunc("compute-shared-with-different-dockerfile-1")},
		{Name: "build-compute-shared-with-different-dockerfile-2", Docker: "compute-custom2.Dockerfile", Context: "/workspace/compute-with-different-dockerfile/", Image: imageNameFunc("compute-shared-with-different-dockerfile-2")},
		{Name: "build-compute-shared-with-different-dockerfile-3", Docker: "compute-custom3.Dockerfile", Context: "/workspace/compute-with-different-dockerfile/", Image: imageNameFunc("compute-shared-with-different-dockerfile-3")},
		{Name: "build-single-job", Docker: "job.Dockerfile", Context: "/workspace/job/", Image: imageNameFunc("single-job")},
		{Name: "build-multi-component-3", Docker: "calc.Dockerfile", Context: "/workspace/calc/", Image: imageNameFunc("multi-component-3")},
	}
	actualJobContainers := slice.Map(job.Spec.Template.Spec.Containers, func(c corev1.Container) jobContainerSpec {
		getEnv := func(env string) string {
			if i := slice.FindIndex(c.Env, func(e corev1.EnvVar) bool { return e.Name == env }); i >= 0 {
				return c.Env[i].Value
			}
			return ""
		}
		return jobContainerSpec{
			Name:    c.Name,
			Docker:  getEnv("DOCKER_FILE_NAME"),
			Image:   getEnv("IMAGE"),
			Context: getEnv("CONTEXT"),
		}
	})
	s.ElementsMatch(expectedJobContainers, actualJobContainers)

	// Check RadixDeployment component and job images
	rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, envName)).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(rds.Items, 1)
	rd := rds.Items[0]
	type deployComponentSpec struct {
		Name  string
		Image string
	}
	expectedDeployComponents := []deployComponentSpec{
		{Name: "client-component-1", Image: imageNameFunc("multi-component")},
		{Name: "client-component-2", Image: imageNameFunc("multi-component")},
		{Name: "server-component-1", Image: imageNameFunc("multi-component-1")},
		{Name: "server-component-2", Image: imageNameFunc("multi-component-1")},
		{Name: "single-component", Image: imageNameFunc("single-component")},
		{Name: "public-image-component", Image: "swaggerapi/swagger-ui"},
		{Name: "private-hub-component", Image: "radixcanary.azurecr.io/nginx:latest"},
		{Name: "compute-shared-1", Image: imageNameFunc("multi-component-2")},
		{Name: "compute-shared-with-different-dockerfile-1", Image: imageNameFunc("compute-shared-with-different-dockerfile-1")},
		{Name: "compute-shared-with-different-dockerfile-2", Image: imageNameFunc("compute-shared-with-different-dockerfile-2")},
	}
	actualDeployComponents := slice.Map(rd.Spec.Components, func(c radixv1.RadixDeployComponent) deployComponentSpec {
		return deployComponentSpec{Name: c.Name, Image: c.Image}
	})
	s.ElementsMatch(expectedDeployComponents, actualDeployComponents)
	expectedJobComponents := []deployComponentSpec{
		{Name: "compute-shared-2", Image: imageNameFunc("multi-component-2")},
		{Name: "compute-shared-with-different-dockerfile-3", Image: imageNameFunc("compute-shared-with-different-dockerfile-3")},
		{Name: "single-job", Image: imageNameFunc("single-job")},
		{Name: "calc-1", Image: imageNameFunc("multi-component-3")},
		{Name: "calc-2", Image: imageNameFunc("multi-component-3")},
		{Name: "public-job-component", Image: "job/job:latest"},
	}
	actualJobComponents := slice.Map(rd.Spec.Jobs, func(c radixv1.RadixDeployJobComponent) deployComponentSpec {
		return deployComponentSpec{Name: c.Name, Image: c.Image}
	})
	s.ElementsMatch(expectedJobComponents, actualJobComponents)
}

func (s *buildTestSuite) Test_BuildJobSpec_MultipleComponents_IgnoreDisabled() {
	appName, envName, rjName, buildBranch, jobPort := "anyapp", "dev", "anyrj", "anybranch", pointers.Ptr[int32](9999)
	prepareConfigMapName := "preparecm"

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	rj := utils.ARadixBuildDeployJob().WithJobName(rjName).WithAppName(appName).BuildRJ()
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(context.Background(), rj, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment(envName, buildBranch).
		WithComponents(
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("client-component-1").WithSourceFolder("./client/").WithDockerfileName("client.Dockerfile"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("client-component-2").WithEnabled(true).WithSourceFolder("./client/").WithDockerfileName("client.Dockerfile"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("client-component-3").WithEnabled(false).WithSourceFolder("./client/").WithDockerfileName("client.Dockerfile"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("client-component-4").WithSourceFolder("./client2/").WithDockerfileName("client.Dockerfile"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("client-component-5").WithEnabled(false).WithSourceFolder("./client2/").WithDockerfileName("client.Dockerfile"),
		).
		WithJobComponents(
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("calc-1").WithDockerfileName("calc.Dockerfile").WithSourceFolder("./calc/"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("calc-2").WithEnabled(true).WithDockerfileName("calc.Dockerfile").WithSourceFolder("./calc/"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("calc-3").WithEnabled(false).WithDockerfileName("calc.Dockerfile").WithSourceFolder("./calc/"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("calc-4").WithDockerfileName("calc.Dockerfile").WithSourceFolder("./calc2/"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("calc-5").WithEnabled(false).WithDockerfileName("calc.Dockerfile").WithSourceFolder("./calc2/"),
		).
		BuildRA()
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra))
	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			PipelineType:      "build-deploy",
			Branch:            buildBranch,
			JobName:           rjName,
			ImageTag:          "imgtag",
			ContainerRegistry: "registry",
			Clustertype:       "clustertype",
			Clustername:       "clustername",
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	applyStep := steps.NewApplyConfigStep()
	applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	jobWaiter := pipelinewait.NewMockJobCompletionWaiter(s.ctrl)
	jobWaiter.EXPECT().Wait(gomock.Any()).Return(nil).Times(1)
	buildStep := steps.NewBuildStep(jobWaiter)
	buildStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	deployStep := steps.NewDeployStep(FakeNamespaceWatcher{})
	deployStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	s.Require().NoError(applyStep.Run(&pipeline))
	s.Require().NoError(buildStep.Run(&pipeline))
	s.Require().NoError(deployStep.Run(&pipeline))
	jobs, _ := s.kubeClient.BatchV1().Jobs(utils.GetAppNamespace(appName)).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
	job := jobs.Items[0]

	// Check build containers
	type jobContainerSpec struct {
		Name    string
		Docker  string
		Image   string
		Context string
	}
	imageNameFunc := func(s string) string {
		return fmt.Sprintf("%s/%s-%s:%s", pipeline.PipelineArguments.ContainerRegistry, appName, s, pipeline.PipelineArguments.ImageTag)
	}
	expectedJobContainers := []jobContainerSpec{
		{Name: "build-multi-component", Docker: "client.Dockerfile", Context: "/workspace/client/", Image: imageNameFunc("multi-component")},
		{Name: "build-multi-component-1", Docker: "calc.Dockerfile", Context: "/workspace/calc/", Image: imageNameFunc("multi-component-1")},
		{Name: "build-client-component-4", Docker: "client.Dockerfile", Context: "/workspace/client2/", Image: imageNameFunc("client-component-4")},
		{Name: "build-calc-4", Docker: "calc.Dockerfile", Context: "/workspace/calc2/", Image: imageNameFunc("calc-4")},
	}
	actualJobContainers := slice.Map(job.Spec.Template.Spec.Containers, func(c corev1.Container) jobContainerSpec {
		getEnv := func(env string) string {
			if i := slice.FindIndex(c.Env, func(e corev1.EnvVar) bool { return e.Name == env }); i >= 0 {
				return c.Env[i].Value
			}
			return ""
		}
		return jobContainerSpec{
			Name:    c.Name,
			Docker:  getEnv("DOCKER_FILE_NAME"),
			Image:   getEnv("IMAGE"),
			Context: getEnv("CONTEXT"),
		}
	})
	s.ElementsMatch(expectedJobContainers, actualJobContainers)

	// Check RadixDeployment component and job images
	rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, envName)).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(rds.Items, 1)
	rd := rds.Items[0]
	type deployComponentSpec struct {
		Name  string
		Image string
	}
	expectedDeployComponents := []deployComponentSpec{
		{Name: "client-component-1", Image: imageNameFunc("multi-component")},
		{Name: "client-component-2", Image: imageNameFunc("multi-component")},
		{Name: "client-component-4", Image: imageNameFunc("client-component-4")},
	}
	actualDeployComponents := slice.Map(rd.Spec.Components, func(c radixv1.RadixDeployComponent) deployComponentSpec {
		return deployComponentSpec{Name: c.Name, Image: c.Image}
	})
	s.ElementsMatch(expectedDeployComponents, actualDeployComponents)
	expectedJobComponents := []deployComponentSpec{
		{Name: "calc-1", Image: imageNameFunc("multi-component-1")},
		{Name: "calc-2", Image: imageNameFunc("multi-component-1")},
		{Name: "calc-4", Image: imageNameFunc("calc-4")},
	}
	actualJobComponents := slice.Map(rd.Spec.Jobs, func(c radixv1.RadixDeployJobComponent) deployComponentSpec {
		return deployComponentSpec{Name: c.Name, Image: c.Image}
	})
	s.ElementsMatch(expectedJobComponents, actualJobComponents)
}

func (s *buildTestSuite) Test_BuildJobSpec_PushImage() {
	appName, rjName, compName := "anyapp", "anyrj", "c1"
	prepareConfigMapName := "preparecm"
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	rj := utils.ARadixBuildDeployJob().WithJobName(rjName).WithAppName(appName).BuildRJ()
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(context.Background(), rj, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment("dev", "main").
		WithComponent(utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName(compName)).
		BuildRA()
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra))
	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			Branch:    "main",
			JobName:   rjName,
			PushImage: true,
		},
		RadixConfigMapName: prepareConfigMapName,
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
	job := jobs.Items[0]
	s.Require().Len(job.Spec.Template.Spec.Containers, 1)
	expectedEnv := []corev1.EnvVar{
		{Name: "PUSH", Value: "--push"},
	}
	s.Subset(job.Spec.Template.Spec.Containers[0].Env, expectedEnv)
}

func (s *buildTestSuite) Test_BuildJobSpec_UseCache() {
	appName, rjName, compName := "anyapp", "anyrj", "c1"
	prepareConfigMapName := "preparecm"
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	rj := utils.ARadixBuildDeployJob().WithJobName(rjName).WithAppName(appName).BuildRJ()
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(context.Background(), rj, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment("dev", "main").
		WithComponent(utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName(compName)).
		BuildRA()
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra))
	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			Branch:   "main",
			JobName:  rjName,
			UseCache: true,
		},
		RadixConfigMapName: prepareConfigMapName,
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
	job := jobs.Items[0]
	s.Require().Len(job.Spec.Template.Spec.Containers, 1)
	expectedEnv := []corev1.EnvVar{
		{Name: "CACHE", Value: ""},
	}
	s.Subset(job.Spec.Template.Spec.Containers[0].Env, expectedEnv)
}

func (s *buildTestSuite) Test_BuildJobSpec_WithDockerfileName() {
	appName, rjName, compName, dockerFileName := "anyapp", "anyrj", "c1", "anydockerfile"
	prepareConfigMapName := "preparecm"
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	rj := utils.ARadixBuildDeployJob().WithJobName(rjName).WithAppName(appName).BuildRJ()
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(context.Background(), rj, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment("dev", "main").
		WithComponent(utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName(compName).WithDockerfileName(dockerFileName)).
		BuildRA()
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra))
	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			Branch:  "main",
			JobName: rjName,
		},
		RadixConfigMapName: prepareConfigMapName,
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
	job := jobs.Items[0]
	s.Require().Len(job.Spec.Template.Spec.Containers, 1)
	expectedEnv := []corev1.EnvVar{
		{Name: "DOCKER_FILE_NAME", Value: dockerFileName},
	}
	s.Subset(job.Spec.Template.Spec.Containers[0].Env, expectedEnv)
}

func (s *buildTestSuite) Test_BuildJobSpec_WithSourceFolder() {
	appName, rjName, compName := "anyapp", "anyrj", "c1"
	prepareConfigMapName := "preparecm"
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	rj := utils.ARadixBuildDeployJob().WithJobName(rjName).WithAppName(appName).BuildRJ()
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(context.Background(), rj, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment("dev", "main").
		WithComponent(utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName(compName).WithSourceFolder(".././path/../../subpath")).
		BuildRA()
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra))
	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			Branch:  "main",
			JobName: rjName,
		},
		RadixConfigMapName: prepareConfigMapName,
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
	job := jobs.Items[0]
	s.Require().Len(job.Spec.Template.Spec.Containers, 1)
	expectedEnv := []corev1.EnvVar{
		{Name: "CONTEXT", Value: "/workspace/subpath/"},
	}
	s.Subset(job.Spec.Template.Spec.Containers[0].Env, expectedEnv)
}

func (s *buildTestSuite) Test_BuildJobSpec_WithBuildSecrets() {
	appName, rjName, compName := "anyapp", "anyrj", "c1"
	prepareConfigMapName := "preparecm"
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	rj := utils.ARadixBuildDeployJob().WithJobName(rjName).WithAppName(appName).BuildRJ()
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(context.Background(), rj, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithBuildSecrets("SECRET1", "SECRET2").
		WithEnvironment("dev", "main").
		WithComponent(utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName(compName)).
		BuildRA()
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra))
	s.Require().NoError(s.createBuildSecret(appName, map[string][]byte{"SECRET1": nil, "SECRET2": nil}))
	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			Branch:  "main",
			JobName: rjName,
		},
		RadixConfigMapName: prepareConfigMapName,
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
	job := jobs.Items[0]
	s.Len(job.Spec.Template.Spec.Volumes, 4)
	expectedVolumes := []corev1.Volume{
		{Name: defaults.BuildSecretsName, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: defaults.BuildSecretsName}}},
	}
	s.Subset(job.Spec.Template.Spec.Volumes, expectedVolumes)
	s.Require().Len(job.Spec.Template.Spec.Containers, 1)
	s.Len(job.Spec.Template.Spec.Containers[0].VolumeMounts, 3)
	expectedVolumeMounts := []corev1.VolumeMount{
		{Name: defaults.BuildSecretsName, MountPath: "/build-secrets", ReadOnly: true},
	}
	s.Subset(job.Spec.Template.Spec.Containers[0].VolumeMounts, expectedVolumeMounts)
}

func (s *buildTestSuite) Test_BuildJobSpec_BuildKit() {
	appName, rjName, compName, sourceFolder, dockerFile := "anyapp", "anyrj", "c1", "../path1/./../../path2", "anydockerfile"
	prepareConfigMapName := "preparecm"
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	rj := utils.ARadixBuildDeployJob().WithJobName(rjName).WithAppName(appName).BuildRJ()
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(context.Background(), rj, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithBuildKit(pointers.Ptr(true)).
		WithEnvironment("dev", "main").
		WithComponent(utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName(compName).WithDockerfileName(dockerFile).WithSourceFolder(sourceFolder)).
		BuildRA()
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra))
	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			Branch:               "main",
			JobName:              rjName,
			BuildKitImageBuilder: "anybuildkitimage:tag",
			ImageTag:             "anyimagetag",
			ContainerRegistry:    "anyregistry",
			AppContainerRegistry: "anyappregistry",
			Clustertype:          "anyclustertype",
			Clustername:          "anyclustername",
			Builder:              model.Builder{ResourcesLimitsMemory: "100M", ResourcesRequestsCPU: "50m", ResourcesRequestsMemory: "50M"},
		},
		RadixConfigMapName: prepareConfigMapName,
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
	job := jobs.Items[0]
	s.Require().Len(job.Spec.Template.Spec.Containers, 1)
	s.Equal(pipeline.PipelineArguments.BuildKitImageBuilder, job.Spec.Template.Spec.Containers[0].Image)
	expectedBuildCmd := strings.Join(
		[]string{
			fmt.Sprintf("/usr/bin/buildah login --username ${BUILDAH_USERNAME} --password ${BUILDAH_PASSWORD} %s && ", pipeline.PipelineArguments.ContainerRegistry),
			fmt.Sprintf("/usr/bin/buildah login --username ${BUILDAH_CACHE_USERNAME} --password ${BUILDAH_CACHE_PASSWORD} %s && ", pipeline.PipelineArguments.AppContainerRegistry),
			"/usr/bin/buildah build --storage-driver=overlay --isolation=chroot --jobs 0 ",
			fmt.Sprintf("--file %s%s ", "/workspace/path2/", dockerFile),
			"--build-arg RADIX_GIT_COMMIT_HASH=\"${RADIX_GIT_COMMIT_HASH}\" ",
			"--build-arg RADIX_GIT_TAGS=\"${RADIX_GIT_TAGS}\" ",
			"--build-arg BRANCH=\"${BRANCH}\" ",
			"--build-arg TARGET_ENVIRONMENTS=\"${TARGET_ENVIRONMENTS}\" ",
			"--layers ",
			fmt.Sprintf("--cache-to=%s/%s/cache ", pipeline.PipelineArguments.AppContainerRegistry, appName),
			fmt.Sprintf("--cache-from=%s/%s/cache ", pipeline.PipelineArguments.AppContainerRegistry, appName),
			"/workspace/path2/",
		},
		"",
	)
	expectedCommand := []string{"/bin/bash", "-c", expectedBuildCmd}
	s.Equal(expectedCommand, job.Spec.Template.Spec.Containers[0].Command)
}

func (s *buildTestSuite) Test_BuildJobSpec_BuildKit_PushImage() {
	appName, rjName, compName, sourceFolder, dockerFile := "anyapp", "anyrj", "c1", "../path1/./../../path2", "anydockerfile"
	prepareConfigMapName := "preparecm"
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	rj := utils.ARadixBuildDeployJob().WithJobName(rjName).WithAppName(appName).BuildRJ()
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(context.Background(), rj, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithBuildKit(pointers.Ptr(true)).
		WithEnvironment("dev", "main").
		WithComponent(utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName(compName).WithDockerfileName(dockerFile).WithSourceFolder(sourceFolder)).
		BuildRA()
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra))
	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			Branch:               "main",
			JobName:              rjName,
			BuildKitImageBuilder: "anybuildkitimage:tag",
			ImageTag:             "anyimagetag",
			ContainerRegistry:    "anyregistry",
			AppContainerRegistry: "anyappregistry",
			Clustertype:          "anyclustertype",
			Clustername:          "anyclustername",
			PushImage:            true,
			Builder:              model.Builder{ResourcesLimitsMemory: "100M", ResourcesRequestsCPU: "50m", ResourcesRequestsMemory: "50M"},
		},
		RadixConfigMapName: prepareConfigMapName,
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
	job := jobs.Items[0]
	s.Require().Len(job.Spec.Template.Spec.Containers, 1)
	s.Equal(pipeline.PipelineArguments.BuildKitImageBuilder, job.Spec.Template.Spec.Containers[0].Image)
	expectedBuildCmd := strings.Join(
		[]string{
			fmt.Sprintf("/usr/bin/buildah login --username ${BUILDAH_USERNAME} --password ${BUILDAH_PASSWORD} %s && ", pipeline.PipelineArguments.ContainerRegistry),
			fmt.Sprintf("/usr/bin/buildah login --username ${BUILDAH_CACHE_USERNAME} --password ${BUILDAH_CACHE_PASSWORD} %s && ", pipeline.PipelineArguments.AppContainerRegistry),
			"/usr/bin/buildah build --storage-driver=overlay --isolation=chroot --jobs 0 ",
			fmt.Sprintf("--file %s%s ", "/workspace/path2/", dockerFile),
			"--build-arg RADIX_GIT_COMMIT_HASH=\"${RADIX_GIT_COMMIT_HASH}\" ",
			"--build-arg RADIX_GIT_TAGS=\"${RADIX_GIT_TAGS}\" ",
			"--build-arg BRANCH=\"${BRANCH}\" ",
			"--build-arg TARGET_ENVIRONMENTS=\"${TARGET_ENVIRONMENTS}\" ",
			"--layers ",
			fmt.Sprintf("--cache-to=%s/%s/cache ", pipeline.PipelineArguments.AppContainerRegistry, appName),
			fmt.Sprintf("--cache-from=%s/%s/cache ", pipeline.PipelineArguments.AppContainerRegistry, appName),
			fmt.Sprintf("--tag %s/%s-%s:%s ", pipeline.PipelineArguments.ContainerRegistry, appName, compName, pipeline.PipelineArguments.ImageTag),
			fmt.Sprintf("--tag %s/%s-%s:%s-%s ", pipeline.PipelineArguments.ContainerRegistry, appName, compName, pipeline.PipelineArguments.Clustertype, pipeline.PipelineArguments.ImageTag),
			fmt.Sprintf("--tag %s/%s-%s:%s-%s ", pipeline.PipelineArguments.ContainerRegistry, appName, compName, pipeline.PipelineArguments.Clustername, pipeline.PipelineArguments.ImageTag),
			"/workspace/path2/ && ",
			fmt.Sprintf("/usr/bin/buildah push --storage-driver=overlay %s/%s-%s:%s && ", pipeline.PipelineArguments.ContainerRegistry, appName, compName, pipeline.PipelineArguments.ImageTag),
			fmt.Sprintf("/usr/bin/buildah push --storage-driver=overlay %s/%s-%s:%s-%s && ", pipeline.PipelineArguments.ContainerRegistry, appName, compName, pipeline.PipelineArguments.Clustertype, pipeline.PipelineArguments.ImageTag),
			fmt.Sprintf("/usr/bin/buildah push --storage-driver=overlay %s/%s-%s:%s-%s", pipeline.PipelineArguments.ContainerRegistry, appName, compName, pipeline.PipelineArguments.Clustername, pipeline.PipelineArguments.ImageTag),
		},
		"",
	)
	expectedCommand := []string{"/bin/bash", "-c", expectedBuildCmd}
	s.Equal(expectedCommand, job.Spec.Template.Spec.Containers[0].Command)
}

func (s *buildTestSuite) Test_BuildJobSpec_BuildKit_WithBuildSecrets() {
	appName, rjName, compName, sourceFolder, dockerFile := "anyapp", "anyrj", "c1", "../path1/./../../path2", "anydockerfile"
	prepareConfigMapName := "preparecm"
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	rj := utils.ARadixBuildDeployJob().WithJobName(rjName).WithAppName(appName).BuildRJ()
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(context.Background(), rj, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithBuildKit(pointers.Ptr(true)).
		WithBuildSecrets("SECRET1", "SECRET2").
		WithEnvironment("dev", "main").
		WithComponent(utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName(compName).WithDockerfileName(dockerFile).WithSourceFolder(sourceFolder)).
		BuildRA()
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra))
	s.Require().NoError(s.createBuildSecret(appName, map[string][]byte{"SECRET1": nil, "SECRET2": nil}))
	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			Branch:               "main",
			JobName:              rjName,
			BuildKitImageBuilder: "anybuildkitimage:tag",
			ImageTag:             "anyimagetag",
			ContainerRegistry:    "anyregistry",
			Clustertype:          "anyclustertype",
			Clustername:          "anyclustername",
			Builder:              model.Builder{ResourcesLimitsMemory: "100M", ResourcesRequestsCPU: "50m", ResourcesRequestsMemory: "50M"},
		},
		RadixConfigMapName: prepareConfigMapName,
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
	job := jobs.Items[0]
	s.Len(job.Spec.Template.Spec.Volumes, 4)
	expectedVolumes := []corev1.Volume{
		{Name: defaults.BuildSecretsName, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: defaults.BuildSecretsName}}},
	}
	s.Subset(job.Spec.Template.Spec.Volumes, expectedVolumes)
	s.Require().Len(job.Spec.Template.Spec.Containers, 1)
	s.Len(job.Spec.Template.Spec.Containers[0].VolumeMounts, 3)
	expectedVolumeMounts := []corev1.VolumeMount{
		{Name: defaults.BuildSecretsName, MountPath: "/build-secrets", ReadOnly: true},
	}
	s.Subset(job.Spec.Template.Spec.Containers[0].VolumeMounts, expectedVolumeMounts)
	s.Equal(pipeline.PipelineArguments.BuildKitImageBuilder, job.Spec.Template.Spec.Containers[0].Image)
	expectedBuildCmd := strings.Join(
		[]string{
			fmt.Sprintf("/usr/bin/buildah login --username ${BUILDAH_USERNAME} --password ${BUILDAH_PASSWORD} %s && ", pipeline.PipelineArguments.ContainerRegistry),
			fmt.Sprintf("/usr/bin/buildah login --username ${BUILDAH_CACHE_USERNAME} --password ${BUILDAH_CACHE_PASSWORD} %s && ", pipeline.PipelineArguments.AppContainerRegistry),
			"/usr/bin/buildah build --storage-driver=overlay --isolation=chroot --jobs 0 ",
			"--secret id=SECRET1,src=/build-secrets/SECRET1 --secret id=SECRET2,src=/build-secrets/SECRET2 ",
			fmt.Sprintf("--file %s%s ", "/workspace/path2/", dockerFile),
			"--build-arg RADIX_GIT_COMMIT_HASH=\"${RADIX_GIT_COMMIT_HASH}\" ",
			"--build-arg RADIX_GIT_TAGS=\"${RADIX_GIT_TAGS}\" ",
			"--build-arg BRANCH=\"${BRANCH}\" ",
			"--build-arg TARGET_ENVIRONMENTS=\"${TARGET_ENVIRONMENTS}\" ",
			"--layers ",
			fmt.Sprintf("--cache-to=%s/%s/cache ", pipeline.PipelineArguments.AppContainerRegistry, appName),
			fmt.Sprintf("--cache-from=%s/%s/cache ", pipeline.PipelineArguments.AppContainerRegistry, appName),
			"/workspace/path2/",
		},
		"",
	)
	expectedCommand := []string{"/bin/bash", "-c", expectedBuildCmd}
	s.Equal(expectedCommand, job.Spec.Template.Spec.Containers[0].Command)
}

func (s *buildTestSuite) createPreparePipelineConfigMapResponse(configMapName, appName string, ra *radixv1.RadixApplication) error {
	raBytes, err := yamlk8s.Marshal(ra)
	if err != nil {
		return err
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: configMapName},
		Data: map[string]string{
			pipelineDefaults.PipelineConfigMapContent: string(raBytes),
		},
	}
	_, err = s.kubeClient.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Create(context.Background(), cm, metav1.CreateOptions{})
	return err
}

func (s *buildTestSuite) createGitInfoConfigMapResponse(configMapName, appName, gitHash, gitTags string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: configMapName},
		Data: map[string]string{
			defaults.RadixGitCommitHashKey: gitHash,
			defaults.RadixGitTagsKey:       gitTags,
		},
	}
	_, err := s.kubeClient.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Create(context.Background(), cm, metav1.CreateOptions{})
	return err
}

func (s *buildTestSuite) createBuildSecret(appName string, data map[string][]byte) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: defaults.BuildSecretsName},
		Data:       data,
	}

	_, err := s.kubeClient.CoreV1().Secrets(utils.GetAppNamespace(appName)).Create(context.Background(), secret, metav1.CreateOptions{})
	return err
}
