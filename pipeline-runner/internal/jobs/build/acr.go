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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	azureServicePrincipleContext = "/radix-image-builder/.azure"
	acrHomeVolumeName            = "radix-image-builder-home"
)

type acrConstructor struct {
	pipelineArgs    model.PipelineArguments
	componentImages []pipeline.BuildComponentImage
	cloneURL        string
	gitCommitHash   string
	gitTags         string
	buildSecrets    []string
}

func (c *acrConstructor) ConstructJobs() ([]batchv1.Job, error) {
	var builder jobBuilder

	initContainers, err := c.getPodInitContainers()
	if err != nil {
		return nil, err
	}

	builder.SetName(c.getJobName())
	builder.SetLabels(c.getJobLabels())
	builder.SetAnnotations(c.getJobAnnotations())
	builder.SetPodLabels(c.getPodLabels())
	builder.SetPodAnnotations(c.getPodAnnotations())
	builder.SetPodTolerations(c.getPodTolerations())
	builder.SetPodAffinity(c.getPodAffinity())
	builder.SetPodSecurityContext(c.getPodSecurityContext())
	builder.SetPodVolumes(c.getPodVolumes())
	builder.SetPodInitContainers(initContainers)
	builder.SetPodContainers(c.getPodContainers())

	return []batchv1.Job{builder.GetJob()}, nil
}

func (c *acrConstructor) getJobName() string {
	return getJobName(time.Now(), c.pipelineArgs.ImageTag, "0")
}

func (c *acrConstructor) getJobLabels() map[string]string {
	return getDefaultJobLabels(c.pipelineArgs.AppName, c.pipelineArgs.JobName, c.pipelineArgs.ImageTag)
}

func (c *acrConstructor) getJobAnnotations() map[string]string {
	return getDefaultJobAnnotations(c.pipelineArgs.Branch, c.componentImages...)
}

func (c *acrConstructor) getPodLabels() map[string]string {
	return getDefaultPodLabels(c.pipelineArgs.JobName)
}

func (c *acrConstructor) getPodAnnotations() map[string]string {
	return getDefaultPodAnnotations()
}

func (c *acrConstructor) getPodTolerations() []corev1.Toleration {
	return getDefaultPodTolerations()
}

func (c *acrConstructor) getPodAffinity() *corev1.Affinity {
	return getDefaultPodAffinity(&radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64})
}

func (*acrConstructor) getPodSecurityContext() *corev1.PodSecurityContext {
	return securitycontext.Pod(
		securitycontext.WithPodFSGroup(1000),
		securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault))
}

func (c *acrConstructor) getPodInitContainers() ([]corev1.Container, error) {
	cloneCfg := internalgit.CloneConfigFromPipelineArgs(c.pipelineArgs)
	return getDefaultPodInitContainers(c.cloneURL, c.pipelineArgs.Branch, cloneCfg)
}

func (c *acrConstructor) getPodContainers() []corev1.Container {
	return slice.Map(c.componentImages, c.getPodContainer)
}

func (c *acrConstructor) getPodContainer(componentImage pipeline.BuildComponentImage) corev1.Container {
	securityContext := securitycontext.Container(
		securitycontext.WithContainerDropAllCapabilities(),
		securitycontext.WithContainerSeccompProfileType(corev1.SeccompProfileTypeRuntimeDefault),
		securitycontext.WithContainerRunAsUser(1000),
		securitycontext.WithContainerRunAsGroup(1000),
		securitycontext.WithReadOnlyRootFileSystem(pointers.Ptr(true)),
	)
	resources := corev1.ResourceRequirements{}

	return corev1.Container{
		Name:            componentImage.ContainerName,
		Image:           fmt.Sprintf("%s/%s", c.pipelineArgs.ContainerRegistry, c.pipelineArgs.ImageBuilder),
		ImagePullPolicy: corev1.PullAlways,
		Env:             c.getPodContainerEnvVars(componentImage),
		VolumeMounts:    c.getPodContainerVolumeMounts(componentImage),
		SecurityContext: securityContext,
		Resources:       resources,
	}
}

func (c *acrConstructor) getPodVolumes() []corev1.Volume {
	volumes := getDefaultPodVolumes(c.componentImages)

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

func (c *acrConstructor) getPodContainerVolumeMounts(componentImage pipeline.BuildComponentImage) []corev1.VolumeMount {
	volumeMounts := getDefaultPodContainerVolumeMounts(componentImage)

	volumeMounts = append(volumeMounts,
		corev1.VolumeMount{
			Name:      defaults.AzureACRServicePrincipleSecretName,
			MountPath: azureServicePrincipleContext,
			ReadOnly:  true,
		},
		corev1.VolumeMount{
			Name:      acrHomeVolumeName, // .azure folder is created in the user home folder
			MountPath: "/home/radix-image-builder",
			ReadOnly:  false,
		},
	)

	return volumeMounts
}

func (c *acrConstructor) getPodContainerEnvVars(componentImage pipeline.BuildComponentImage) []corev1.EnvVar {
	envVars := getDefaultPodContainerEnvVars(componentImage, c.pipelineArgs.Branch, c.gitCommitHash, c.gitTags)
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

func (c *acrConstructor) getPodContainerBuildSecretEnvVars() []corev1.EnvVar {
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
