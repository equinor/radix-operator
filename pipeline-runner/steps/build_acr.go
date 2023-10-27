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
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixannotations "github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/radix-operator/common/commandbuilder"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const buildSecretsMountPath = "/build-secrets"

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
	annotations := radixannotations.ForClusterAutoscalerSafeToEvict(false)
	buildPodSecurityContext := &pipelineInfo.PipelineArguments.PodSecurityContext
	if isUsingBuildKit(pipelineInfo) {
		for _, buildContainer := range buildContainers {
			annotations[fmt.Sprintf("container.apparmor.security.beta.kubernetes.io/%s", buildContainer.Name)] = "unconfined"
		}
		buildPodSecurityContext = &pipelineInfo.PipelineArguments.BuildKitPodSecurityContext
	}

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
					Labels:      radixlabels.ForPipelineJobName(jobName),
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:   "Never",
					InitContainers:  initContainers,
					Containers:      buildContainers,
					SecurityContext: buildPodSecurityContext,
					Volumes:         getACRBuildJobVolumes(&defaultMode, buildSecrets),
					Affinity:        utils.GetPodSpecAffinity(nil, appName, "", false, true),
					Tolerations:     utils.GetPodSpecTolerations(nil, false, true),
				},
			},
		},
	}
	return &job, nil
}

func getACRBuildJobVolumes(defaultMode *int32, buildSecrets []corev1.EnvVar) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: git.BuildContextVolumeName,
		},
		{
			Name: git.GitSSHKeyVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  git.GitSSHKeyVolumeName,
					DefaultMode: defaultMode,
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
	}

	if len(buildSecrets) > 0 {
		volumes = append(volumes,
			corev1.Volume{
				Name: defaults.BuildSecretsName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: defaults.BuildSecretsName,
					},
				},
			})
	}

	return volumes
}

func createACRBuildContainers(appName string, pipelineInfo *model.PipelineInfo, buildSecrets []corev1.EnvVar) []corev1.Container {
	imageTag := pipelineInfo.PipelineArguments.ImageTag
	pushImage := pipelineInfo.PipelineArguments.PushImage
	buildContainerSecContext := &pipelineInfo.PipelineArguments.ContainerSecurityContext
	var containerCommand []string

	clusterType := pipelineInfo.PipelineArguments.Clustertype
	clusterName := pipelineInfo.PipelineArguments.Clustername
	containerRegistry := pipelineInfo.PipelineArguments.ContainerRegistry
	imageBuilder := fmt.Sprintf("%s/%s", containerRegistry, pipelineInfo.PipelineArguments.ImageBuilder)
	subscriptionId := pipelineInfo.PipelineArguments.SubscriptionId
	branch := pipelineInfo.PipelineArguments.Branch
	targetEnvs := strings.Join(getTargetEnvsToBuild(pipelineInfo), ",")
	secretMountsArgsString := ""

	if isUsingBuildKit(pipelineInfo) {
		imageBuilder = pipelineInfo.PipelineArguments.BuildKitImageBuilder
		buildContainerSecContext = getBuildContainerSecContext()
		secretMountsArgsString = getSecretArgs(buildSecrets)
	}

	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags

	var containers []corev1.Container
	azureServicePrincipleContext := "/radix-image-builder/.azure"
	firstPartContainerRegistry := strings.Split(containerRegistry, ".")[0]
	var push string
	var useCache string
	if pushImage {
		push = "--push"
	}
	if !pipelineInfo.PipelineArguments.UseCache {
		useCache = "--no-cache"
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
			// buildah specific env vars
			{
				Name: "BUILDAH_USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: defaults.AzureACRServicePrincipleBuildahSecretName},
						Key:                  "username",
					},
				},
			},
			{
				Name: "BUILDAH_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: defaults.AzureACRServicePrincipleBuildahSecretName},
						Key:                  "password",
					},
				},
			},
			{
				Name: "BUILDAH_CACHE_USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: defaults.AzureACRTokenPasswordBuildahCacheSecretName},
						Key:                  "username",
					},
				},
			},
			{
				Name: "BUILDAH_CACHE_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: defaults.AzureACRTokenPasswordBuildahCacheSecretName},
						Key:                  "password",
					},
				},
			},
		}

		envVars = append(envVars, buildSecrets...)

		container := corev1.Container{
			Name:            componentImage.ContainerName,
			Image:           imageBuilder,
			ImagePullPolicy: corev1.PullAlways,
			Env:             envVars,
			VolumeMounts:    getBuildAcrJobContainerVolumeMounts(azureServicePrincipleContext, buildSecrets),
			SecurityContext: buildContainerSecContext,
		}
		if isUsingBuildKit(pipelineInfo) {
			cacheImagePath := utils.GetImagePathWithoutTag(pipelineInfo.PipelineArguments.CacheContainerRegistry, pipelineInfo.RadixApplication.Name, container.Name)
			useBuildCache := pipelineInfo.RadixApplication.Spec.Build.UseBuildCache == nil || *pipelineInfo.RadixApplication.Spec.Build.UseBuildCache
			cacheContainerRegistry := pipelineInfo.PipelineArguments.CacheContainerRegistry

			containerCommand = getBuildahContainerCommand(containerRegistry, secretMountsArgsString, componentImage.Context, componentImage.Dockerfile, componentImage.ImagePath, clusterTypeImage, clusterNameImage, cacheContainerRegistry, cacheImagePath, useBuildCache, pushImage)
			container.Command = containerCommand
			container.Resources.Requests = map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse(pipelineInfo.PipelineArguments.Builder.ResourcesRequestsCPU),
				corev1.ResourceMemory: resource.MustParse(pipelineInfo.PipelineArguments.Builder.ResourcesRequestsMemory),
			}
			container.Resources.Limits = map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(pipelineInfo.PipelineArguments.Builder.ResourcesLimitsMemory),
			}
		}
		containers = append(containers, container)
	}

	return containers
}

func getBuildAcrJobContainerVolumeMounts(azureServicePrincipleContext string, buildSecrets []corev1.EnvVar) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      git.BuildContextVolumeName,
			MountPath: git.Workspace,
		},
		{
			Name:      defaults.AzureACRServicePrincipleSecretName,
			MountPath: azureServicePrincipleContext,
			ReadOnly:  true,
		},
	}
	if len(buildSecrets) == 0 {
		return volumeMounts
	}
	volumeMounts = append(volumeMounts,
		corev1.VolumeMount{
			Name:      defaults.BuildSecretsName,
			MountPath: buildSecretsMountPath,
			ReadOnly:  true,
		})
	return volumeMounts
}

func getBuildahContainerCommand(containerImageRegistry, secretArgsString, context, dockerFileName, imageTag, clusterTypeImageTag, clusterNameImageTag, cacheContainerImageRegistry, cacheImagePath string, useBuildCache bool) []string {

	commandList := commandbuilder.NewCommandList()

	commandList.AddStrCmd("/usr/bin/buildah login --username ${BUILDAH_USERNAME} --password ${BUILDAH_PASSWORD} %s", containerImageRegistry)
	if useBuildCache {
		commandList.AddStrCmd("/usr/bin/buildah login --username ${BUILDAH_CACHE_USERNAME} --password ${BUILDAH_CACHE_PASSWORD} %s", cacheContainerImageRegistry)
	}

	buildah := commandbuilder.NewCommand("/usr/bin/buildah build")
	commandList.AddCmd(buildah)

	buildah.
		AddArgf("--storage-driver=vfs").
		AddArgf("--isolation=chroot").
		AddArgf("--jobs 0").
		AddArgf(secretArgsString).
		AddArgf("--file %s%s", context, dockerFileName).
		AddArgf("--build-arg RADIX_GIT_COMMIT_HASH=\"${RADIX_GIT_COMMIT_HASH}\"").
		AddArgf("--build-arg RADIX_GIT_TAGS=\"${RADIX_GIT_TAGS}\"").
		AddArgf("--build-arg BRANCH=\"${BRANCH}\"").
		AddArgf("--build-arg TARGET_ENVIRONMENTS=\"${TARGET_ENVIRONMENTS}\"")

	if useBuildCache {
		buildah.
			AddArgf("--layers").
			AddArgf("--cache-to=%s", cacheImagePath).
			AddArgf("--cache-from=%s", cacheImagePath)
	}

	buildah.
		AddArgf("--tag %s", imageTag).
		AddArgf("--tag %s", clusterTypeImageTag).
		AddArgf("--tag %s", clusterNameImageTag).
		AddArgf(context)

	commandList.
		AddStrCmd("/usr/bin/buildah push --storage-driver=vfs %s", imageTag).
		AddStrCmd("/usr/bin/buildah push --storage-driver=vfs %s", clusterTypeImageTag).
		AddStrCmd("/usr/bin/buildah push --storage-driver=vfs %s", clusterNameImageTag)

	return []string{"/bin/bash", "-c", commandList.String()}
}

func isUsingBuildKit(pipelineInfo *model.PipelineInfo) bool {
	return pipelineInfo.RadixApplication.Spec.Build != nil && pipelineInfo.RadixApplication.Spec.Build.UseBuildKit != nil && *pipelineInfo.RadixApplication.Spec.Build.UseBuildKit
}

func getBuildContainerSecContext() *corev1.SecurityContext {
	return securitycontext.Container(
		securitycontext.WithContainerDropAllCapabilities(),
		securitycontext.WithContainerCapabilities([]corev1.Capability{"SETUID", "SETGID", "SETFCAP"}),
		securitycontext.WithContainerSeccompProfile(corev1.SeccompProfile{
			Type:             corev1.SeccompProfileTypeLocalhost,
			LocalhostProfile: utils.StringPtr("allow-buildah.json"),
		}),
		securitycontext.WithContainerRunAsNonRoot(utils.BoolPtr(false)),
	)
}

func getSecretArgs(buildSecrets []corev1.EnvVar) string {
	var secretArgs []string
	for _, envVar := range buildSecrets {
		secretArgs = append(secretArgs, fmt.Sprintf("--secret id=%s,src=%s/%s", envVar.ValueFrom.SecretKeyRef.Key, buildSecretsMountPath, envVar.ValueFrom.SecretKeyRef.Key))
	}
	return strings.Join(secretArgs, " ")
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
