package utils

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
)

// GetPodSpecTolerations returns tolerations required to schedule the pod on nodes
func GetPodSpecTolerations(node *v1.RadixNode, isScheduledJob bool, isPipelineJob bool) []corev1.Toleration {
	if UseGPUNode(node) {
		return []corev1.Toleration{getNodeTolerationExists(kube.NodeTaintGpuCountKey)}
	}
	if isPipelineJob || isScheduledJob {
		return []corev1.Toleration{getNodeTolerationExists(kube.NodeTaintJobsKey)}
	}
	return nil
}

func getNodeTolerationExists(key string) corev1.Toleration {
	return corev1.Toleration{Key: key, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}
}
