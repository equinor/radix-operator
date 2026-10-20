package build

import (
	"fmt"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/git"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixannotations "github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	volumeDefaultMode = int32(256)
)

// Common functions - move
func getJobName(timeStamp time.Time, imageTag, hash string) string {
	ts := timeStamp.Format("20060102150405")
	return fmt.Sprintf("radix-builder-%s-%s-%s", ts, imageTag, hash)
}

func getCommonJobLabels(appName, pipelineJobName, envName, componentName string) map[string]string {
	return map[string]string{
		kube.RadixJobNameLabel:   pipelineJobName,
		kube.RadixAppLabel:       appName,
		kube.RadixJobTypeLabel:   kube.RadixJobTypeBuild,
		kube.RadixEnvLabel:       envName,
		kube.RadixComponentLabel: componentName,
	}
}

func getCommonJobAnnotations(branch, gitRef, gitRefType string) map[string]string {
	return map[string]string{
		kube.RadixBranchAnnotation:     branch,
		kube.RadixGitRefAnnotation:     gitRef,
		kube.RadixGitRefTypeAnnotation: gitRefType,
	}
}

func getCommonPodLabels(pipelineJobName string, appName string, appID radixv1.ULID) map[string]string {
	return radixlabels.Merge(
		radixlabels.ForApplicationName(appName),
		radixlabels.ForPipelineJobName(pipelineJobName),
		radixlabels.ForApplicationID(appID),
	)
}

func getCommonPodAnnotations() map[string]string {
	return radixannotations.ForClusterAutoscalerSafeToEvict(false)
}

func getCommonPodAffinity(nodeArch string) *corev1.Affinity {
	return utils.GetAffinityForPipelineJob(nodeArch)
}

func getCommonPodTolerations() []corev1.Toleration {
	return utils.GetPipelineJobPodSpecTolerations()
}

func getCommonPodInitContainers(cloneURL, gitRefName, gitCommitID, workspace, gitImage string) []corev1.Container {
	return git.CloneInitContainersWithSourceCode(cloneURL, gitRefName, gitCommitID, workspace, gitImage)
}

func getCommonPodVolumes() []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: git.BuildContextVolumeName,
		},
		{
			Name: git.GitSSHKeyVolumeName,
			Secret: &corev1.SecretVolumeSource{
				SecretName:  git.GitSSHKeyVolumeName,
				DefaultMode: new(volumeDefaultMode),
			},
		},
		{
			Name: git.CloneRepoHomeVolumeName,
			EmptyDir: &corev1.EmptyDirVolumeSource{
				SizeLimit: resource.NewScaledQuantity(5, resource.Mega),
			},
		},
		{
			Name: getTmpVolumeNameForContainer(),
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: resource.NewScaledQuantity(100, resource.Giga),
				},
			},
		},
		{
			Name: getVarVolumeNameForContainer(),
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: resource.NewScaledQuantity(100, resource.Giga),
				},
			},
		},
	}

	return volumes
}

func getCommonPodContainerVolumeMounts(workspace string) []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      git.BuildContextVolumeName,
			MountPath: workspace,
		},
		{
			Name:      getTmpVolumeNameForContainer(), // image-builder creates a script there
			MountPath: "/tmp",
			ReadOnly:  false,
		},
		{
			Name:      getVarVolumeNameForContainer(), // image-builder creates files there
			MountPath: "/var",
			ReadOnly:  false,
		},
	}
}

func getTmpVolumeNameForContainer() string {
	return fmt.Sprintf("tmp-%s", defaults.RadixPipelineJobBuildContainerName)
}

func getVarVolumeNameForContainer() string {
	return fmt.Sprintf("var-%s", defaults.RadixPipelineJobBuildContainerName)
}
