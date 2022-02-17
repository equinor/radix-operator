package utils

import (
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
)

// GetPodSpecTolerations returns tolerations required to schedule the pod on nodes defined by RadixNode
func GetPodSpecTolerations(node *v1.RadixNode) []corev1.Toleration {
	if node == nil {
		return nil
	}

	// No toleration required if Gpu is empty
	if len(strings.ReplaceAll(node.Gpu, " ", "")) == 0 {
		return nil
	}

	var tolerations []corev1.Toleration

	tolerations = append(tolerations, getGpuNodeTolerations(node)...)

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

	tolerations := []corev1.Toleration{
		{Key: kube.NodeTaintGpuCountKey, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
	}

	return tolerations
}
