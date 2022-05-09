package utils

import (
	"fmt"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"time"
)

const (
	podLabelsVolumeName = "pod-labels"
	podLabelsFileName   = "labels"
)

//CreateTektonPipelineJob Create Tekton pipeline job
func CreateTektonPipelineJob(containerName string, action string, pipelineInfo *model.PipelineInfo, appName string, initContainers []corev1.Container, envVars *[]corev1.EnvVar) *batchv1.Job {
	imageTag := pipelineInfo.PipelineArguments.ImageTag
	jobName := pipelineInfo.PipelineArguments.JobName
	timestamp := time.Now().Format("20060102150405")
	hash := strings.ToLower(utils.RandStringStrSeed(5, pipelineInfo.PipelineArguments.RadixPipelineRun))
	backOffLimit := int32(0)
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("radix-tekton-pipeline-%s-%s-%s-%s", action, timestamp, imageTag, hash),
			Labels: map[string]string{
				kube.RadixJobNameLabel:     jobName,
				kube.RadixAppLabel:         appName,
				kube.RadixImageTagLabel:    imageTag,
				kube.RadixJobTypeLabel:     kube.RadixJobTypeTektonPipeline,
				kube.RadixPipelineRunLabel: pipelineInfo.PipelineArguments.RadixPipelineRun,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: defaults.RadixTektonRoleName,
					SecurityContext:    &pipelineInfo.PipelineArguments.PodSecurityContext,
					InitContainers:     initContainers,
					Containers: []corev1.Container{
						{
							Name:            containerName,
							Image:           fmt.Sprintf("%s/%s", pipelineInfo.ContainerRegistry, pipelineInfo.PipelineArguments.TektonPipeline),
							ImagePullPolicy: corev1.PullAlways,
							VolumeMounts:    getJobContainerVolumeMounts(),
							SecurityContext: &pipelineInfo.PipelineArguments.ContainerSecurityContext,
							Env:             *envVars,
						},
					},
					Volumes:       getJobVolumes(),
					RestartPolicy: "Never",
				},
			},
		},
	}
	return &job
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
