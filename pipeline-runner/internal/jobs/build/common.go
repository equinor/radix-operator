package build

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixannotations "github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
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

func getCommonJobLabels(appName, pipelineJobName, imageTag string) map[string]string {
	return map[string]string{
		kube.RadixJobNameLabel:  pipelineJobName,
		kube.RadixAppLabel:      appName,
		kube.RadixImageTagLabel: imageTag,
		kube.RadixJobTypeLabel:  kube.RadixJobTypeBuild,
	}
}

func getCommonJobAnnotations(branch string, componentImages ...pipeline.BuildComponentImage) map[string]string {
	componentImagesAnnotation, _ := json.Marshal(componentImages)
	return map[string]string{
		kube.RadixBranchAnnotation:          branch,
		kube.RadixBuildComponentsAnnotation: string(componentImagesAnnotation),
	}
}

func getCommonPodLabels(pipelineJobName string) map[string]string {
	return radixlabels.ForPipelineJobName(pipelineJobName)
}

func getCommonPodAnnotations() map[string]string {
	return radixannotations.ForClusterAutoscalerSafeToEvict(false)
}

func getCommonPodAffinity(runtime *radixv1.Runtime) *corev1.Affinity {
	return utils.GetAffinityForPipelineJob(runtime)
}

func getCommonPodTolerations() []corev1.Toleration {
	return utils.GetPipelineJobPodSpecTolerations()
}

func getCommonPodInitContainers(cloneURL, branch string, cloneConfig git.CloneConfig) []corev1.Container {
	return git.CloneInitContainers(cloneURL, branch, cloneConfig)
}

func getCommonPodVolumes(componentImages []pipeline.BuildComponentImage) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: git.BuildContextVolumeName,
		},
		{
			Name: git.GitSSHKeyVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  git.GitSSHKeyVolumeName,
					DefaultMode: pointers.Ptr(volumeDefaultMode),
				},
			},
		},
	}

	for _, image := range componentImages {
		volumes = append(volumes,
			corev1.Volume{
				Name: getTmpVolumeNameForContainer(image.ContainerName),
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						SizeLimit: resource.NewScaledQuantity(100, resource.Giga),
					},
				},
			},
			corev1.Volume{
				Name: getVarVolumeNameForContainer(image.ContainerName),
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						SizeLimit: resource.NewScaledQuantity(100, resource.Giga),
					},
				},
			},
		)
	}

	return volumes
}

func getCommonPodContainerVolumeMounts(componentImage pipeline.BuildComponentImage) []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      git.BuildContextVolumeName,
			MountPath: git.Workspace,
		},
		{
			Name:      getTmpVolumeNameForContainer(componentImage.ContainerName), // image-builder creates a script there
			MountPath: "/tmp",
			ReadOnly:  false,
		},
		{
			Name:      getVarVolumeNameForContainer(componentImage.ContainerName), // image-builder creates files there
			MountPath: "/var",
			ReadOnly:  false,
		},
	}
}

func getTmpVolumeNameForContainer(containerName string) string {
	return fmt.Sprintf("tmp-%s", containerName)
}

func getVarVolumeNameForContainer(containerName string) string {
	return fmt.Sprintf("var-%s", containerName)
}
