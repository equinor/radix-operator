package build

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pipeline-runner/internal/jobs/build/internal"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/runtime"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/apis/utils/random"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	buildKitRunVolumeName           = "build-kit-run"
	buildKitRootVolumeName          = "build-kit-root"
	buildKitHomeVolumeName          = "radix-image-builder-home"
	buildKitHomePath                = "/home/build"
	buildKitBuildSecretsPath        = "/build-secrets"
	privateImageHubDockerAuthPath   = "/radix-private-image-hubs"
	defaultExternalRegistryAuthPath = "/radix-default-external-registry-auth"
)

// NewBuildKit returns a JobBuilder implementation for building components and jobs using radix-buildkit-builder (https://github.com/equinor/radix-buildkit-builder)
func NewBuildKit(cfg config.Config, args model.PipelineArguments, rr radixv1.RadixRegistration) JobsBuilder {
	return &buildKit{cfg: cfg, args: args, rr: rr}
}

type buildKit struct {
	cfg  config.Config
	args model.PipelineArguments
	rr   radixv1.RadixRegistration
}

func (c *buildKit) BuildJobs(useBuildCache, refreshBuildCache bool, gitCommitHash, gitTags string, componentImages []pipeline.BuildComponentImage, buildSecrets []string) []batchv1.Job {
	var jobs []batchv1.Job

	for _, componentImage := range componentImages {
		job := c.buildJob(componentImage, useBuildCache, refreshBuildCache, gitCommitHash, gitTags, buildSecrets)
		jobs = append(jobs, job)
	}

	return jobs
}

func (c *buildKit) buildJob(componentImage pipeline.BuildComponentImage, useBuildCache, refreshBuildCache bool, gitCommitHash, gitTags string, buildSecrets []string) batchv1.Job {

	props := &buildKitKubeJobProps{
		cfg:               c.cfg,
		pipelineArgs:      c.args,
		componentImage:    componentImage,
		cloneURL:          c.rr.Spec.CloneURL,
		gitCommitHash:     gitCommitHash,
		gitTags:           gitTags,
		buildSecrets:      buildSecrets,
		useBuildCache:     useBuildCache,
		refreshBuildCache: refreshBuildCache,
		appID:             c.rr.Spec.AppID,
	}

	return internal.BuildKubeJob(props)
}

var _ internal.KubeJobProps = &buildKitKubeJobProps{}

type buildKitKubeJobProps struct {
	cfg               config.Config
	pipelineArgs      model.PipelineArguments
	componentImage    pipeline.BuildComponentImage
	cloneURL          string
	gitCommitHash     string
	gitTags           string
	buildSecrets      []string
	useBuildCache     bool
	refreshBuildCache bool
	appID             radixv1.ULID
}

func (c *buildKitKubeJobProps) JobName() string {
	hash := strings.ToLower(random.RandStringStrSeed(5, fmt.Sprintf("%s-%s-%s", c.pipelineArgs.JobName, c.componentImage.EnvName, c.componentImage.ComponentName)))
	return getJobName(time.Now(), c.pipelineArgs.ImageTag, hash)
}

func (c *buildKitKubeJobProps) JobLabels() map[string]string {
	return labels.Merge(
		getCommonJobLabels(c.pipelineArgs.AppName, c.pipelineArgs.JobName, c.pipelineArgs.ImageTag),
		labels.ForEnvironmentName(c.componentImage.EnvName),
		labels.ForComponentName(c.componentImage.ComponentName),
	)
}

func (c *buildKitKubeJobProps) JobAnnotations() map[string]string {
	branch := c.pipelineArgs.Branch //nolint:staticcheck
	return getCommonJobAnnotations(branch, c.pipelineArgs.GitRef, c.pipelineArgs.GitRefType, c.componentImage)
}

func (c *buildKitKubeJobProps) PodLabels() map[string]string {
	return getCommonPodLabels(c.pipelineArgs.JobName, c.pipelineArgs.AppName, c.appID)
}

func (c *buildKitKubeJobProps) PodAnnotations() map[string]string {
	annotations := getCommonPodAnnotations()
	annotations[fmt.Sprintf("container.apparmor.security.beta.kubernetes.io/%s", c.componentImage.ContainerName)] = "unconfined"
	return annotations
}

func (c *buildKitKubeJobProps) PodTolerations() []corev1.Toleration {
	return getCommonPodTolerations()
}

func (c *buildKitKubeJobProps) PodAffinity() *corev1.Affinity {
	nodeArch := runtime.GetArchitectureFromRuntimeOrDefault(c.componentImage.Runtime)
	return getCommonPodAffinity(nodeArch)
}

func (*buildKitKubeJobProps) PodSecurityContext() *corev1.PodSecurityContext {
	return securitycontext.Pod(
		securitycontext.WithPodFSGroup(1000),
		securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault),
		securitycontext.WithPodRunAsNonRoot(new(false)))
}

func (c *buildKitKubeJobProps) PodImagePullSecrets() []corev1.LocalObjectReference {
	var pullSecrets []corev1.LocalObjectReference
	if c.cfg.Common.ExternalRegistryAuthSecret != "" {
		pullSecrets = append(pullSecrets, corev1.LocalObjectReference{Name: c.cfg.Common.ExternalRegistryAuthSecret})
	}

	return pullSecrets
}

func (c *buildKitKubeJobProps) PodVolumes() []corev1.Volume {
	volumes := getCommonPodVolumes([]pipeline.BuildComponentImage{c.componentImage})

	volumes = append(volumes,
		corev1.Volume{
			Name: defaults.PrivateImageHubSecretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: defaults.PrivateImageHubSecretName,
				},
			},
		},
		corev1.Volume{
			Name: buildKitHomeVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: resource.NewScaledQuantity(5, resource.Mega),
				},
			},
		},
		corev1.Volume{
			Name: buildKitRunVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: resource.NewScaledQuantity(100, resource.Giga), // buildah puts container overlays there, which can be as large as several gigabytes
				},
			},
		},
		corev1.Volume{
			Name: buildKitRootVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: resource.NewScaledQuantity(100, resource.Giga), // buildah puts container overlays there, which can be as large as several gigabytes
				},
			},
		},
	)

	if len(c.cfg.Common.ExternalRegistryAuthSecret) > 0 {
		volumes = append(volumes,
			corev1.Volume{
				Name: c.cfg.Common.ExternalRegistryAuthSecret,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: c.cfg.Common.ExternalRegistryAuthSecret,
					},
				},
			},
		)
	}

	if len(c.buildSecrets) > 0 {
		volumes = append(volumes,
			corev1.Volume{
				Name: defaults.BuildSecretsName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: defaults.BuildSecretsName,
					},
				},
			},
		)
	}

	return volumes
}

func (c *buildKitKubeJobProps) PodInitContainers() []corev1.Container {
	return getCommonPodInitContainers(c.cloneURL, c.pipelineArgs.GetGitRefOrDefault(), c.gitCommitHash, c.pipelineArgs.GitWorkspace, c.cfg.PipelineRunner.GitCloneImage.String())
}

func (c *buildKitKubeJobProps) PodContainers() []corev1.Container {
	container := corev1.Container{
		Name:            c.componentImage.ContainerName,
		Image:           c.cfg.PipelineRunner.Builder.Image.String(),
		ImagePullPolicy: corev1.PullAlways,
		Args:            c.getPodContainerArgs(),
		Env:             c.getPodContainerEnvVars(),
		VolumeMounts:    c.getPodContainerVolumeMounts(),
		SecurityContext: c.getPodContainerSecurityContext(),
		Resources:       c.getPodContainerResources(),
	}

	return []corev1.Container{container}
}

func (c *buildKitKubeJobProps) getPodContainerArgs() []string {
	args := []string{
		"--registry", c.pipelineArgs.ContainerRegistry,
		"--registry-username", "$(BUILDAH_USERNAME)",
		"--registry-password", "$(BUILDAH_PASSWORD)",
		"--cache-registry", c.pipelineArgs.CacheContainerRegistry,
		"--cache-registry-username", "$(BUILDAH_CACHE_USERNAME)",
		"--cache-registry-password", "$(BUILDAH_CACHE_PASSWORD)",
		"--cache-repository", utils.GetImageCachePath(c.pipelineArgs.CacheContainerRegistry, c.pipelineArgs.AppName),
		"--tag", c.componentImage.ImagePath,
		"--cluster-type-tag", c.componentImage.ClusterTypeImagePath,
		"--cluster-name-tag", c.componentImage.ClusterNameImagePath,
		"--secrets-path", buildKitBuildSecretsPath,
		"--dockerfile", c.componentImage.Dockerfile,
		"--context", c.componentImage.Context,
		"--branch", c.pipelineArgs.GetGitRefOrDefault(),
		"--git-commit-hash", c.gitCommitHash,
		"--git-tags", c.gitTags,
		"--target-environments", c.componentImage.EnvName,
	}

	if c.useBuildCache {
		args = append(args, "--use-cache")
	}

	if c.refreshBuildCache {
		args = append(args, "--refresh-cache")
	}

	if c.pipelineArgs.PushImage {
		args = append(args, "--push")
	}

	for _, secret := range c.buildSecrets {
		args = append(args, "--secret", secret)
	}

	// The order of auth-files matters when multiple are defined:
	// When multiple files contains credentials for the same registry (e.g. docker.io), credentials from the last file is used
	var authFiles []string
	if len(c.cfg.Common.ExternalRegistryAuthSecret) > 0 {
		authFiles = append(authFiles, path.Join(defaultExternalRegistryAuthPath, corev1.DockerConfigJsonKey))
	}
	authFiles = append(authFiles, path.Join(privateImageHubDockerAuthPath, corev1.DockerConfigJsonKey))
	for _, authFile := range authFiles {
		args = append(args, "--auth-file", authFile)
	}

	return args
}

func (c *buildKitKubeJobProps) getPodContainerResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    c.cfg.PipelineRunner.Builder.Resources.Requests.CPU,
			corev1.ResourceMemory: c.cfg.PipelineRunner.Builder.Resources.Requests.Memory,
		},
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    c.cfg.PipelineRunner.Builder.Resources.Limits.CPU,
			corev1.ResourceMemory: c.cfg.PipelineRunner.Builder.Resources.Limits.Memory,
		},
	}
}

func (c *buildKitKubeJobProps) getPodContainerSecurityContext() *corev1.SecurityContext {
	return securitycontext.Container(
		securitycontext.WithContainerDropAllCapabilities(),
		securitycontext.WithContainerCapabilities([]corev1.Capability{"SETUID", "SETGID", "SETFCAP"}),
		securitycontext.WithContainerSeccompProfile(corev1.SeccompProfile{
			Type:             corev1.SeccompProfileTypeLocalhost,
			LocalhostProfile: new(c.cfg.PipelineRunner.Builder.SeccompProfileLocalhostProfile),
		}),
		securitycontext.WithContainerRunAsNonRoot(new(false)),
		securitycontext.WithReadOnlyRootFileSystem(new(true)),
	)
}

func (c *buildKitKubeJobProps) getPodContainerEnvVars() []corev1.EnvVar {
	envVars := []corev1.EnvVar{
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
	}

	return envVars
}

func (c *buildKitKubeJobProps) getPodContainerVolumeMounts() []corev1.VolumeMount {
	volumeMounts := getCommonPodContainerVolumeMounts(c.componentImage, c.pipelineArgs.GitWorkspace)

	volumeMounts = append(volumeMounts,
		corev1.VolumeMount{
			Name:      buildKitRunVolumeName, // buildah creates folder container overlays and secrets there
			MountPath: "/run",
			ReadOnly:  false,
		},
		corev1.VolumeMount{
			Name:      buildKitRootVolumeName, // Required by buildah
			MountPath: "/root",
			ReadOnly:  false,
		},
		corev1.VolumeMount{
			Name:      defaults.PrivateImageHubSecretName,
			MountPath: privateImageHubDockerAuthPath,
			ReadOnly:  true,
		},
		corev1.VolumeMount{
			Name:      buildKitHomeVolumeName,
			MountPath: buildKitHomePath, // Writable directory where buildah's auth.json file is stored
			ReadOnly:  false,
		},
	)

	if len(c.cfg.Common.ExternalRegistryAuthSecret) > 0 {
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      c.cfg.Common.ExternalRegistryAuthSecret,
				MountPath: defaultExternalRegistryAuthPath,
				ReadOnly:  true,
			},
		)
	}

	if len(c.buildSecrets) > 0 {
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      defaults.BuildSecretsName,
				MountPath: buildKitBuildSecretsPath,
				ReadOnly:  true,
			},
		)
	}

	return volumeMounts
}
