package steps_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

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
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
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

func (s *buildTestSuite) SetupSubTest() {
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

	applicationConfig, _ := application.NewApplicationConfig(s.kubeClient, s.kubeUtil, s.radixClient, rr, ra)
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
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra, nil))
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
	s.Equal(pipeline.RadixApplicationHash(), rd.GetAnnotations()[kube.RadixConfigHash])
	s.Greater(len(rd.GetAnnotations()[kube.RadixConfigHash]), 0)
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
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra, nil))
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
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("client-component-6").WithEnabled(false).WithSourceFolder("./client3/").WithDockerfileName("client.Dockerfile").
				WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment(envName).WithEnabled(true)),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("client-component-7").WithEnabled(true).WithSourceFolder("./client4/").WithDockerfileName("client.Dockerfile").
				WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment(envName).WithEnabled(false)),
		).
		WithJobComponents(
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("calc-1").WithDockerfileName("calc.Dockerfile").WithSourceFolder("./calc/"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("calc-2").WithEnabled(true).WithDockerfileName("calc.Dockerfile").WithSourceFolder("./calc/"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("calc-3").WithEnabled(false).WithDockerfileName("calc.Dockerfile").WithSourceFolder("./calc/"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("calc-4").WithDockerfileName("calc.Dockerfile").WithSourceFolder("./calc2/"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("calc-5").WithEnabled(false).WithDockerfileName("calc.Dockerfile").WithSourceFolder("./calc2/"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("calc-6").WithEnabled(false).WithDockerfileName("calc.Dockerfile").WithSourceFolder("./calc3/").
				WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName).WithEnabled(true)),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("calc-7").WithEnabled(true).WithDockerfileName("calc.Dockerfile").WithSourceFolder("./calc4/").
				WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName).WithEnabled(false)),
		).
		BuildRA()
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra, nil))
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
		{Name: "build-client-component-6", Docker: "client.Dockerfile", Context: "/workspace/client3/", Image: imageNameFunc("client-component-6")},
		{Name: "build-calc-4", Docker: "calc.Dockerfile", Context: "/workspace/calc2/", Image: imageNameFunc("calc-4")},
		{Name: "build-calc-6", Docker: "calc.Dockerfile", Context: "/workspace/calc3/", Image: imageNameFunc("calc-6")},
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
		{Name: "client-component-6", Image: imageNameFunc("client-component-6")},
	}
	actualDeployComponents := slice.Map(rd.Spec.Components, func(c radixv1.RadixDeployComponent) deployComponentSpec {
		return deployComponentSpec{Name: c.Name, Image: c.Image}
	})
	s.ElementsMatch(expectedDeployComponents, actualDeployComponents)
	expectedJobComponents := []deployComponentSpec{
		{Name: "calc-1", Image: imageNameFunc("multi-component-1")},
		{Name: "calc-2", Image: imageNameFunc("multi-component-1")},
		{Name: "calc-4", Image: imageNameFunc("calc-4")},
		{Name: "calc-6", Image: imageNameFunc("calc-6")},
	}
	actualJobComponents := slice.Map(rd.Spec.Jobs, func(c radixv1.RadixDeployJobComponent) deployComponentSpec {
		return deployComponentSpec{Name: c.Name, Image: c.Image}
	})
	s.ElementsMatch(expectedJobComponents, actualJobComponents)
}

func (s *buildTestSuite) Test_BuildChangedComponents() {
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
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("comp-changed").WithDockerfileName("comp-changed.Dockerfile"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("comp-new").WithDockerfileName("comp-new.Dockerfile"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("comp-unchanged").WithDockerfileName("comp-unchanged.Dockerfile"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("comp-common1-changed").WithDockerfileName("common1.Dockerfile"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("comp-common2-unchanged").WithDockerfileName("common2.Dockerfile"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("comp-common3-changed").WithDockerfileName("common3.Dockerfile"),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("comp-deployonly").WithImage("comp-deployonly:anytag"),
		).
		WithJobComponents(
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("job-changed").WithDockerfileName("job-changed.Dockerfile"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("job-new").WithDockerfileName("job-new.Dockerfile"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("job-unchanged").WithDockerfileName("job-unchanged.Dockerfile"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("job-common1-unchanged").WithDockerfileName("common1.Dockerfile"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("job-common2-changed").WithDockerfileName("common2.Dockerfile"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("job-common3-changed").WithDockerfileName("common3.Dockerfile"),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("job-deployonly").WithImage("job-deployonly:anytag"),
		).
		BuildRA()
	currentRd := utils.NewDeploymentBuilder().
		WithDeploymentName("currentrd").
		WithAppName(appName).
		WithEnvironment(envName).
		WithAnnotations(map[string]string{kube.RadixConfigHash: s.getRadixApplicationHash(ra)}).
		WithCondition(radixv1.DeploymentActive).
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp-changed").WithImage("comp-changed-current:anytag"),
			utils.NewDeployComponentBuilder().WithName("comp-unchanged").WithImage("comp-unchanged-current:anytag"),
			utils.NewDeployComponentBuilder().WithName("comp-common1-changed").WithImage("comp-common1-changed:anytag"),
			utils.NewDeployComponentBuilder().WithName("comp-common2-unchanged").WithImage("comp-common2-unchanged:anytag"),
		).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().WithName("job-changed").WithImage("job-changed-current:anytag"),
			utils.NewDeployJobComponentBuilder().WithName("job-unchanged").WithImage("job-unchanged-current:anytag"),
			utils.NewDeployJobComponentBuilder().WithName("job-common1-unchanged").WithImage("job-common1-unchanged:anytag"),
			utils.NewDeployJobComponentBuilder().WithName("job-common2-changed").WithImage("job-common2-changed:anytag"),
		).
		BuildRD()
	_, _ = s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, envName)).Create(context.Background(), currentRd, metav1.CreateOptions{})

	buildCtx := &model.PrepareBuildContext{
		EnvironmentsToBuild: []model.EnvironmentToBuild{
			{Environment: envName, Components: []string{"comp-changed", "comp-common1-changed", "comp-common3-changed", "job-changed", "job-common2-changed", "job-common3-changed"}},
		},
	}
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra, buildCtx))
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
	imageNameFunc := func(s string) string {
		return fmt.Sprintf("%s/%s-%s:%s", pipeline.PipelineArguments.ContainerRegistry, appName, s, pipeline.PipelineArguments.ImageTag)
	}
	expectedJobContainers := []string{
		"build-comp-changed",
		"build-comp-new",
		"build-job-changed",
		"build-job-new",
		"build-comp-common1-changed",
		"build-job-common2-changed",
		"build-multi-component",
	}
	actualJobContainers := slice.Map(job.Spec.Template.Spec.Containers, func(c corev1.Container) string { return c.Name })
	s.ElementsMatch(expectedJobContainers, actualJobContainers)

	// Check RadixDeployment component and job images
	rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, envName)).List(context.Background(), metav1.ListOptions{LabelSelector: labels.ForPipelineJobName(rjName).String()})
	s.Require().Len(rds.Items, 1)
	rd := rds.Items[0]
	type deployComponentSpec struct {
		Name  string
		Image string
	}
	expectedDeployComponents := []deployComponentSpec{
		{Name: "comp-changed", Image: imageNameFunc("comp-changed")},
		{Name: "comp-new", Image: imageNameFunc("comp-new")},
		{Name: "comp-unchanged", Image: "comp-unchanged-current:anytag"},
		{Name: "comp-deployonly", Image: "comp-deployonly:anytag"},
		{Name: "comp-common1-changed", Image: imageNameFunc("comp-common1-changed")},
		{Name: "comp-common2-unchanged", Image: "comp-common2-unchanged:anytag"},
		{Name: "comp-common3-changed", Image: imageNameFunc("multi-component")},
	}
	actualDeployComponents := slice.Map(rd.Spec.Components, func(c radixv1.RadixDeployComponent) deployComponentSpec {
		return deployComponentSpec{Name: c.Name, Image: c.Image}
	})
	s.ElementsMatch(expectedDeployComponents, actualDeployComponents)
	expectedJobComponents := []deployComponentSpec{
		{Name: "job-changed", Image: imageNameFunc("job-changed")},
		{Name: "job-new", Image: imageNameFunc("job-new")},
		{Name: "job-unchanged", Image: "job-unchanged-current:anytag"},
		{Name: "job-deployonly", Image: "job-deployonly:anytag"},
		{Name: "job-common1-unchanged", Image: "job-common1-unchanged:anytag"},
		{Name: "job-common2-changed", Image: imageNameFunc("job-common2-changed")},
		{Name: "job-common3-changed", Image: imageNameFunc("multi-component")},
	}
	actualJobComponents := slice.Map(rd.Spec.Jobs, func(c radixv1.RadixDeployJobComponent) deployComponentSpec {
		return deployComponentSpec{Name: c.Name, Image: c.Image}
	})
	s.ElementsMatch(expectedJobComponents, actualJobComponents)
}

func (s *buildTestSuite) Test_DetectComponentsToBuild() {
	appName, envName, rjName, buildBranch, jobPort := "anyapp", "dev", "anyrj", "anybranch", pointers.Ptr[int32](9999)
	prepareConfigMapName := "preparecm"
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	rj := utils.ARadixBuildDeployJob().WithJobName(rjName).WithAppName(appName).BuildRJ()
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment(envName, buildBranch).
		WithComponents(
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("comp").WithDockerfileName("comp.Dockerfile"),
		).
		WithJobComponents(
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("job").WithDockerfileName("job.Dockerfile"),
		).
		BuildRA()
	radixDeploymentFactory := func(annotations map[string]string, condition radixv1.RadixDeployCondition, componentBuilders []utils.DeployComponentBuilder, jobBuilders []utils.DeployJobComponentBuilder) *radixv1.RadixDeployment {
		builder := utils.NewDeploymentBuilder().
			WithDeploymentName("currentrd").
			WithAppName(appName).
			WithEnvironment(envName).
			WithAnnotations(annotations).
			WithCondition(condition).
			WithActiveFrom(time.Now().Add(-1 * time.Hour)).
			WithComponents(componentBuilders...).
			WithJobComponents(jobBuilders...)

		if condition == radixv1.DeploymentInactive {
			builder = builder.WithActiveTo(time.Now().Add(1 * time.Hour))
		}

		return builder.BuildRD()
	}
	piplineArgs := model.PipelineArguments{
		PipelineType:      "build-deploy",
		Branch:            buildBranch,
		JobName:           rjName,
		ImageTag:          "imgtag",
		ContainerRegistry: "registry",
		Clustertype:       "clustertype",
		Clustername:       "clustername",
	}
	imageNameFunc := func(s string) string {
		return fmt.Sprintf("%s/%s-%s:%s", piplineArgs.ContainerRegistry, appName, s, piplineArgs.ImageTag)
	}
	type deployComponentSpec struct {
		Name  string
		Image string
	}
	type testSpec struct {
		name                     string
		existingRd               *radixv1.RadixDeployment
		prepareBuildCtx          *model.PrepareBuildContext
		expectedJobContainers    []string
		expectedDeployComponents []deployComponentSpec
		expectedDeployJobs       []deployComponentSpec
	}
	tests := []testSpec{
		{
			name: "radixconfig hash unchanged, component changed, job changed - build all",
			existingRd: radixDeploymentFactory(
				map[string]string{kube.RadixConfigHash: s.getRadixApplicationHash(ra)},
				radixv1.DeploymentActive,
				[]utils.DeployComponentBuilder{utils.NewDeployComponentBuilder().WithName("comp").WithImage("comp:anytag")},
				[]utils.DeployJobComponentBuilder{utils.NewDeployJobComponentBuilder().WithName("job").WithImage("job-changed-current:anytag")},
			),
			prepareBuildCtx: &model.PrepareBuildContext{
				EnvironmentsToBuild: []model.EnvironmentToBuild{
					{
						Environment: envName,
						Components:  []string{"comp", "job"},
					},
				},
			},
			expectedJobContainers:    []string{"build-comp", "build-job"},
			expectedDeployComponents: []deployComponentSpec{{Name: "comp", Image: imageNameFunc("comp")}},
			expectedDeployJobs:       []deployComponentSpec{{Name: "job", Image: imageNameFunc("job")}},
		},
		{
			name: "radixconfig hash unchanged, component changed - build component",
			existingRd: radixDeploymentFactory(
				map[string]string{kube.RadixConfigHash: s.getRadixApplicationHash(ra)},
				radixv1.DeploymentActive,
				[]utils.DeployComponentBuilder{utils.NewDeployComponentBuilder().WithName("comp").WithImage("comp-current:anytag")},
				[]utils.DeployJobComponentBuilder{utils.NewDeployJobComponentBuilder().WithName("job").WithImage("job-current:anytag")},
			),
			prepareBuildCtx: &model.PrepareBuildContext{
				EnvironmentsToBuild: []model.EnvironmentToBuild{
					{
						Environment: envName,
						Components:  []string{"comp"},
					},
				},
			},
			expectedJobContainers:    []string{"build-comp"},
			expectedDeployComponents: []deployComponentSpec{{Name: "comp", Image: imageNameFunc("comp")}},
			expectedDeployJobs:       []deployComponentSpec{{Name: "job", Image: "job-current:anytag"}},
		},
		{
			name: "radixconfig hash unchanged, job changed - build job",
			existingRd: radixDeploymentFactory(
				map[string]string{kube.RadixConfigHash: s.getRadixApplicationHash(ra)},
				radixv1.DeploymentActive,
				[]utils.DeployComponentBuilder{utils.NewDeployComponentBuilder().WithName("comp").WithImage("comp-current:anytag")},
				[]utils.DeployJobComponentBuilder{utils.NewDeployJobComponentBuilder().WithName("job").WithImage("job-current:anytag")},
			),
			prepareBuildCtx: &model.PrepareBuildContext{
				EnvironmentsToBuild: []model.EnvironmentToBuild{
					{
						Environment: envName,
						Components:  []string{"job"},
					},
				},
			},
			expectedJobContainers:    []string{"build-job"},
			expectedDeployComponents: []deployComponentSpec{{Name: "comp", Image: "comp-current:anytag"}},
			expectedDeployJobs:       []deployComponentSpec{{Name: "job", Image: imageNameFunc("job")}},
		},
		{
			name: "radixconfig hash unchanged, component unchanged, job unchanged - no build job",
			existingRd: radixDeploymentFactory(
				map[string]string{kube.RadixConfigHash: s.getRadixApplicationHash(ra)},
				radixv1.DeploymentActive,
				[]utils.DeployComponentBuilder{utils.NewDeployComponentBuilder().WithName("comp").WithImage("comp-current:anytag")},
				[]utils.DeployJobComponentBuilder{utils.NewDeployJobComponentBuilder().WithName("job").WithImage("job-current:anytag")},
			),
			prepareBuildCtx: &model.PrepareBuildContext{
				EnvironmentsToBuild: []model.EnvironmentToBuild{
					{
						Environment: envName,
						Components:  []string{},
					},
				},
			},
			expectedJobContainers:    []string{},
			expectedDeployComponents: []deployComponentSpec{{Name: "comp", Image: "comp-current:anytag"}},
			expectedDeployJobs:       []deployComponentSpec{{Name: "job", Image: "job-current:anytag"}},
		},
		{
			name: "radixconfig hash unchanged, missing prepare context for environment - build all",
			existingRd: radixDeploymentFactory(
				map[string]string{kube.RadixConfigHash: s.getRadixApplicationHash(ra)},
				radixv1.DeploymentActive,
				[]utils.DeployComponentBuilder{utils.NewDeployComponentBuilder().WithName("comp").WithImage("comp-current:anytag")},
				[]utils.DeployJobComponentBuilder{utils.NewDeployJobComponentBuilder().WithName("job").WithImage("job-current:anytag")},
			),
			prepareBuildCtx: &model.PrepareBuildContext{
				EnvironmentsToBuild: []model.EnvironmentToBuild{
					{
						Environment: "otherenv",
						Components:  []string{},
					},
				},
			},
			expectedJobContainers:    []string{"build-comp", "build-job"},
			expectedDeployComponents: []deployComponentSpec{{Name: "comp", Image: imageNameFunc("comp")}},
			expectedDeployJobs:       []deployComponentSpec{{Name: "job", Image: imageNameFunc("job")}},
		},
		{
			name: "radixconfig hash unchanged, component unchanged, job unchanged - no build job",
			existingRd: radixDeploymentFactory(
				map[string]string{kube.RadixConfigHash: s.getRadixApplicationHash(ra)},
				radixv1.DeploymentActive,
				[]utils.DeployComponentBuilder{utils.NewDeployComponentBuilder().WithName("comp").WithImage("comp-current:anytag")},
				[]utils.DeployJobComponentBuilder{utils.NewDeployJobComponentBuilder().WithName("job").WithImage("job-current:anytag")},
			),
			prepareBuildCtx: &model.PrepareBuildContext{
				EnvironmentsToBuild: []model.EnvironmentToBuild{
					{
						Environment: envName,
						Components:  []string{},
					},
				},
			},
			expectedJobContainers:    []string{},
			expectedDeployComponents: []deployComponentSpec{{Name: "comp", Image: "comp-current:anytag"}},
			expectedDeployJobs:       []deployComponentSpec{{Name: "job", Image: "job-current:anytag"}},
		},
		{
			name: "radixconfig hash changed, component unchanged, job unchanged - build all",
			existingRd: radixDeploymentFactory(
				map[string]string{kube.RadixConfigHash: "sha256=0000000000000000000000000000000000000000000000000000000000000000"},
				radixv1.DeploymentActive,
				[]utils.DeployComponentBuilder{utils.NewDeployComponentBuilder().WithName("comp").WithImage("comp-current:anytag")},
				[]utils.DeployJobComponentBuilder{utils.NewDeployJobComponentBuilder().WithName("job").WithImage("job-current:anytag")},
			),
			prepareBuildCtx: &model.PrepareBuildContext{
				EnvironmentsToBuild: []model.EnvironmentToBuild{
					{
						Environment: envName,
						Components:  []string{},
					},
				},
			},
			expectedJobContainers:    []string{"build-comp", "build-job"},
			expectedDeployComponents: []deployComponentSpec{{Name: "comp", Image: imageNameFunc("comp")}},
			expectedDeployJobs:       []deployComponentSpec{{Name: "job", Image: imageNameFunc("job")}},
		},
		{
			name: "radixconfig hash missing, component unchanged, job unchanged - build all",
			existingRd: radixDeploymentFactory(
				nil,
				radixv1.DeploymentActive,
				[]utils.DeployComponentBuilder{utils.NewDeployComponentBuilder().WithName("comp").WithImage("comp-current:anytag")},
				[]utils.DeployJobComponentBuilder{utils.NewDeployJobComponentBuilder().WithName("job").WithImage("job-current:anytag")},
			),
			prepareBuildCtx: &model.PrepareBuildContext{
				EnvironmentsToBuild: []model.EnvironmentToBuild{
					{
						Environment: envName,
						Components:  []string{},
					},
				},
			},
			expectedJobContainers:    []string{"build-comp", "build-job"},
			expectedDeployComponents: []deployComponentSpec{{Name: "comp", Image: imageNameFunc("comp")}},
			expectedDeployJobs:       []deployComponentSpec{{Name: "job", Image: imageNameFunc("job")}},
		},
		{
			name: "missing current RD, component unchanged, job unchanged - build all",
			prepareBuildCtx: &model.PrepareBuildContext{
				EnvironmentsToBuild: []model.EnvironmentToBuild{
					{
						Environment: envName,
						Components:  []string{},
					},
				},
			},
			expectedJobContainers:    []string{"build-comp", "build-job"},
			expectedDeployComponents: []deployComponentSpec{{Name: "comp", Image: imageNameFunc("comp")}},
			expectedDeployJobs:       []deployComponentSpec{{Name: "job", Image: imageNameFunc("job")}},
		},
		{
			name: "no current RD, component unchanged, job unchanged - build all",
			existingRd: radixDeploymentFactory(
				map[string]string{kube.RadixConfigHash: s.getRadixApplicationHash(ra)},
				radixv1.DeploymentInactive,
				[]utils.DeployComponentBuilder{utils.NewDeployComponentBuilder().WithName("comp").WithImage("comp-current:anytag")},
				[]utils.DeployJobComponentBuilder{utils.NewDeployJobComponentBuilder().WithName("job").WithImage("job-current:anytag")},
			),
			prepareBuildCtx: &model.PrepareBuildContext{
				EnvironmentsToBuild: []model.EnvironmentToBuild{
					{
						Environment: envName,
						Components:  []string{},
					},
				},
			},
			expectedJobContainers:    []string{"build-comp", "build-job"},
			expectedDeployComponents: []deployComponentSpec{{Name: "comp", Image: imageNameFunc("comp")}},
			expectedDeployJobs:       []deployComponentSpec{{Name: "job", Image: imageNameFunc("job")}},
		},
	}

	for _, test := range tests {

		s.Run(test.name, func() {
			_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
			_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(context.Background(), rj, metav1.CreateOptions{})
			if test.existingRd != nil {
				_, _ = s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, envName)).Create(context.Background(), test.existingRd, metav1.CreateOptions{})
			}
			s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra, test.prepareBuildCtx))
			pipeline := model.PipelineInfo{PipelineArguments: piplineArgs, RadixConfigMapName: prepareConfigMapName}
			applyStep := steps.NewApplyConfigStep()
			applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
			jobWaiter := pipelinewait.NewMockJobCompletionWaiter(s.ctrl)
			if len(test.expectedJobContainers) > 0 {
				jobWaiter.EXPECT().Wait(gomock.Any()).Return(nil).Times(1)
			}
			buildStep := steps.NewBuildStep(jobWaiter)
			buildStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
			deployStep := steps.NewDeployStep(FakeNamespaceWatcher{})
			deployStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

			// Run pipeline steps
			s.Require().NoError(applyStep.Run(&pipeline))
			s.Require().NoError(buildStep.Run(&pipeline))
			s.Require().NoError(deployStep.Run(&pipeline))

			// Check Job containers
			jobs, _ := s.kubeClient.BatchV1().Jobs(utils.GetAppNamespace(appName)).List(context.Background(), metav1.ListOptions{})
			s.Require().Equal(len(test.expectedJobContainers) > 0, len(jobs.Items) == 1)
			if len(test.expectedJobContainers) > 0 {
				job := jobs.Items[0]
				actualJobContainers := slice.Map(job.Spec.Template.Spec.Containers, func(c corev1.Container) string { return c.Name })
				s.ElementsMatch(test.expectedJobContainers, actualJobContainers)
			}

			// Check RadixDeployment component and job images
			rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, envName)).List(context.Background(), metav1.ListOptions{LabelSelector: labels.ForPipelineJobName(rjName).String()})
			s.Require().Len(rds.Items, 1)
			rd := rds.Items[0]
			actualDeployComponents := slice.Map(rd.Spec.Components, func(c radixv1.RadixDeployComponent) deployComponentSpec {
				return deployComponentSpec{Name: c.Name, Image: c.Image}
			})
			s.ElementsMatch(test.expectedDeployComponents, actualDeployComponents)
			actualJobComponents := slice.Map(rd.Spec.Jobs, func(c radixv1.RadixDeployJobComponent) deployComponentSpec {
				return deployComponentSpec{Name: c.Name, Image: c.Image}
			})
			s.ElementsMatch(test.expectedDeployJobs, actualJobComponents)
		})
	}
}

func (s *buildTestSuite) Test_BuildJobSpec_ImageTagNames() {
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
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("comp1").WithImage("comp1img:{imageTagName}").
				WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment(envName).WithImageTagName("comp1envtag")),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("comp2").WithImage("comp2img:{imageTagName}").
				WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment(envName).WithImageTagName("comp2envtag")),
		).
		WithJobComponents(
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("job1").WithImage("job1img:{imageTagName}").
				WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName).WithImageTagName("job1envtag")),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("job2").WithImage("job2img:{imageTagName}").
				WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName).WithImageTagName("job2envtag")),
		).
		BuildRA()
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra, nil))
	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			PipelineType:  "deploy",
			ToEnvironment: envName,
			JobName:       rjName,
			ImageTagNames: map[string]string{"comp1": "comp1customtag", "job1": "job1customtag"},
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	applyStep := steps.NewApplyConfigStep()
	applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	deployStep := steps.NewDeployStep(FakeNamespaceWatcher{})
	deployStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	s.Require().NoError(applyStep.Run(&pipeline))
	s.Require().NoError(deployStep.Run(&pipeline))

	// Check RadixDeployment component and job images
	rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, envName)).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(rds.Items, 1)
	rd := rds.Items[0]
	type deployComponentSpec struct {
		Name  string
		Image string
	}
	expectedDeployComponents := []deployComponentSpec{
		{Name: "comp1", Image: "comp1img:comp1customtag"},
		{Name: "comp2", Image: "comp2img:comp2envtag"},
	}
	actualDeployComponents := slice.Map(rd.Spec.Components, func(c radixv1.RadixDeployComponent) deployComponentSpec {
		return deployComponentSpec{Name: c.Name, Image: c.Image}
	})
	s.ElementsMatch(expectedDeployComponents, actualDeployComponents)
	expectedJobComponents := []deployComponentSpec{
		{Name: "job1", Image: "job1img:job1customtag"},
		{Name: "job2", Image: "job2img:job2envtag"},
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
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra, nil))
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
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra, nil))
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
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra, nil))
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
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra, nil))
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
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra, nil))
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
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra, nil))
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
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra, nil))
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
	s.Require().NoError(s.createPreparePipelineConfigMapResponse(prepareConfigMapName, appName, ra, nil))
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

func (s *buildTestSuite) createPreparePipelineConfigMapResponse(configMapName, appName string, ra *radixv1.RadixApplication, buildCtx *model.PrepareBuildContext) error {
	raBytes, err := yamlk8s.Marshal(ra)
	if err != nil {
		return err
	}
	data := map[string]string{
		pipelineDefaults.PipelineConfigMapContent: string(raBytes),
	}

	if buildCtx != nil {
		buildCtxBytes, err := yaml.Marshal(buildCtx)
		if err != nil {
			return err
		}
		data[pipelineDefaults.PipelineConfigMapBuildContext] = string(buildCtxBytes)
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: configMapName},
		Data:       data,
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

func (s *buildTestSuite) getRadixApplicationHash(ra *radixv1.RadixApplication) string {
	b, err := yamlk8s.Marshal(&ra.Spec)
	if err != nil {
		return ""
	}

	hashBytes := sha256.Sum256(b)
	return fmt.Sprintf("sha256=%s", hex.EncodeToString(hashBytes[:]))
}
