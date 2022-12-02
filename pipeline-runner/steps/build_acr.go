package steps

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type void struct{}

var member void

func createACRBuildJob(rr *v1.RadixRegistration, pipelineInfo *model.PipelineInfo, buildSecrets []corev1.EnvVar) (*batchv1.Job, error) {
	appName := rr.Name
	branch := pipelineInfo.PipelineArguments.Branch
	imageTag := pipelineInfo.PipelineArguments.ImageTag
	jobName := pipelineInfo.PipelineArguments.JobName

	initContainers := git.CloneInitContainers(rr.Spec.CloneURL, branch, pipelineInfo.PipelineArguments.ContainerSecurityContext)
	buildContainers := createACRBuildContainers(appName, pipelineInfo, buildSecrets)
	timestamp := time.Now().Format("20060102150405")
	defaultMode, backOffLimit := int32(256), int32(0)

	componentImagesAnnotation, _ := json.Marshal(pipelineInfo.ComponentImages)
	hash := strings.ToLower(utils.RandStringStrSeed(5, pipelineInfo.PipelineArguments.JobName))

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("radix-builder-%s-%s-%s", timestamp, imageTag, hash),
			Labels: map[string]string{
				kube.RadixJobNameLabel:  jobName,
				kube.RadixBuildLabel:    fmt.Sprintf("%s-%s-%s", appName, imageTag, hash),
				kube.RadixAppLabel:      appName,
				kube.RadixImageTagLabel: imageTag,
				kube.RadixJobTypeLabel:  kube.RadixJobTypeBuild,
			},
			Annotations: map[string]string{
				kube.RadixBranchAnnotation:          branch,
				kube.RadixComponentImagesAnnotation: string(componentImagesAnnotation),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						kube.RadixJobNameLabel: jobName,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:   "Never",
					InitContainers:  initContainers,
					Containers:      buildContainers,
					SecurityContext: &pipelineInfo.PipelineArguments.PodSecurityContext,
					Volumes: []corev1.Volume{
						{
							Name: git.BuildContextVolumeName,
						},
						{
							Name: git.GitSSHKeyVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  git.GitSSHKeyVolumeName,
									DefaultMode: &defaultMode,
								},
							},
						},
						{
							Name: defaults.AzureACRServicePrincipleSecretName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: defaults.AzureACRServicePrincipleSecretName,
								},
							},
						},
					},
				},
			},
		},
	}
	return &job, nil
}

func createACRBuildContainers(appName string, pipelineInfo *model.PipelineInfo, buildSecrets []corev1.EnvVar) []corev1.Container {
	imageTag := pipelineInfo.PipelineArguments.ImageTag
	pushImage := pipelineInfo.PipelineArguments.PushImage

	imageBuilder := pipelineInfo.PipelineArguments.ImageBuilder
	clusterType := pipelineInfo.PipelineArguments.Clustertype
	clusterName := pipelineInfo.PipelineArguments.Clustername
	containerRegistry := pipelineInfo.PipelineArguments.ContainerRegistry
	subscriptionId := pipelineInfo.PipelineArguments.SubscriptionId
	branch := pipelineInfo.PipelineArguments.Branch
	targetEnvs := strings.Join(getTargetEnvsToBuild(pipelineInfo), ",")

	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags

	var containers []corev1.Container
	azureServicePrincipleContext := "/radix-image-builder/.azure"
	firstPartContainerRegistry := strings.Split(containerRegistry, ".")[0]
	var push string
	var useCache string
	var useBuildKit string
	if pushImage {
		push = "--push"
	}
	if !pipelineInfo.PipelineArguments.UseCache {
		useCache = "--no-cache"
	}
	if pipelineInfo.RadixApplication.Spec.Build != nil && pipelineInfo.RadixApplication.Spec.Build.UseBuildKit != nil && *pipelineInfo.RadixApplication.Spec.Build.UseBuildKit {
		useBuildKit = "1"
	}
	distinctBuildContainers := make(map[string]void)
	for _, componentImage := range pipelineInfo.ComponentImages {
		if !componentImage.Build {
			// Nothing to build
			continue
		}

		if _, exists := distinctBuildContainers[componentImage.ContainerName]; exists {
			// We already have a container for this multi-component
			continue
		}

		distinctBuildContainers[componentImage.ContainerName] = member

		// For extra meta information about an image
		clusterTypeImage := utils.GetImagePath(containerRegistry, appName, componentImage.ImageName, fmt.Sprintf("%s-%s", clusterType, imageTag))
		clusterNameImage := utils.GetImagePath(containerRegistry, appName, componentImage.ImageName, fmt.Sprintf("%s-%s", clusterName, imageTag))
		containerImageRepositoryName := utils.GetRepositoryName(appName, componentImage.ImageName)

		envVars := []corev1.EnvVar{
			{
				Name:  "DOCKER_FILE_NAME",
				Value: componentImage.Dockerfile,
			},
			{
				Name:  "DOCKER_REGISTRY",
				Value: firstPartContainerRegistry,
			},
			{
				Name:  "IMAGE",
				Value: componentImage.ImagePath,
			},
			{
				Name:  "CONTEXT",
				Value: componentImage.Context,
			},
			{
				Name:  "PUSH",
				Value: push,
			},
			{
				Name:  "AZURE_CREDENTIALS",
				Value: fmt.Sprintf("%s/sp_credentials.json", azureServicePrincipleContext),
			},
			{
				Name:  "SUBSCRIPTION_ID",
				Value: subscriptionId,
			},
			{
				Name:  "CLUSTERTYPE_IMAGE",
				Value: clusterTypeImage,
			},
			{
				Name:  "CLUSTERNAME_IMAGE",
				Value: clusterNameImage,
			},
			{
				Name:  "REPOSITORY_NAME",
				Value: containerImageRepositoryName,
			},
			{
				Name:  "CACHE",
				Value: useCache,
			},
			{
				Name:  defaults.RadixZoneEnvironmentVariable,
				Value: pipelineInfo.PipelineArguments.RadixZone,
			},
			{
				Name:  defaults.UseBuildKitEnvironmentVariable,
				Value: useBuildKit,
			},
			// Extra meta information
			{
				Name:  defaults.RadixBranchEnvironmentVariable,
				Value: branch,
			},
			{
				Name:  defaults.RadixPipelineTargetEnvironmentsVariable,
				Value: targetEnvs,
			},
			{
				Name:  defaults.RadixCommitHashEnvironmentVariable,
				Value: gitCommitHash,
			},
			{
				Name:  defaults.RadixGitTagsEnvironmentVariable,
				Value: gitTags,
			},
		}

		envVars = append(envVars, buildSecrets...)
		imageBuilder := fmt.Sprintf("%s/%s", containerRegistry, imageBuilder)

		container := corev1.Container{
			Name:            componentImage.ContainerName,
			Image:           imageBuilder,
			ImagePullPolicy: corev1.PullAlways,
			Env:             envVars,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      git.BuildContextVolumeName,
					MountPath: git.Workspace,
				},
				{
					Name:      defaults.AzureACRServicePrincipleSecretName,
					MountPath: azureServicePrincipleContext,
					ReadOnly:  true,
				},
			},
			SecurityContext: &pipelineInfo.PipelineArguments.ContainerSecurityContext,
		}
		containers = append(containers, container)
	}

	return containers
}

func getTargetEnvsToBuild(pipelineInfo *model.PipelineInfo) []string {
	var envs []string
	for env, toBuild := range pipelineInfo.TargetEnvironments {
		if toBuild {
			envs = append(envs, env)
		}
	}
	return envs
}
