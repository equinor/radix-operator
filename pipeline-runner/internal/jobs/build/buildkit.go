package build

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	internalgit "github.com/equinor/radix-operator/pipeline-runner/internal/git"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
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
	defaultExternalRegistruAuthPath = "/radix-default-external-registry-auth"
)

type BuildKit struct{}

func (c *BuildKit) GetJobs(useBuildCache bool, pipelineArgs model.PipelineArguments, cloneURL, gitCommitHash, gitTags string, componentImages []pipeline.BuildComponentImage, buildSecrets []string) ([]batchv1.Job, error) {
	var jobs []batchv1.Job

	for _, componentImage := range componentImages {
		job, err := c.constructJob(componentImage, useBuildCache, pipelineArgs, cloneURL, gitCommitHash, gitTags, buildSecrets)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

func (c *BuildKit) constructJob(componentImage pipeline.BuildComponentImage, useBuildCache bool, pipelineArgs model.PipelineArguments, cloneURL, gitCommitHash, gitTags string, buildSecrets []string) (batchv1.Job, error) {
	var kubeJob kubeJobBuilder
	buildkit := &buildKitJob{
		pipelineArgs:   pipelineArgs,
		componentImage: componentImage,
		cloneURL:       cloneURL,
		gitCommitHash:  gitCommitHash,
		gitTags:        gitTags,
		buildSecrets:   buildSecrets,
		useBuildCache:  useBuildCache,
	}
	initContainers, err := buildkit.GetPodInitContainers()
	if err != nil {
		return batchv1.Job{}, err
	}

	kubeJob.SetName(buildkit.GetJobName())
	kubeJob.SetLabels(buildkit.GetJobLabels())
	kubeJob.SetAnnotations(buildkit.GetJobAnnotations())
	kubeJob.SetPodLabels(buildkit.GetPodLabels())
	kubeJob.SetPodAnnotations(buildkit.GetPodAnnotations())
	kubeJob.SetPodTolerations(buildkit.GetPodTolerations())
	kubeJob.SetPodAffinity(buildkit.GetPodAffinity())
	kubeJob.SetPodSecurityContext(buildkit.GetPodSecurityContext())
	kubeJob.SetPodVolumes(buildkit.GetPodVolumes())
	kubeJob.SetPodInitContainers(initContainers)
	kubeJob.SetPodContainers(buildkit.GetPodContainers())

	return kubeJob.GetJob(), nil
}

type buildKitJob struct {
	pipelineArgs   model.PipelineArguments
	componentImage pipeline.BuildComponentImage
	cloneURL       string
	gitCommitHash  string
	gitTags        string
	buildSecrets   []string
	useBuildCache  bool
}

func (c *buildKitJob) GetJobName() string {
	hash := strings.ToLower(utils.RandStringStrSeed(5, fmt.Sprintf("%s-%s-%s", c.pipelineArgs.JobName, c.componentImage.EnvName, c.componentImage.ComponentName)))
	return getJobName(time.Now(), c.pipelineArgs.ImageTag, hash)
}

func (c *buildKitJob) GetJobLabels() map[string]string {
	return labels.Merge(
		getDefaultJobLabels(c.pipelineArgs.AppName, c.pipelineArgs.JobName, c.pipelineArgs.ImageTag),
		labels.ForEnvironmentName(c.componentImage.EnvName),
		labels.ForComponentName(c.componentImage.ComponentName),
	)
}

func (c *buildKitJob) GetJobAnnotations() map[string]string {
	return getDefaultJobAnnotations(c.pipelineArgs.Branch, c.componentImage)
}

func (c *buildKitJob) GetPodLabels() map[string]string {
	return getDefaultPodLabels(c.pipelineArgs.JobName)
}

func (c *buildKitJob) GetPodAnnotations() map[string]string {
	annotations := getDefaultPodAnnotations()
	annotations[fmt.Sprintf("container.apparmor.security.beta.kubernetes.io/%s", c.componentImage.ContainerName)] = "unconfined"
	return annotations
}

func (c *buildKitJob) GetPodTolerations() []corev1.Toleration {
	return getDefaultPodTolerations()
}

func (c *buildKitJob) GetPodAffinity() *corev1.Affinity {
	return getDefaultPodAffinity(c.componentImage.Runtime)
}

func (*buildKitJob) GetPodSecurityContext() *corev1.PodSecurityContext {
	return securitycontext.Pod(
		securitycontext.WithPodFSGroup(1000),
		securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault),
		securitycontext.WithPodRunAsNonRoot(pointers.Ptr(false)))
}

func (c *buildKitJob) GetPodInitContainers() ([]corev1.Container, error) {
	cloneCfg := internalgit.CloneConfigFromPipelineArgs(c.pipelineArgs)
	return getDefaultPodInitContainers(c.cloneURL, c.pipelineArgs.Branch, cloneCfg)
}

func (c *buildKitJob) GetPodContainers() []corev1.Container {
	container := corev1.Container{
		Name:            c.componentImage.ContainerName,
		Image:           fmt.Sprintf("%s/%s", c.pipelineArgs.ContainerRegistry, c.pipelineArgs.BuildKitImageBuilder),
		ImagePullPolicy: corev1.PullAlways,
		Args:            c.getPodContainerArgs(),
		Env:             c.getPodContainerEnvVars(),
		VolumeMounts:    c.getPodContainerVolumeMounts(),
		SecurityContext: c.getPodContainerSecurityContext(),
		Resources:       c.getPodContainerResources(),
	}

	return []corev1.Container{container}
}

func (c *buildKitJob) getPodContainerArgs() []string {
	args := []string{
		"--registry", c.pipelineArgs.ContainerRegistry,
		"--registry-username", "$(BUILDAH_USERNAME)",
		"--registry-password", "$(BUILDAH_PASSWORD)",
		"--cache-registry", c.pipelineArgs.AppContainerRegistry,
		"--cache-registry-username", "$(BUILDAH_CACHE_USERNAME)",
		"--cache-registry-password", "$(BUILDAH_CACHE_PASSWORD)",
		"--cache-repository", utils.GetImageCachePath(c.pipelineArgs.AppContainerRegistry, c.pipelineArgs.AppName),
		"--tag", c.componentImage.ImagePath,
		"--cluster-type-tag", c.componentImage.ClusterTypeImagePath,
		"--cluster-name-tag", c.componentImage.ClusterNameImagePath,
		"--secrets-path", buildKitBuildSecretsPath,
		"--dockerfile", c.componentImage.Dockerfile,
		"--context", c.componentImage.Context,
		"--branch", c.pipelineArgs.Branch,
		"--git-commit-hash", c.gitCommitHash,
		"--git-tags", c.gitTags,
		"--target-environments", c.componentImage.EnvName,
	}

	if c.useBuildCache {
		args = append(args, "--use-cache")
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
	if len(c.pipelineArgs.ExternalContainerRegistryDefaultAuthSecret) > 0 {
		authFiles = append(authFiles, path.Join(defaultExternalRegistruAuthPath, corev1.DockerConfigJsonKey))
	}
	authFiles = append(authFiles, path.Join(privateImageHubDockerAuthPath, corev1.DockerConfigJsonKey))
	for _, authFile := range authFiles {
		args = append(args, "--auth-file", authFile)
	}

	return args
}

// func (c *buildkitJob) getPodContainerCommand() []string {
// 	commandList := commandbuilder.NewCommandList()
// 	commandList.AddStrCmd("mkdir /var/tmp && cp %s %s", path.Join(privateImageHubDockerAuthPath, ".dockerconfigjson"), buildKitRegistryAuthFile)
// 	commandList.AddStrCmd("/usr/bin/buildah login --username ${BUILDAH_USERNAME} --password ${BUILDAH_PASSWORD} %s", c.pipelineArgs.ContainerRegistry)
// 	if c.useBuildCache {
// 		commandList.AddStrCmd("/usr/bin/buildah login --username ${BUILDAH_CACHE_USERNAME} --password ${BUILDAH_CACHE_PASSWORD} %s", c.pipelineArgs.AppContainerRegistry)
// 	}
// 	buildah := commandbuilder.NewCommand("/usr/bin/buildah build")
// 	commandList.AddCmd(buildah)

// 	context := componentImage.Context
// 	buildah.
// 		AddArgf("--storage-driver=overlay").
// 		AddArgf("--isolation=chroot").
// 		AddArgf("--jobs 0").
// 		AddArgf("--ulimit nofile=4096:4096").
// 		AddArg(c.getSecretArgs()).
// 		AddArgf("--file %s%s", context, componentImage.Dockerfile).
// 		AddArgf(`--build-arg RADIX_GIT_COMMIT_HASH="${RADIX_GIT_COMMIT_HASH}"`).
// 		AddArgf(`--build-arg RADIX_GIT_TAGS="${RADIX_GIT_TAGS}"`).
// 		AddArgf(`--build-arg BRANCH="${BRANCH}"`).
// 		AddArgf(`--build-arg TARGET_ENVIRONMENTS="${TARGET_ENVIRONMENTS}"`)

// 	if c.useBuildCache {
// 		cacheImagePath := utils.GetImageCachePath(c.pipelineArgs.AppContainerRegistry, c.pipelineArgs.AppName)
// 		buildah.
// 			AddArgf("--layers").
// 			AddArgf("--cache-to=%s", cacheImagePath).
// 			AddArgf("--cache-from=%s", cacheImagePath)
// 	}

// 	imageTag, clusterTypeImageTag, clusterNameImageTag := componentImage.ImagePath, componentImage.ClusterTypeImagePath, componentImage.ClusterNameImagePath
// 	if c.pipelineArgs.PushImage {
// 		buildah.
// 			AddArgf("--tag %s", imageTag).
// 			AddArgf("--tag %s", clusterTypeImageTag).
// 			AddArgf("--tag %s", clusterNameImageTag)
// 	}

// 	buildah.AddArg(context)

// 	if c.pipelineArgs.PushImage {
// 		commandList.
// 			AddStrCmd("/usr/bin/buildah push --storage-driver=overlay %s", imageTag).
// 			AddStrCmd("/usr/bin/buildah push --storage-driver=overlay %s", clusterTypeImageTag).
// 			AddStrCmd("/usr/bin/buildah push --storage-driver=overlay %s", clusterNameImageTag)
// 	}

// 	return []string{"/bin/bash", "-c", commandList.String()}
// }

// func (c *buildahConstructor) getSecretArgs() string {
// 	var secretArgs []string
// 	for _, secret := range c.buildSecrets {
// 		secretArgs = append(secretArgs, fmt.Sprintf("--secret id=%s,src=%s/%s", secret, buildKitBuildSecretsPath, secret))
// 	}
// 	return strings.Join(secretArgs, " ")
// }

func (c *buildKitJob) getPodContainerResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse(c.pipelineArgs.Builder.ResourcesRequestsCPU),
			corev1.ResourceMemory: resource.MustParse(c.pipelineArgs.Builder.ResourcesRequestsMemory),
		},
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceMemory: resource.MustParse(c.pipelineArgs.Builder.ResourcesLimitsMemory),
		},
	}
}

func (*buildKitJob) getPodContainerSecurityContext() *corev1.SecurityContext {
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

func (c *buildKitJob) getPodContainerEnvVars() []corev1.EnvVar {
	envVars := getDefaultPodContainerEnvVars(c.componentImage, c.pipelineArgs.Branch, c.gitCommitHash, c.gitTags)

	envVars = append(envVars,
		corev1.EnvVar{
			Name: "BUILDAH_USERNAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: defaults.AzureACRServicePrincipleBuildahSecretName},
					Key:                  "username",
				},
			},
		},
		corev1.EnvVar{
			Name: "BUILDAH_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: defaults.AzureACRServicePrincipleBuildahSecretName},
					Key:                  "password",
				},
			},
		},
		corev1.EnvVar{
			Name: "BUILDAH_CACHE_USERNAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: defaults.AzureACRTokenPasswordAppRegistrySecretName},
					Key:                  "username",
				},
			},
		},
		corev1.EnvVar{
			Name: "BUILDAH_CACHE_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: defaults.AzureACRTokenPasswordAppRegistrySecretName},
					Key:                  "password",
				},
			},
		},
		// corev1.EnvVar{
		// 	// Ready by buildah to located default docker auth file, ref https://github.com/containers/buildah/blob/main/docs/buildah-login.1.md#options
		// 	Name:  "REGISTRY_AUTH_FILE",
		// 	Value: buildKitRegistryAuthFile,
		// },
	)

	return envVars
}

func (c *buildKitJob) getPodContainerVolumeMounts() []corev1.VolumeMount {
	volumeMounts := getDefaultPodContainerVolumeMounts(c.componentImage)

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

	if len(c.pipelineArgs.ExternalContainerRegistryDefaultAuthSecret) > 0 {
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      c.pipelineArgs.ExternalContainerRegistryDefaultAuthSecret,
				MountPath: defaultExternalRegistruAuthPath,
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

func (c *buildKitJob) GetPodVolumes() []corev1.Volume {
	volumes := getDefaultPodVolumes([]pipeline.BuildComponentImage{c.componentImage})

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

	if len(c.pipelineArgs.ExternalContainerRegistryDefaultAuthSecret) > 0 {
		volumes = append(volumes,
			corev1.Volume{
				Name: c.pipelineArgs.ExternalContainerRegistryDefaultAuthSecret,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: c.pipelineArgs.ExternalContainerRegistryDefaultAuthSecret,
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
