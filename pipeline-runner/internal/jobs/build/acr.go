package build

import (
	"path"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/internal/jobs/build/internal"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	azureServicePrincipleContext = "/radix-image-builder/.azure"
	acrHomeVolumeName            = "radix-image-builder-home"
	acrHomePath                  = "/home/radix-image-builder"
)

// NewBuildKit returns a JobBuilder implementation for building components and jobs using radix-image-builder (https://github.com/equinor/radix-image-builder)
func NewACR() JobsBuilder {
	return &acr{}
}

type acr struct{}

func (c *acr) BuildJobs(_, _ bool, pipelineArgs model.PipelineArguments, cloneURL, gitCommitHash, gitTags string, componentImages []pipeline.BuildComponentImage, buildSecrets []string, appID radixv1.ULID, imagePullSecret string) []batchv1.Job {

	var pullSecrets []corev1.LocalObjectReference
	if imagePullSecret != "" {
		pullSecrets = append(pullSecrets, corev1.LocalObjectReference{Name: imagePullSecret})
	}

	props := &acrKubeJobProps{
		pipelineArgs:     pipelineArgs,
		componentImages:  componentImages,
		cloneURL:         cloneURL,
		gitCommitHash:    gitCommitHash,
		gitTags:          gitTags,
		buildSecrets:     buildSecrets,
		appID:            appID,
		imagePullSecrets: pullSecrets,
	}

	return []batchv1.Job{internal.BuildKubeJob(props)}
}

var _ internal.KubeJobProps = &acrKubeJobProps{}

type acrKubeJobProps struct {
	pipelineArgs     model.PipelineArguments
	componentImages  []pipeline.BuildComponentImage
	cloneURL         string
	gitCommitHash    string
	gitTags          string
	buildSecrets     []string
	appID            radixv1.ULID
	imagePullSecrets []corev1.LocalObjectReference
}

func (c *acrKubeJobProps) JobName() string {
	hash := strings.ToLower(utils.RandStringStrSeed(5, c.pipelineArgs.JobName))
	return getJobName(time.Now(), c.pipelineArgs.ImageTag, hash)
}

func (c *acrKubeJobProps) JobLabels() map[string]string {
	return getCommonJobLabels(c.pipelineArgs.AppName, c.pipelineArgs.JobName, c.pipelineArgs.ImageTag)
}

func (c *acrKubeJobProps) JobAnnotations() map[string]string {
	branch := c.pipelineArgs.Branch //nolint:staticcheck
	return getCommonJobAnnotations(branch, c.pipelineArgs.GitRef, c.pipelineArgs.GitRefType, c.componentImages...)
}

func (c *acrKubeJobProps) PodLabels() map[string]string {
	return getCommonPodLabels(c.pipelineArgs.JobName, c.pipelineArgs.AppName, c.appID)
}

func (c *acrKubeJobProps) PodAnnotations() map[string]string {
	return getCommonPodAnnotations()
}

func (c *acrKubeJobProps) PodTolerations() []corev1.Toleration {
	return getCommonPodTolerations()
}

func (c *acrKubeJobProps) PodAffinity() *corev1.Affinity {
	return getCommonPodAffinity(string(radixv1.RuntimeArchitectureArm64))
}

func (*acrKubeJobProps) PodSecurityContext() *corev1.PodSecurityContext {
	return securitycontext.Pod(
		securitycontext.WithPodFSGroup(1000),
		securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault))
}

func (c *acrKubeJobProps) PodVolumes() []corev1.Volume {
	volumes := getCommonPodVolumes(c.componentImages)

	volumes = append(volumes,
		corev1.Volume{
			Name: defaults.AzureACRServicePrincipleSecretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: defaults.AzureACRServicePrincipleSecretName,
				},
			},
		},
		corev1.Volume{
			Name: acrHomeVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: resource.NewScaledQuantity(5, resource.Mega),
				},
			},
		},
	)

	return volumes
}

func (c *acrKubeJobProps) PodInitContainers() []corev1.Container {
	return getCommonPodInitContainers(c.cloneURL, c.pipelineArgs.GetGitRefOrDefault(), c.gitCommitHash, c.pipelineArgs.GitWorkspace, c.pipelineArgs.GitCloneGitImage)
}

func (c *acrKubeJobProps) PodImagePullSecrets() []corev1.LocalObjectReference {
	return c.imagePullSecrets
}

func (c *acrKubeJobProps) PodContainers() []corev1.Container {
	return slice.Map(c.componentImages, c.getPodContainer)
}

func (c *acrKubeJobProps) getPodContainer(componentImage pipeline.BuildComponentImage) corev1.Container {
	return corev1.Container{
		Name:            componentImage.ContainerName,
		Image:           c.pipelineArgs.ImageBuilder,
		ImagePullPolicy: corev1.PullAlways,
		Env:             c.getPodContainerEnvVars(componentImage),
		VolumeMounts:    c.getPodContainerVolumeMounts(componentImage),
		SecurityContext: c.getPodContainerSecurityContext(),
		Resources:       c.getPodContainerResources(),
	}
}

func (*acrKubeJobProps) getPodContainerSecurityContext() *corev1.SecurityContext {
	return securitycontext.Container(
		securitycontext.WithContainerDropAllCapabilities(),
		securitycontext.WithContainerSeccompProfileType(corev1.SeccompProfileTypeRuntimeDefault),
		securitycontext.WithContainerRunAsUser(1000),
		securitycontext.WithContainerRunAsGroup(1000),
		securitycontext.WithReadOnlyRootFileSystem(pointers.Ptr(true)),
	)
}

func (c *acrKubeJobProps) getPodContainerResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse("50m"),
			corev1.ResourceMemory: resource.MustParse("100M"),
		},
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("500M"),
		},
	}
}

func (c *acrKubeJobProps) getPodContainerVolumeMounts(componentImage pipeline.BuildComponentImage) []corev1.VolumeMount {
	volumeMounts := getCommonPodContainerVolumeMounts(componentImage, c.pipelineArgs.GitWorkspace)

	volumeMounts = append(volumeMounts,
		corev1.VolumeMount{
			Name:      defaults.AzureACRServicePrincipleSecretName,
			MountPath: azureServicePrincipleContext,
			ReadOnly:  true,
		},
		// .azure folder is created in the user home folder
		corev1.VolumeMount{
			Name:      acrHomeVolumeName,
			MountPath: acrHomePath,
			ReadOnly:  false,
		},
	)

	return volumeMounts
}

func (c *acrKubeJobProps) getPodContainerEnvVars(componentImage pipeline.BuildComponentImage) []corev1.EnvVar {
	var push string
	if c.pipelineArgs.PushImage {
		push = "--push"
	}
	firstPartContainerRegistry := strings.Split(c.pipelineArgs.ContainerRegistry, ".")[0]
	envVars := []corev1.EnvVar{
		{
			Name:  defaults.RadixBranchEnvironmentVariable,
			Value: c.pipelineArgs.Branch, //nolint:staticcheck
		},
		{
			Name:  defaults.RadixGitRefEnvironmentVariable,
			Value: c.pipelineArgs.GitRef,
		},
		{
			Name:  defaults.RadixGitRefTypeEnvironmentVariable,
			Value: c.pipelineArgs.GitRefType,
		},
		{
			Name:  defaults.RadixPipelineTargetEnvironmentsVariable,
			Value: componentImage.EnvName,
		},
		{
			Name:  defaults.RadixCommitHashEnvironmentVariable,
			Value: c.gitCommitHash,
		},
		{
			Name:  defaults.RadixGitTagsEnvironmentVariable,
			Value: c.gitTags,
		},
		{
			Name:  "AZURE_CREDENTIALS",
			Value: path.Join(azureServicePrincipleContext, "sp_credentials.json"),
		},
		{
			Name:  "SUBSCRIPTION_ID",
			Value: c.pipelineArgs.SubscriptionId,
		},
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
			Name:  "CLUSTERTYPE_IMAGE",
			Value: componentImage.ClusterTypeImagePath,
		},
		{
			Name:  "CLUSTERNAME_IMAGE",
			Value: componentImage.ClusterNameImagePath,
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
			Name:  defaults.RadixZoneEnvironmentVariable,
			Value: c.pipelineArgs.RadixZone,
		},
	}

	envVars = append(envVars, c.getPodContainerBuildSecretEnvVars()...)

	return envVars
}

func (c *acrKubeJobProps) getPodContainerBuildSecretEnvVars() []corev1.EnvVar {
	return slice.Map(c.buildSecrets, func(secret string) corev1.EnvVar {
		return corev1.EnvVar{
			Name: defaults.BuildSecretPrefix + secret,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: defaults.BuildSecretsName,
					},
					Key: secret,
				},
			},
		}
	})
}
