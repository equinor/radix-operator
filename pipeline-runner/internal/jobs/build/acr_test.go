package build_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/internal/jobs/build"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/git"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Test_ACR_JobSpec(t *testing.T) {
	t.Run("push", func(t *testing.T) { assertACRJobSpec(t, true) })
	t.Run("nopush", func(t *testing.T) { assertACRJobSpec(t, false) })
}

func assertACRJobSpec(t *testing.T, pushImage bool) {
	const (
		cloneURL      = "anycloneurl"
		gitCommitHash = "anygitcommithash"
		gitTags       = "anygittags"
	)

	args := model.PipelineArguments{
		AppName:               "anyappname",
		PipelineType:          "anypipelinetype",
		JobName:               "anyjobname",
		Branch:                "anybranch",
		GitRefType:            "anygitreftype",
		CommitID:              "anycommitid",
		ImageTag:              "anyimagetag",
		PushImage:             pushImage,
		ImageBuilder:          "anyimagebuilder",
		GitCloneNsLookupImage: "anynslookupimage",
		GitCloneGitImage:      "anygitcloneimage",
		GitCloneBashImage:     "anybashimage",
		Clustertype:           "anyclustertype",
		Clustername:           "anyclustername",
		ContainerRegistry:     "anycontainerregistry",
		SubscriptionId:        "anysubscriptionid",
		RadixZone:             "anyradixzone",
		GitWorkspace:          "/some-workspace",
	}
	require.Equal(t, pushImage, args.PushImage)
	componentImages := []pipeline.BuildComponentImage{
		{ComponentName: "c1", EnvName: "c1env", ContainerName: "c1container", Context: "c1ctx", Dockerfile: "c1dockerfile", ImageName: "c1imagename", ImagePath: "c1image", ClusterTypeImagePath: "c1clustertypeimage", ClusterNameImagePath: "c1clusternameimage"},
		{ComponentName: "c2", EnvName: "c2env", ContainerName: "c2container", Context: "c2ctx", Dockerfile: "c2dockerfile", ImageName: "c2imagename", ImagePath: "c2image", ClusterTypeImagePath: "c2clustertypeimage", ClusterNameImagePath: "c2clusternameimage"},
	}
	buildSecrets := []string{"secret1", "secret2"}

	sut := build.NewACR()
	jobs := sut.BuildJobs(false, false, args, cloneURL, gitCommitHash, gitTags, componentImages, buildSecrets)
	require.Len(t, jobs, 1)
	job := jobs[0]

	// Check job
	expectedJobLabels := map[string]string{
		kube.RadixJobNameLabel:  args.JobName,
		kube.RadixAppLabel:      args.AppName,
		kube.RadixImageTagLabel: args.ImageTag,
		kube.RadixJobTypeLabel:  kube.RadixJobTypeBuild,
	}
	assert.Equal(t, expectedJobLabels, job.Labels)
	componentImagesAnnotation, _ := json.Marshal(componentImages)
	expectedJobAnnotations := map[string]string{
		kube.RadixBranchAnnotation:          args.Branch, //nolint:staticcheck
		kube.RadixGitRefAnnotation:          args.GitRef,
		kube.RadixGitRefTypeAnnotation:      args.GitRefType,
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
	assert.Equal(t, expectedPodAnnotations, job.Spec.Template.Annotations)

	assert.Equal(t, corev1.RestartPolicyNever, job.Spec.Template.Spec.RestartPolicy)
	expectedAffinity := &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
		{Key: kube.RadixJobNodeLabel, Operator: corev1.NodeSelectorOpExists},
		{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
		{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{string(radixv1.RuntimeArchitectureArm64)}},
	}}}}}}
	assert.Equal(t, expectedAffinity, job.Spec.Template.Spec.Affinity)
	assert.ElementsMatch(t, utils.GetPipelineJobPodSpecTolerations(), job.Spec.Template.Spec.Tolerations)
	expectedPodSecurityContext := securitycontext.Pod(
		securitycontext.WithPodFSGroup(1000),
		securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault))
	assert.Equal(t, expectedPodSecurityContext, job.Spec.Template.Spec.SecurityContext)
	expectedVolumes := []corev1.Volume{
		{Name: git.BuildContextVolumeName},
		{Name: git.GitSSHKeyVolumeName, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: git.GitSSHKeyVolumeName, DefaultMode: pointers.Ptr[int32](256)}}},
		{Name: defaults.AzureACRServicePrincipleSecretName, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: defaults.AzureACRServicePrincipleSecretName}}},
		{Name: "radix-image-builder-home", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: resource.NewScaledQuantity(5, resource.Mega)}}},
		{Name: git.CloneRepoHomeVolumeName, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: resource.NewScaledQuantity(5, resource.Mega)}}},
	}
	for _, image := range componentImages {
		expectedVolumes = append(expectedVolumes,
			corev1.Volume{Name: fmt.Sprintf("tmp-%s", image.ContainerName), VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: resource.NewScaledQuantity(100, resource.Giga)}}},
			corev1.Volume{Name: fmt.Sprintf("var-%s", image.ContainerName), VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: resource.NewScaledQuantity(100, resource.Giga)}}},
		)
	}
	assert.ElementsMatch(t, expectedVolumes, job.Spec.Template.Spec.Volumes)

	// Check init containers
	assert.ElementsMatch(t, []string{"internal-nslookup", "clone", "internal-chmod"}, slice.Map(job.Spec.Template.Spec.InitContainers, func(c corev1.Container) string { return c.Name }))
	cloneContainer, _ := slice.FindFirst(job.Spec.Template.Spec.InitContainers, func(c corev1.Container) bool { return c.Name == "clone" })
	assert.Equal(t, args.GitCloneGitImage, cloneContainer.Image)
	assert.Equal(t, []string{"sh", "-c", "git config --global --add safe.directory /some-workspace && git clone anycloneurl -b anybranch --verbose --progress /some-workspace && (git submodule update --init --recursive || echo \"Warning: Unable to clone submodules, proceeding without them\") && cd /some-workspace && if [ -n \"$(git lfs ls-files 2>/dev/null)\" ]; then git lfs install && echo 'Pulling large files...' && git lfs pull && echo 'Done'; fi && cd -"}, cloneContainer.Command)
	assert.Empty(t, cloneContainer.Args)
	expectedCloneVolumeMounts := []corev1.VolumeMount{
		{Name: git.BuildContextVolumeName, MountPath: "/some-workspace"},
		{Name: git.GitSSHKeyVolumeName, MountPath: "/.ssh", ReadOnly: true},
		{Name: git.CloneRepoHomeVolumeName, MountPath: git.CloneRepoHomeVolumePath},
	}
	assert.ElementsMatch(t, expectedCloneVolumeMounts, cloneContainer.VolumeMounts)

	// Check containers
	var pushArg string
	if args.PushImage {
		pushArg = "--push"
	}
	assert.Len(t, job.Spec.Template.Spec.Containers, len(componentImages))
	for _, ci := range componentImages {
		t.Run(fmt.Sprintf("check container %s", ci.ContainerName), func(t *testing.T) {
			c, ok := slice.FindFirst(job.Spec.Template.Spec.Containers, func(c corev1.Container) bool { return c.Name == ci.ContainerName })
			require.True(t, ok)
			assert.Equal(t, ci.ContainerName, c.Name)
			assert.Equal(t, fmt.Sprintf("%s/%s", args.ContainerRegistry, args.ImageBuilder), c.Image)
			assert.Equal(t, corev1.PullAlways, c.ImagePullPolicy)
			assert.Equal(t, resource.MustParse("500m"), *c.Resources.Limits.Cpu())
			assert.Equal(t, resource.MustParse("50m"), *c.Resources.Requests.Cpu())
			assert.Equal(t, resource.MustParse("500M"), *c.Resources.Limits.Memory())
			assert.Equal(t, resource.MustParse("100M"), *c.Resources.Requests.Memory())
			expectedSecurityCtx := securitycontext.Container(
				securitycontext.WithContainerDropAllCapabilities(),
				securitycontext.WithContainerSeccompProfileType(corev1.SeccompProfileTypeRuntimeDefault),
				securitycontext.WithContainerRunAsUser(1000),
				securitycontext.WithContainerRunAsGroup(1000),
				securitycontext.WithReadOnlyRootFileSystem(pointers.Ptr(true)),
			)
			assert.Equal(t, expectedSecurityCtx, c.SecurityContext)
			expectedEnvs := []corev1.EnvVar{
				{Name: "DOCKER_FILE_NAME", Value: ci.Dockerfile},
				{Name: "DOCKER_REGISTRY", Value: args.ContainerRegistry},
				{Name: "IMAGE", Value: ci.ImagePath},
				{Name: "CONTEXT", Value: ci.Context},
				{Name: "PUSH", Value: pushArg},
				{Name: "AZURE_CREDENTIALS", Value: "/radix-image-builder/.azure/sp_credentials.json"},
				{Name: "SUBSCRIPTION_ID", Value: args.SubscriptionId},
				{Name: "CLUSTERTYPE_IMAGE", Value: ci.ClusterTypeImagePath},
				{Name: "CLUSTERNAME_IMAGE", Value: ci.ClusterNameImagePath},
				{Name: "RADIX_ZONE", Value: args.RadixZone},
				{Name: "BRANCH", Value: args.Branch},
				{Name: "GIT_REF", Value: args.GitRef},
				{Name: "GIT_REF_TYPE", Value: args.GitRefType},
				{Name: "TARGET_ENVIRONMENTS", Value: ci.EnvName},
				{Name: "RADIX_GIT_COMMIT_HASH", Value: gitCommitHash},
				{Name: "RADIX_GIT_TAGS", Value: gitTags},
			}
			for _, s := range buildSecrets {
				expectedEnvs = append(expectedEnvs, corev1.EnvVar{
					Name:      defaults.BuildSecretPrefix + s,
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: s, LocalObjectReference: corev1.LocalObjectReference{Name: defaults.BuildSecretsName}}},
				})
			}
			assert.ElementsMatch(t, expectedEnvs, c.Env)
			expectedVolumeMounts := []corev1.VolumeMount{
				{Name: git.BuildContextVolumeName, MountPath: "/some-workspace"},
				{Name: fmt.Sprintf("tmp-%s", ci.ContainerName), MountPath: "/tmp", ReadOnly: false},
				{Name: fmt.Sprintf("var-%s", ci.ContainerName), MountPath: "/var", ReadOnly: false},
				{Name: defaults.AzureACRServicePrincipleSecretName, MountPath: "/radix-image-builder/.azure", ReadOnly: true},
				{Name: "radix-image-builder-home", MountPath: "/home/radix-image-builder", ReadOnly: false},
			}
			assert.ElementsMatch(t, expectedVolumeMounts, c.VolumeMounts)
		})
	}
}
