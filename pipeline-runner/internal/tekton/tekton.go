package utils

import (
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	podLabelsVolumeName = "pod-labels"
	podLabelsFileName   = "labels"
)

// CreateActionPipelineJob Create action pipeline job
func CreateActionPipelineJob(containerName string, action string, pipelineInfo *model.PipelineInfo, appName string, initContainers []corev1.Container, envVars *[]corev1.EnvVar) *batchv1.Job {
	imageTag := pipelineInfo.PipelineArguments.ImageTag
	pipelineJobName := pipelineInfo.PipelineArguments.JobName
	timestamp := time.Now().Format("20060102150405")
	hash := strings.ToLower(utils.RandStringStrSeed(5, pipelineInfo.PipelineArguments.JobName))
	backOffLimit := int32(0)
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("radix-%s-pipelines-%s-%s-%s", action, timestamp, imageTag, hash),
			Labels: map[string]string{
				kube.RadixJobNameLabel:  pipelineJobName,
				kube.RadixAppLabel:      appName,
				kube.RadixImageTagLabel: imageTag,
				kube.RadixJobTypeLabel:  getActionPipelineJobTypeLabelByPipelinesAction(action),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      radixlabels.ForPipelineJobName(pipelineJobName),
					Annotations: annotations.ForClusterAutoscalerSafeToEvict(false),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: defaults.RadixTektonServiceAccountName,
					SecurityContext:    &pipelineInfo.PipelineArguments.PodSecurityContext,
					InitContainers:     initContainers,
					Containers: []corev1.Container{
						{
							Name:            containerName,
							Image:           fmt.Sprintf("%s/%s", pipelineInfo.PipelineArguments.ContainerRegistry, pipelineInfo.PipelineArguments.TektonPipeline),
							ImagePullPolicy: corev1.PullAlways,
							VolumeMounts:    getJobContainerVolumeMounts(),
							SecurityContext: &pipelineInfo.PipelineArguments.ContainerSecurityContext,
							Env:             *envVars,
						},
					},
					Volumes:       getJobVolumes(),
					RestartPolicy: "Never",
					Affinity:      utils.GetPipelineJobPodSpecAffinity(),
					Tolerations:   utils.GetPipelineJobPodSpecTolerations(),
				},
			},
		},
	}
	return &job
}

func getActionPipelineJobTypeLabelByPipelinesAction(action string) string {
	if action == pipelineDefaults.RadixPipelineActionPrepare {
		return kube.RadixJobTypePreparePipelines
	}
	return kube.RadixJobTypeRunPipelines
}

func getJobVolumes() []corev1.Volume {
	defaultMode := int32(256)

	volumes := []corev1.Volume{
		{
			Name: git.BuildContextVolumeName,
		},
		{
			Name: git.GitSSHKeyVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  git.GitSSHKeyVolumeName,
					DefaultMode: &defaultMode,
				},
			},
		},
		{
			Name: podLabelsVolumeName,
			VolumeSource: corev1.VolumeSource{
				DownwardAPI: &corev1.DownwardAPIVolumeSource{
					Items: []corev1.DownwardAPIVolumeFile{
						{
							Path:     podLabelsFileName,
							FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.labels"},
						},
					},
				},
			},
		},
	}

	return volumes
}

func getJobContainerVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      git.BuildContextVolumeName,
			MountPath: git.Workspace,
		},
		{
			Name:      podLabelsVolumeName,
			MountPath: fmt.Sprintf("/%s", podLabelsVolumeName),
		},
	}
}
