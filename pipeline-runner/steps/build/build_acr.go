package build

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pipeline-runner/internal/commandbuilder"
	internalgit "github.com/equinor/radix-operator/pipeline-runner/internal/git"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixannotations "github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/rs/zerolog/log"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RadixImageBuilderHomeVolumeName = "radix-image-builder-home"
	BuildKitRunVolumeName           = "build-kit-run"
	BuildKitRootVolumeName          = "build-kit-root"

	buildSecretsMountPath        = "/build-secrets"
	privateImageHubMountPath     = "/radix-private-image-hubs"
	buildahRegistryAuthFile      = "/home/build/auth.json"
	azureServicePrincipleContext = "/radix-image-builder/.azure"
)

func (step *BuildStepImplementation) buildContainerImageBuildingJobs(ctx context.Context, pipelineInfo *model.PipelineInfo, buildSecrets []corev1.EnvVar) ([]*batchv1.Job, error) {
	rr := step.GetRegistration()
	if pipelineInfo.IsUsingBuildKit() {
		return step.buildContainerImageBuildingJobsForBuildKit(ctx, rr, pipelineInfo, buildSecrets)
	}
	return step.buildContainerImageBuildingJobsForACRTasks(ctx, rr, pipelineInfo, buildSecrets)
}

func (step *BuildStepImplementation) buildContainerImageBuildingJobsForACRTasks(ctx context.Context, rr *radixv1.RadixRegistration, pipelineInfo *model.PipelineInfo, buildSecrets []corev1.EnvVar) ([]*batchv1.Job, error) {
	var buildComponentImages []pipeline.BuildComponentImage
	for _, envComponentImages := range pipelineInfo.BuildComponentImages {
		buildComponentImages = append(buildComponentImages, envComponentImages...)
	}

	log.Ctx(ctx).Debug().Msg("build a build-job")
	hash := strings.ToLower(utils.RandStringStrSeed(5, pipelineInfo.PipelineArguments.JobName))
	job, err := buildContainerImageBuildingJob(ctx, rr, pipelineInfo, buildSecrets, hash, &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}, buildComponentImages...)
	if err != nil {
		return nil, err
	}
	return []*batchv1.Job{job}, nil
}

func (step *BuildStepImplementation) buildContainerImageBuildingJobsForBuildKit(ctx context.Context, rr *radixv1.RadixRegistration, pipelineInfo *model.PipelineInfo, buildSecrets []corev1.EnvVar) ([]*batchv1.Job, error) {
	var jobs []*batchv1.Job
	for envName, buildComponentImages := range pipelineInfo.BuildComponentImages {
		log.Ctx(ctx).Debug().Msgf("build a build-kit jobs for the env %s", envName)
		for _, componentImage := range buildComponentImages {
			log.Ctx(ctx).Debug().Msgf("build a job for the image %s", componentImage.ImageName)
			hash := strings.ToLower(utils.RandStringStrSeed(5, fmt.Sprintf("%s-%s-%s", pipelineInfo.PipelineArguments.JobName, envName, componentImage.ComponentName)))

			job, err := buildContainerImageBuildingJob(ctx, rr, pipelineInfo, buildSecrets, hash, componentImage.Runtime, componentImage)
			if err != nil {
				return nil, err
			}

			job.ObjectMeta.Labels[kube.RadixEnvLabel] = envName
			job.ObjectMeta.Labels[kube.RadixComponentLabel] = componentImage.ComponentName
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

func buildContainerImageBuildingJob(ctx context.Context, rr *radixv1.RadixRegistration, pipelineInfo *model.PipelineInfo, buildSecrets []corev1.EnvVar, hash string, jobRuntime *radixv1.Runtime, buildComponentImages ...pipeline.BuildComponentImage) (*batchv1.Job, error) {
	appName := rr.Name
	branch := pipelineInfo.PipelineArguments.Branch
	imageTag := pipelineInfo.PipelineArguments.ImageTag
	pipelineJobName := pipelineInfo.PipelineArguments.JobName
	initContainers, err := git.CloneInitContainers(rr.Spec.CloneURL, branch, internalgit.CloneConfigFromPipelineArgs(pipelineInfo.PipelineArguments))
	if err != nil {
		return nil, err
	}
	buildContainers := createContainerImageBuildingContainers(appName, pipelineInfo, buildComponentImages, buildSecrets)
	timestamp := time.Now().Format("20060102150405")
	defaultMode, backOffLimit := int32(256), int32(0)
	componentImagesAnnotation, _ := json.Marshal(buildComponentImages)
	annotations := radixannotations.ForClusterAutoscalerSafeToEvict(false)
	buildPodSecurityContext := getAcrTaskBuildPodSecurityContext()

	if pipelineInfo.IsUsingBuildKit() {
		for _, buildContainer := range buildContainers {
			annotations[fmt.Sprintf("container.apparmor.security.beta.kubernetes.io/%s", buildContainer.Name)] = "unconfined"
		}
		buildPodSecurityContext = getBuildKitPodSecurityContext()
	}

	buildJobName := fmt.Sprintf("radix-builder-%s-%s-%s", timestamp, imageTag, hash)
	log.Ctx(ctx).Debug().Msgf("build a job %s", buildJobName)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: buildJobName,
			Labels: map[string]string{
				kube.RadixJobNameLabel:  pipelineJobName,
				kube.RadixBuildLabel:    fmt.Sprintf("%s-%s-%s", appName, imageTag, hash),
				kube.RadixAppLabel:      appName,
				kube.RadixImageTagLabel: imageTag,
				kube.RadixJobTypeLabel:  kube.RadixJobTypeBuild,
			},
			Annotations: map[string]string{
				kube.RadixBranchAnnotation:          branch,
				kube.RadixBuildComponentsAnnotation: string(componentImagesAnnotation),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      radixlabels.ForPipelineJobName(pipelineJobName),
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:   "Never",
					InitContainers:  initContainers,
					Containers:      buildContainers,
					SecurityContext: buildPodSecurityContext,
					Volumes:         getContainerImageBuildingJobVolumes(&defaultMode, buildSecrets, pipelineInfo.IsUsingBuildKit(), buildContainers),
					Affinity:        utils.GetAffinityForPipelineJob(jobRuntime),
					Tolerations:     utils.GetPipelineJobPodSpecTolerations(),
				},
			},
		},
	}
	return job, nil
}

func getContainerImageBuildingJobVolumes(defaultMode *int32, buildSecrets []corev1.EnvVar, isUsingBuildKit bool, containers []corev1.Container) []corev1.Volume {
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
		{
			Name: defaults.PrivateImageHubSecretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: defaults.PrivateImageHubSecretName,
				},
			},
		},
		{
			Name: RadixImageBuilderHomeVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: resource.NewScaledQuantity(5, resource.Mega),
				},
			},
		},
	}

	for _, container := range containers {
		volumes = append(volumes, []corev1.Volume{
			{
				Name: getTmpVolumeNameForContainer(container.Name),
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						SizeLimit: resource.NewScaledQuantity(100, resource.Giga),
					},
				},
			},
			{
				Name: getVarVolumeNameForContainer(container.Name),
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						SizeLimit: resource.NewScaledQuantity(100, resource.Giga),
					},
				},
			},
		}...)
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

	if isUsingBuildKit {
		volumes = append(volumes,
			[]corev1.Volume{
				{
					Name: BuildKitRunVolumeName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							SizeLimit: resource.NewScaledQuantity(100, resource.Giga), // buildah puts container overlays there, which can be as large as several gigabytes
						},
					},
				},
				{
					Name: BuildKitRootVolumeName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							SizeLimit: resource.NewScaledQuantity(100, resource.Giga), // buildah puts container overlays there, which can be as large as several gigabytes
						},
					},
				},
			}...)
	}

	return volumes
}

func createContainerImageBuildingContainers(appName string, pipelineInfo *model.PipelineInfo, buildComponentImages []pipeline.BuildComponentImage, buildSecrets []corev1.EnvVar) []corev1.Container {
	var containers []corev1.Container
	imageTag := pipelineInfo.PipelineArguments.ImageTag
	clusterType := pipelineInfo.PipelineArguments.Clustertype
	clusterName := pipelineInfo.PipelineArguments.Clustername
	containerRegistry := pipelineInfo.PipelineArguments.ContainerRegistry

	imageBuilder := fmt.Sprintf("%s/%s", containerRegistry, pipelineInfo.PipelineArguments.ImageBuilder)
	buildContainerSecContext := getAcrTaskBuildContainerSecurityContext()
	var secretMountsArgsString string
	if pipelineInfo.IsUsingBuildKit() {
		imageBuilder = pipelineInfo.PipelineArguments.BuildKitImageBuilder
		buildContainerSecContext = getBuildKitContainerSecurityContext()
		secretMountsArgsString = getSecretArgs(buildSecrets)
	}

	for _, componentImage := range buildComponentImages {
		// For extra meta information about an image
		clusterTypeImage := utils.GetImagePath(containerRegistry, appName, componentImage.ImageName, fmt.Sprintf("%s-%s", clusterType, imageTag))
		clusterNameImage := utils.GetImagePath(containerRegistry, appName, componentImage.ImageName, fmt.Sprintf("%s-%s", clusterName, imageTag))
		envVars := getContainerEnvVars(appName, pipelineInfo, componentImage, buildSecrets, clusterTypeImage, clusterNameImage)
		command := getContainerCommand(pipelineInfo, containerRegistry, secretMountsArgsString, componentImage, clusterTypeImage, clusterNameImage)
		resources := getContainerResources(pipelineInfo)

		container := corev1.Container{
			Name:            componentImage.ContainerName,
			Image:           imageBuilder,
			Command:         command,
			ImagePullPolicy: corev1.PullAlways,
			Env:             envVars,
			VolumeMounts:    getContainerImageBuildingJobVolumeMounts(buildSecrets, pipelineInfo.IsUsingBuildKit(), componentImage.ContainerName),
			SecurityContext: buildContainerSecContext,
			Resources:       resources,
		}
		containers = append(containers, container)
	}
	return containers
}

func getContainerEnvVars(appName string, pipelineInfo *model.PipelineInfo, componentImage pipeline.BuildComponentImage, buildSecrets []corev1.EnvVar, clusterTypeImage string, clusterNameImage string) []corev1.EnvVar {
	envVars := getStandardEnvVars(appName, pipelineInfo, componentImage, clusterTypeImage, clusterNameImage)
	if pipelineInfo.IsUsingBuildKit() {
		envVars = append(envVars, getBuildKitEnvVars()...)
	}
	envVars = append(envVars, buildSecrets...)
	return envVars
}

func getContainerResources(pipelineInfo *model.PipelineInfo) corev1.ResourceRequirements {
	var resources corev1.ResourceRequirements
	if pipelineInfo.IsUsingBuildKit() {
		resources = corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse(pipelineInfo.PipelineArguments.Builder.ResourcesRequestsCPU),
				corev1.ResourceMemory: resource.MustParse(pipelineInfo.PipelineArguments.Builder.ResourcesRequestsMemory),
			},
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(pipelineInfo.PipelineArguments.Builder.ResourcesLimitsMemory),
			},
		}
	}
	return resources
}

func getContainerCommand(pipelineInfo *model.PipelineInfo, containerRegistry string, secretMountsArgsString string, componentImage pipeline.BuildComponentImage, clusterTypeImage, clusterNameImage string) []string {
	if !pipelineInfo.IsUsingBuildKit() {
		return nil
	}
	cacheImagePath := utils.GetImageCachePath(pipelineInfo.PipelineArguments.AppContainerRegistry, pipelineInfo.RadixApplication.Name)
	useBuildCache := pipelineInfo.RadixApplication.Spec.Build.UseBuildCache == nil || *pipelineInfo.RadixApplication.Spec.Build.UseBuildCache
	if pipelineInfo.PipelineArguments.OverrideUseBuildCache != nil {
		useBuildCache = *pipelineInfo.PipelineArguments.OverrideUseBuildCache
	}
	cacheContainerRegistry := pipelineInfo.PipelineArguments.AppContainerRegistry // Store application cache in the App Registry
	return getBuildahContainerCommand(containerRegistry, secretMountsArgsString, componentImage, clusterTypeImage, clusterNameImage, cacheContainerRegistry, cacheImagePath, useBuildCache, pipelineInfo.PipelineArguments.PushImage)
}

func getBuildKitEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
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
					LocalObjectReference: corev1.LocalObjectReference{Name: defaults.AzureACRTokenPasswordAppRegistrySecretName},
					Key:                  "username",
				},
			},
		},
		{
			Name: "BUILDAH_CACHE_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: defaults.AzureACRTokenPasswordAppRegistrySecretName},
					Key:                  "password",
				},
			},
		},
		{
			Name:  "REGISTRY_AUTH_FILE",
			Value: buildahRegistryAuthFile,
		},
	}
}

func getStandardEnvVars(appName string, pipelineInfo *model.PipelineInfo, componentImage pipeline.BuildComponentImage, clusterTypeImage string, clusterNameImage string) []corev1.EnvVar {
	var push string
	if pipelineInfo.PipelineArguments.PushImage {
		push = "--push"
	}
	var useCache string
	if !pipelineInfo.PipelineArguments.UseCache {
		useCache = "--no-cache"
	}
	containerImageRepositoryName := utils.GetRepositoryName(appName, componentImage.ImageName)
	subscriptionId := pipelineInfo.PipelineArguments.SubscriptionId
	branch := pipelineInfo.PipelineArguments.Branch
	targetEnvs := componentImage.EnvName
	if len(targetEnvs) == 0 {
		targetEnvs = strings.Join(pipelineInfo.TargetEnvironments, ",")
	}
	containerRegistry := pipelineInfo.PipelineArguments.ContainerRegistry
	firstPartContainerRegistry := strings.Split(containerRegistry, ".")[0]
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
			Value: path.Join(azureServicePrincipleContext, "sp_credentials.json"),
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
			Value: pipelineInfo.GitCommitHash,
		},
		{
			Name:  defaults.RadixGitTagsEnvironmentVariable,
			Value: pipelineInfo.GitTags,
		},
	}
	return envVars
}

func getContainerImageBuildingJobVolumeMounts(buildSecrets []corev1.EnvVar, isUsingBuildKit bool, containerName string) []corev1.VolumeMount {
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

	if isUsingBuildKit {
		volumeMounts = append(volumeMounts, []corev1.VolumeMount{
			{
				Name:      BuildKitRunVolumeName, // buildah creates folder container overlays and secrets there
				MountPath: "/run",
				ReadOnly:  false,
			},
			{
				Name:      BuildKitRootVolumeName, // buildah home folder
				MountPath: "/root",
				ReadOnly:  false,
			},
			{
				Name:      defaults.PrivateImageHubSecretName,
				MountPath: privateImageHubMountPath,
				ReadOnly:  true,
			},
			{
				Name:      RadixImageBuilderHomeVolumeName, // the file /radix-private-image-hubs/.dockerconfigjson is copied to auth.json file in the user home folder
				MountPath: "/home/build",
				ReadOnly:  false,
			},
		}...)
	} else {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      RadixImageBuilderHomeVolumeName, // .azure folder is created in the user home folder
			MountPath: "/home/radix-image-builder",
			ReadOnly:  false,
		})
	}
	volumeMounts = append(volumeMounts, []corev1.VolumeMount{
		{
			Name:      getTmpVolumeNameForContainer(containerName), // image-builder creates a script there
			MountPath: "/tmp",
			ReadOnly:  false,
		},
		{
			Name:      getVarVolumeNameForContainer(containerName), // image-builder creates files there
			MountPath: "/var",
			ReadOnly:  false,
		},
	}...)
	if len(buildSecrets) > 0 {
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      defaults.BuildSecretsName,
				MountPath: buildSecretsMountPath,
				ReadOnly:  true,
			})
	}
	return volumeMounts
}

func getTmpVolumeNameForContainer(containerName string) string {
	return fmt.Sprintf("tmp-%s", containerName)
}

func getVarVolumeNameForContainer(containerName string) string {
	return fmt.Sprintf("var-%s", containerName)
}

func getBuildahContainerCommand(containerImageRegistry, secretArgsString string, componentImage pipeline.BuildComponentImage, clusterTypeImageTag, clusterNameImageTag, cacheContainerImageRegistry, cacheImagePath string, useBuildCache, pushImage bool) []string {
	commandList := commandbuilder.NewCommandList()
	commandList.AddStrCmd("mkdir /var/tmp && cp %s %s", path.Join(privateImageHubMountPath, ".dockerconfigjson"), buildahRegistryAuthFile)
	commandList.AddStrCmd("/usr/bin/buildah login --username ${BUILDAH_USERNAME} --password ${BUILDAH_PASSWORD} %s", containerImageRegistry)
	if useBuildCache {
		commandList.AddStrCmd("/usr/bin/buildah login --username ${BUILDAH_CACHE_USERNAME} --password ${BUILDAH_CACHE_PASSWORD} %s", cacheContainerImageRegistry)
	}
	buildah := commandbuilder.NewCommand("/usr/bin/buildah build")
	commandList.AddCmd(buildah)

	context := componentImage.Context
	buildah.
		AddArgf("--storage-driver=overlay").
		AddArgf("--isolation=chroot").
		AddArgf("--jobs 0").
		AddArgf("--ulimit nofile=4096:4096").
		AddArg(secretArgsString).
		AddArgf("--file %s%s", context, componentImage.Dockerfile).
		AddArgf(`--build-arg RADIX_GIT_COMMIT_HASH="${RADIX_GIT_COMMIT_HASH}"`).
		AddArgf(`--build-arg RADIX_GIT_TAGS="${RADIX_GIT_TAGS}"`).
		AddArgf(`--build-arg BRANCH="${BRANCH}"`).
		AddArgf(`--build-arg TARGET_ENVIRONMENTS="${TARGET_ENVIRONMENTS}"`)

	if useBuildCache {
		buildah.
			AddArgf("--layers").
			AddArgf("--cache-to=%s", cacheImagePath).
			AddArgf("--cache-from=%s", cacheImagePath)
	}

	imageTag := componentImage.ImagePath
	if pushImage {
		buildah.
			AddArgf("--tag %s", imageTag).
			AddArgf("--tag %s", clusterTypeImageTag).
			AddArgf("--tag %s", clusterNameImageTag)
	}

	buildah.AddArg(context)

	if pushImage {
		commandList.
			AddStrCmd("/usr/bin/buildah push --storage-driver=overlay %s", imageTag).
			AddStrCmd("/usr/bin/buildah push --storage-driver=overlay %s", clusterTypeImageTag).
			AddStrCmd("/usr/bin/buildah push --storage-driver=overlay %s", clusterNameImageTag)
	}

	return []string{"/bin/bash", "-c", commandList.String()}
}

func getAcrTaskBuildPodSecurityContext() *corev1.PodSecurityContext {
	return securitycontext.Pod(
		securitycontext.WithPodFSGroup(1000),
		securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault))
}

func getBuildKitPodSecurityContext() *corev1.PodSecurityContext {
	return securitycontext.Pod(
		securitycontext.WithPodFSGroup(1000),
		securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault),
		securitycontext.WithPodRunAsNonRoot(pointers.Ptr(false)))
}

func getAcrTaskBuildContainerSecurityContext() *corev1.SecurityContext {
	return securitycontext.Container(
		securitycontext.WithContainerDropAllCapabilities(),
		securitycontext.WithContainerSeccompProfileType(corev1.SeccompProfileTypeRuntimeDefault),
		securitycontext.WithContainerRunAsUser(1000),
		securitycontext.WithContainerRunAsGroup(1000),
		securitycontext.WithReadOnlyRootFileSystem(pointers.Ptr(true)),
	)
}

func getBuildKitContainerSecurityContext() *corev1.SecurityContext {
	return securitycontext.Container(
		securitycontext.WithContainerDropAllCapabilities(),
		securitycontext.WithContainerCapabilities([]corev1.Capability{"SETUID", "SETGID", "SETFCAP"}),
		securitycontext.WithContainerSeccompProfile(corev1.SeccompProfile{
			Type:             corev1.SeccompProfileTypeLocalhost,
			LocalhostProfile: utils.StringPtr("allow-buildah.json"),
		}),
		securitycontext.WithContainerRunAsNonRoot(pointers.Ptr(false)),
		securitycontext.WithReadOnlyRootFileSystem(pointers.Ptr(true)),
	)
}

func getSecretArgs(buildSecrets []corev1.EnvVar) string {
	var secretArgs []string
	for _, envVar := range buildSecrets {
		secretArgs = append(secretArgs, fmt.Sprintf("--secret id=%s,src=%s/%s", envVar.ValueFrom.SecretKeyRef.Key, buildSecretsMountPath, envVar.ValueFrom.SecretKeyRef.Key))
	}
	return strings.Join(secretArgs, " ")
}
