package utils

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
)

// GetPipelineJobPodSpecTolerations returns tolerations required to schedule the pipeline job pod on specific nodes
func GetPipelineJobPodSpecTolerations() []corev1.Toleration {
	return []corev1.Toleration{getNodeTolerationExists(kube.NodeTaintJobsKey)}
}

// GetScheduledJobPodSpecTolerations returns tolerations required to schedule the job-component pod on specific nodes
func GetScheduledJobPodSpecTolerations(node *v1.RadixNode) []corev1.Toleration {
	if UseGPUNode(node) {
		return []corev1.Toleration{getNodeTolerationExists(kube.NodeTaintGpuCountKey)}
	}
	return []corev1.Toleration{getNodeTolerationExists(kube.NodeTaintJobsKey)}
}

// GetDeploymentPodSpecTolerations returns tolerations required to schedule the component pod on specific nodes
func GetDeploymentPodSpecTolerations(node *v1.RadixNode) []corev1.Toleration {
	if !UseGPUNode(node) {
		return nil
	}
	if _, err := GetNodeGPUCount(node.GpuCount); err == nil {
		return []corev1.Toleration{getNodeTolerationExists(kube.NodeTaintGpuCountKey)}
	}
	return nil
}

func getNodeTolerationExists(key string) corev1.Toleration {
	return corev1.Toleration{Key: key, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}
}
