package build

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	internalgit "github.com/equinor/radix-operator/pipeline-runner/internal/git"
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

type ACR struct{}

func (c *ACR) GetJobs(useBuildCache bool, pipelineArgs model.PipelineArguments, cloneURL, gitCommitHash, gitTags string, componentImages []pipeline.BuildComponentImage, buildSecrets []string) []batchv1.Job {
	kubeJob := kubeJobBuilder{
		source: &acrJobSource{
			pipelineArgs:    pipelineArgs,
			componentImages: componentImages,
			cloneURL:        cloneURL,
			gitCommitHash:   gitCommitHash,
			gitTags:         gitTags,
			buildSecrets:    buildSecrets,
		},
	}
	return []batchv1.Job{kubeJob.GetJob()}
}

var _ kubeJobBuilderSource = &acrJobSource{}

type acrJobSource struct {
	pipelineArgs    model.PipelineArguments
	componentImages []pipeline.BuildComponentImage
	cloneURL        string
	gitCommitHash   string
	gitTags         string
	buildSecrets    []string
}

func (c *acrJobSource) JobName() string {
	hash := strings.ToLower(utils.RandStringStrSeed(5, c.pipelineArgs.JobName))
	return getJobName(time.Now(), c.pipelineArgs.ImageTag, hash)
}

func (c *acrJobSource) JobLabels() map[string]string {
	return getCommonJobLabels(c.pipelineArgs.AppName, c.pipelineArgs.JobName, c.pipelineArgs.ImageTag)
}

func (c *acrJobSource) JobAnnotations() map[string]string {
	return getCommonJobAnnotations(c.pipelineArgs.Branch, c.componentImages...)
}

func (c *acrJobSource) PodLabels() map[string]string {
	return getCommonPodLabels(c.pipelineArgs.JobName)
}

func (c *acrJobSource) PodAnnotations() map[string]string {
	return getCommonPodAnnotations()
}

func (c *acrJobSource) PodTolerations() []corev1.Toleration {
	return getCommonPodTolerations()
}

func (c *acrJobSource) PodAffinity() *corev1.Affinity {
	return getCommonPodAffinity(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64})
}

func (*acrJobSource) PodSecurityContext() *corev1.PodSecurityContext {
	return securitycontext.Pod(
		securitycontext.WithPodFSGroup(1000),
		securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault))
}

func (c *acrJobSource) PodVolumes() []corev1.Volume {
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

func (c *acrJobSource) PodInitContainers() []corev1.Container {
	cloneCfg := internalgit.CloneConfigFromPipelineArgs(c.pipelineArgs)
	return getCommonPodInitContainers(c.cloneURL, c.pipelineArgs.Branch, cloneCfg)
}

func (c *acrJobSource) PodContainers() []corev1.Container {
	return slice.Map(c.componentImages, c.getPodContainer)
}

func (c *acrJobSource) getPodContainer(componentImage pipeline.BuildComponentImage) corev1.Container {
	return corev1.Container{
		Name:            componentImage.ContainerName,
		Image:           fmt.Sprintf("%s/%s", c.pipelineArgs.ContainerRegistry, c.pipelineArgs.ImageBuilder),
		ImagePullPolicy: corev1.PullAlways,
		Env:             c.getPodContainerEnvVars(componentImage),
		VolumeMounts:    c.getPodContainerVolumeMounts(componentImage),
		SecurityContext: c.getPodContainerSecurityContext(),
	}
}

func (*acrJobSource) getPodContainerSecurityContext() *corev1.SecurityContext {
	return securitycontext.Container(
		securitycontext.WithContainerDropAllCapabilities(),
		securitycontext.WithContainerSeccompProfileType(corev1.SeccompProfileTypeRuntimeDefault),
		securitycontext.WithContainerRunAsUser(1000),
		securitycontext.WithContainerRunAsGroup(1000),
		securitycontext.WithReadOnlyRootFileSystem(pointers.Ptr(true)),
	)
}

func (c *acrJobSource) getPodContainerVolumeMounts(componentImage pipeline.BuildComponentImage) []corev1.VolumeMount {
	volumeMounts := getCommonPodContainerVolumeMounts(componentImage)

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

func (c *acrJobSource) getPodContainerEnvVars(componentImage pipeline.BuildComponentImage) []corev1.EnvVar {
	envVars := getCommonPodContainerEnvVars(componentImage, c.pipelineArgs.Branch, c.gitCommitHash, c.gitTags)
	envVars = append(envVars, c.getPodContainerBuildSecretEnvVars()...)

	var push string
	if c.pipelineArgs.PushImage {
		push = "--push"
	}
	firstPartContainerRegistry := strings.Split(c.pipelineArgs.ContainerRegistry, ".")[0]
	envVars = append(envVars,
		corev1.EnvVar{
			Name:  "AZURE_CREDENTIALS",
			Value: path.Join(azureServicePrincipleContext, "sp_credentials.json"),
		},
		corev1.EnvVar{
			Name:  "SUBSCRIPTION_ID",
			Value: c.pipelineArgs.SubscriptionId,
		},
		corev1.EnvVar{
			Name:  "DOCKER_FILE_NAME",
			Value: componentImage.Dockerfile,
		},
		corev1.EnvVar{
			Name:  "DOCKER_REGISTRY",
			Value: firstPartContainerRegistry,
		},
		corev1.EnvVar{
			Name:  "IMAGE",
			Value: componentImage.ImagePath,
		},
		corev1.EnvVar{
			Name:  "CLUSTERTYPE_IMAGE",
			Value: componentImage.ClusterTypeImagePath,
		},
		corev1.EnvVar{
			Name:  "CLUSTERNAME_IMAGE",
			Value: componentImage.ClusterNameImagePath,
		},
		corev1.EnvVar{
			Name:  "CONTEXT",
			Value: componentImage.Context,
		},
		corev1.EnvVar{
			Name:  "PUSH",
			Value: push,
		},
		corev1.EnvVar{
			Name:  defaults.RadixZoneEnvironmentVariable,
			Value: c.pipelineArgs.RadixZone,
		},
	)

	return envVars
}

func (c *acrJobSource) getPodContainerBuildSecretEnvVars() []corev1.EnvVar {
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
