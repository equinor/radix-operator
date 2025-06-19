package utils

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/runtime"
	corev1 "k8s.io/api/core/v1"
)

// GetPipelineJobPodSpecTolerations returns tolerations required to schedule the pipeline job pod on specific nodes
func GetPipelineJobPodSpecTolerations() []corev1.Toleration {
	return []corev1.Toleration{getNodeTolerationExists(kube.NodeTaintJobsKey)}
}

// GetScheduledJobPodSpecTolerations returns tolerations required to schedule the job-component pod on specific nodes
func GetScheduledJobPodSpecTolerations(node *v1.RadixNode, nodeType *string) []corev1.Toleration {
	if tolerations, done := getNodeTypeTolerations(nodeType); done {
		return tolerations
	}
	if UseGPUNode(node) {
		return []corev1.Toleration{getNodeTolerationExists(kube.NodeTaintGpuCountKey)}
	}
	return []corev1.Toleration{getNodeTolerationExists(kube.NodeTaintJobsKey)}
}

// GetDeploymentPodSpecTolerations returns tolerations required to schedule the component pod on specific nodes
func GetDeploymentPodSpecTolerations(deployComponent v1.RadixCommonDeployComponent) []corev1.Toleration {
	if tolerations, done := getNodeTypeTolerations(deployComponent.GetRuntime().GetNodeType()); done {
		return tolerations
	}
	node := deployComponent.GetNode()
	if !UseGPUNode(node) {
		return nil
	}
	if _, err := GetNodeGPUCount(node.GpuCount); err == nil {
		return []corev1.Toleration{getNodeTolerationExists(kube.NodeTaintGpuCountKey)}
	}
	return nil
}

func getNodeTypeTolerations(nodeType *string) ([]corev1.Toleration, bool) {
	if nodeType == nil {
		return nil, false
	}
	if len(*nodeType) > 0 {
		return []corev1.Toleration{{
			Key:      runtime.NodeTypeTolerationKey,
			Operator: corev1.TolerationOpEqual,
			Value:    *nodeType,
			Effect:   corev1.TaintEffectNoSchedule,
		}}, true
	}
	return nil, true
}

func getNodeTolerationExists(key string) corev1.Toleration {
	return corev1.Toleration{Key: key, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}
}
