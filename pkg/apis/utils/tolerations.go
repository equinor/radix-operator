package utils

import (
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
)

// GetPodSpecTolerations returns tolerations required to schedule the pod on nodes
func GetPodSpecTolerations(node *v1.RadixNode, isScheduledJob bool, isPipelineJob bool) []corev1.Toleration {
	var tolerations []corev1.Toleration
	tolerations = append(tolerations, getGpuNodeTolerations(node)...)
	if isPipelineJob || isScheduledJob {
		return append(tolerations, getJobNodeToleration())
	}
	return tolerations
}

func getGpuNodeTolerations(node *v1.RadixNode) []corev1.Toleration {
	if node == nil {
		return nil
	}

	// No toleration required if Gpu is empty
	if len(strings.ReplaceAll(node.Gpu, " ", "")) == 0 {
		return nil
	}

	return []corev1.Toleration{
		getNodeTolerationExists(kube.NodeTaintGpuCountKey),
	}
}

func getJobNodeToleration() corev1.Toleration {
	return getNodeTolerationExists(kube.NodeTaintJobsKey)
}

func getNodeTolerationExists(key string) corev1.Toleration {
	return corev1.Toleration{Key: key, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}
}
