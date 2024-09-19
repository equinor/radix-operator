package build_test

import (
	"encoding/json"
	"fmt"
	"path"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/internal/jobs/build"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Test_BuildKit_JobSpec(t *testing.T) {
	t.Run("nocache-nopush", func(t *testing.T) { assertBuildKitJobSpec(t, false, false, nil, "") })
	t.Run("cache-nopush", func(t *testing.T) { assertBuildKitJobSpec(t, true, false, nil, "") })
	t.Run("nocache-push", func(t *testing.T) { assertBuildKitJobSpec(t, false, true, nil, "") })
	t.Run("cache-push", func(t *testing.T) { assertBuildKitJobSpec(t, true, true, nil, "") })
	t.Run("with buildsecrets", func(t *testing.T) { assertBuildKitJobSpec(t, true, true, []string{"secret1", "secret2"}, "") })
	t.Run("with buildsecrets", func(t *testing.T) { assertBuildKitJobSpec(t, true, true, nil, "anyexternalregsecret") })
}

func assertBuildKitJobSpec(t *testing.T, useCache, pushImage bool, buildSecrets []string, externalRegistrySecret string) {
	const (
		cloneURL      = "anycloneurl"
		gitCommitHash = "anygitcommithash"
		gitTags       = "anygittags"
	)

	args := model.PipelineArguments{
		AppName:                "anyappname",
		PipelineType:           "anypipelinetype",
		JobName:                "anyjobname",
		Branch:                 "anybranch",
		CommitID:               "anycommitid",
		ImageTag:               "anyimagetag",
		PushImage:              pushImage,
		BuildKitImageBuilder:   "anyimagebuilder",
		GitCloneNsLookupImage:  "anynslookupimage",
		GitCloneGitImage:       "anygitcloneimage",
		GitCloneBashImage:      "anybashimage",
		Clustertype:            "anyclustertype",
		Clustername:            "anyclustername",
		ContainerRegistry:      "anycontainerregistry",
		AppContainerRegistry:   "anyappcontainerregistry",
		SeccompProfileFileName: "anyseccompprofilefile",
		ExternalContainerRegistryDefaultAuthSecret: externalRegistrySecret,
		Builder: model.Builder{ResourcesLimitsMemory: "100M", ResourcesRequestsCPU: "50m", ResourcesRequestsMemory: "50M"},
	}
	require.Equal(t, pushImage, args.PushImage)
	require.Equal(t, externalRegistrySecret, args.ExternalContainerRegistryDefaultAuthSecret)
	componentImages := []pipeline.BuildComponentImage{
		{ComponentName: "c1", EnvName: "c1env", ContainerName: "c1container", Context: "c1ctx", Dockerfile: "c1dockerfile", ImageName: "c1imagename", ImagePath: "c1image", ClusterTypeImagePath: "c1clustertypeimage", ClusterNameImagePath: "c1clusternameimage"},
		{ComponentName: "c2", EnvName: "c2env", ContainerName: "c2container", Context: "c2ctx", Dockerfile: "c2dockerfile", ImageName: "c2imagename", ImagePath: "c2image", ClusterTypeImagePath: "c2clustertypeimage", ClusterNameImagePath: "c2clusternameimage", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}},
		{ComponentName: "c3", EnvName: "c3env", ContainerName: "c3container", Context: "c3ctx", Dockerfile: "c3dockerfile", ImageName: "c2imagename", ImagePath: "c2image", ClusterTypeImagePath: "c3clustertypeimage", ClusterNameImagePath: "c3clusternameimage", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}},
	}

	sut := build.NewBuildKit()
	jobs := sut.BuildJobs(useCache, args, cloneURL, gitCommitHash, gitTags, componentImages, buildSecrets)
	require.Len(t, jobs, len(componentImages))

	for _, ci := range componentImages {
		t.Run(fmt.Sprintf("%s-%s", ci.EnvName, ci.ComponentName), func(t *testing.T) {
			job, ok := slice.FindFirst(jobs, func(j batchv1.Job) bool {
				return j.Labels[kube.RadixEnvLabel] == ci.EnvName && j.Labels[kube.RadixComponentLabel] == ci.ComponentName
			})
			require.True(t, ok)
			assert.NotEmpty(t, job)

			// Check job
			expectedJobLabels := map[string]string{
				kube.RadixJobNameLabel:   args.JobName,
				kube.RadixAppLabel:       args.AppName,
				kube.RadixImageTagLabel:  args.ImageTag,
				kube.RadixJobTypeLabel:   kube.RadixJobTypeBuild,
				kube.RadixEnvLabel:       ci.EnvName,
				kube.RadixComponentLabel: ci.ComponentName,
			}
			assert.Equal(t, expectedJobLabels, job.Labels)
			componentImagesAnnotation, _ := json.Marshal([]pipeline.BuildComponentImage{ci})
			expectedJobAnnotations := map[string]string{
				kube.RadixBranchAnnotation:          args.Branch,
				kube.RadixBuildComponentsAnnotation: string(componentImagesAnnotation),
			}
			assert.Equal(t, expectedJobAnnotations, job.Annotations)
			assert.Equal(t, pointers.Ptr[int32](0), job.Spec.BackoffLimit)

			// Check pod template
			expectedPodLabels := map[string]string{
				kube.RadixJobNameLabel: args.JobName,
			}
			assert.Equal(t, expectedPodLabels, job.Spec.Template.Labels)
			expectedPodAnnotations := annotations.ForClusterAutoscalerSafeToEvict(false)
			expectedPodAnnotations[fmt.Sprintf("container.apparmor.security.beta.kubernetes.io/%s", ci.ContainerName)] = "unconfined"
			assert.Equal(t, expectedPodAnnotations, job.Spec.Template.Annotations)
			assert.Equal(t, corev1.RestartPolicyNever, job.Spec.Template.Spec.RestartPolicy)
			arch := radixv1.RuntimeArchitectureAmd64
			if ci.Runtime != nil {
				arch = ci.Runtime.Architecture
			}
			expectedAffinity := &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
				{Key: kube.RadixJobNodeLabel, Operator: corev1.NodeSelectorOpExists},
				{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
				{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{string(arch)}},
			}}}}}}
			assert.Equal(t, expectedAffinity, job.Spec.Template.Spec.Affinity)
			assert.ElementsMatch(t, utils.GetPipelineJobPodSpecTolerations(), job.Spec.Template.Spec.Tolerations)
			expectedPodSecurityContext := securitycontext.Pod(
				securitycontext.WithPodFSGroup(1000),
				securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault),
				securitycontext.WithPodRunAsNonRoot(pointers.Ptr(false)))
			assert.Equal(t, expectedPodSecurityContext, job.Spec.Template.Spec.SecurityContext)
			expectedVolumes := []corev1.Volume{
				{Name: git.BuildContextVolumeName},
				{Name: git.GitSSHKeyVolumeName, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: git.GitSSHKeyVolumeName, DefaultMode: pointers.Ptr[int32](256)}}},
				{Name: fmt.Sprintf("tmp-%s", ci.ContainerName), VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: resource.NewScaledQuantity(100, resource.Giga)}}},
				{Name: fmt.Sprintf("var-%s", ci.ContainerName), VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: resource.NewScaledQuantity(100, resource.Giga)}}},
				{Name: defaults.PrivateImageHubSecretName, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: defaults.PrivateImageHubSecretName}}},
				{Name: "radix-image-builder-home", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: resource.NewScaledQuantity(5, resource.Mega)}}},
				{Name: "build-kit-run", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: resource.NewScaledQuantity(100, resource.Giga)}}},
				{Name: "build-kit-root", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: resource.NewScaledQuantity(100, resource.Giga)}}},
				{Name: git.CloneRepoHomeVolumeName, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: resource.NewScaledQuantity(5, resource.Mega)}}},
			}
			if len(args.ExternalContainerRegistryDefaultAuthSecret) > 0 {
				expectedVolumes = append(expectedVolumes, corev1.Volume{
					Name: args.ExternalContainerRegistryDefaultAuthSecret, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: args.ExternalContainerRegistryDefaultAuthSecret}},
				})
			}
			if len(buildSecrets) > 0 {
				expectedVolumes = append(expectedVolumes, corev1.Volume{
					Name: defaults.BuildSecretsName, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: defaults.BuildSecretsName}},
				})
			}
			assert.ElementsMatch(t, expectedVolumes, job.Spec.Template.Spec.Volumes)

			// Check init containers
			assert.ElementsMatch(t, []string{"internal-nslookup", "clone", "internal-chmod"}, slice.Map(job.Spec.Template.Spec.InitContainers, func(c corev1.Container) string { return c.Name }))
			cloneContainer, _ := slice.FindFirst(job.Spec.Template.Spec.InitContainers, func(c corev1.Container) bool { return c.Name == "clone" })
			assert.Equal(t, args.GitCloneGitImage, cloneContainer.Image)
			assert.Equal(t, []string{"sh", "-c", "git config --global --add safe.directory /workspace && git clone --recurse-submodules anycloneurl -b anybranch --verbose --progress /workspace && cd /workspace && if [ -n \"$(git lfs ls-files 2>/dev/null)\" ]; then git lfs install && echo 'Pulling large files...' && git lfs pull && echo 'Done'; fi && cd -"}, cloneContainer.Command)
			assert.Empty(t, cloneContainer.Args)
			expectedCloneVolumeMounts := []corev1.VolumeMount{
				{Name: git.BuildContextVolumeName, MountPath: git.Workspace},
				{Name: git.GitSSHKeyVolumeName, MountPath: "/.ssh", ReadOnly: true},
				{Name: git.CloneRepoHomeVolumeName, MountPath: git.CloneRepoHomeVolumePath},
			}
			assert.ElementsMatch(t, expectedCloneVolumeMounts, cloneContainer.VolumeMounts)

			// Check container
			require.Len(t, job.Spec.Template.Spec.Containers, 1)
			c := job.Spec.Template.Spec.Containers[0]
			assert.Equal(t, ci.ContainerName, c.Name)
			assert.Equal(t, fmt.Sprintf("%s/%s", args.ContainerRegistry, args.BuildKitImageBuilder), c.Image)
			assert.Equal(t, corev1.PullAlways, c.ImagePullPolicy)
			expectedResources := corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse(args.Builder.ResourcesRequestsCPU),
					corev1.ResourceMemory: resource.MustParse(args.Builder.ResourcesRequestsMemory),
				},
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse(args.Builder.ResourcesLimitsMemory),
				},
			}
			assert.Equal(t, expectedResources, c.Resources)
			expectedSecurityContext := securitycontext.Container(
				securitycontext.WithContainerDropAllCapabilities(),
				securitycontext.WithContainerCapabilities([]corev1.Capability{"SETUID", "SETGID", "SETFCAP"}),
				securitycontext.WithContainerSeccompProfile(corev1.SeccompProfile{
					Type:             corev1.SeccompProfileTypeLocalhost,
					LocalhostProfile: utils.StringPtr(args.SeccompProfileFileName),
				}),
				securitycontext.WithContainerRunAsNonRoot(pointers.Ptr(false)),
				securitycontext.WithReadOnlyRootFileSystem(pointers.Ptr(true)),
			)
			assert.Equal(t, expectedSecurityContext, c.SecurityContext)
			expectedEnvVars := []corev1.EnvVar{
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
			assert.ElementsMatch(t, expectedEnvVars, c.Env)
			expectedVolumeMounts := []corev1.VolumeMount{
				{Name: git.BuildContextVolumeName, MountPath: git.Workspace},
				{Name: fmt.Sprintf("tmp-%s", ci.ContainerName), MountPath: "/tmp", ReadOnly: false},
				{Name: fmt.Sprintf("var-%s", ci.ContainerName), MountPath: "/var", ReadOnly: false},
				{Name: "build-kit-run", MountPath: "/run", ReadOnly: false},
				{Name: "build-kit-root", MountPath: "/root", ReadOnly: false},
				{Name: defaults.PrivateImageHubSecretName, MountPath: "/radix-private-image-hubs", ReadOnly: true},
				{Name: "radix-image-builder-home", MountPath: "/home/build", ReadOnly: false},
			}
			if len(args.ExternalContainerRegistryDefaultAuthSecret) > 0 {
				expectedVolumeMounts = append(expectedVolumeMounts, corev1.VolumeMount{Name: args.ExternalContainerRegistryDefaultAuthSecret, MountPath: "/radix-default-external-registry-auth", ReadOnly: true})
			}
			if len(buildSecrets) > 0 {
				expectedVolumeMounts = append(expectedVolumeMounts, corev1.VolumeMount{Name: defaults.BuildSecretsName, MountPath: "/build-secrets", ReadOnly: true})
			}
			assert.ElementsMatch(t, expectedVolumeMounts, c.VolumeMounts)
			expectedArgs := []string{
				"--registry", args.ContainerRegistry,
				"--registry-username", "$(BUILDAH_USERNAME)",
				"--registry-password", "$(BUILDAH_PASSWORD)",
				"--cache-registry", args.AppContainerRegistry,
				"--cache-registry-username", "$(BUILDAH_CACHE_USERNAME)",
				"--cache-registry-password", "$(BUILDAH_CACHE_PASSWORD)",
				"--cache-repository", utils.GetImageCachePath(args.AppContainerRegistry, args.AppName),
				"--tag", ci.ImagePath,
				"--cluster-type-tag", ci.ClusterTypeImagePath,
				"--cluster-name-tag", ci.ClusterNameImagePath,
				"--secrets-path", "/build-secrets",
				"--dockerfile", ci.Dockerfile,
				"--context", ci.Context,
				"--branch", args.Branch,
				"--git-commit-hash", gitCommitHash,
				"--git-tags", gitTags,
				"--target-environments", ci.EnvName,
			}
			if useCache {
				expectedArgs = append(expectedArgs, "--use-cache")
			}
			if args.PushImage {
				expectedArgs = append(expectedArgs, "--push")
			}
			for _, secret := range buildSecrets {
				expectedArgs = append(expectedArgs, "--secret", secret)
			}
			if len(args.ExternalContainerRegistryDefaultAuthSecret) > 0 {
				expectedArgs = append(expectedArgs, "--auth-file", path.Join("/radix-default-external-registry-auth", corev1.DockerConfigJsonKey))
			}
			expectedArgs = append(expectedArgs, "--auth-file", path.Join("/radix-private-image-hubs", corev1.DockerConfigJsonKey))
			assert.Equal(t, expectedArgs, c.Args)
		})
	}
}
