package models

import (
	"github.com/equinor/radix-common/utils/slice"
	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
	corev1 "k8s.io/api/core/v1"
)

// BuildReplicaSummaryList builds a list of ReplicaSummary models.
func BuildReplicaSummaryList(podList []corev1.Pod, lastEventWarnings map[string]string) []deploymentModels.ReplicaSummary {
	return slice.Map(podList, func(pod corev1.Pod) deploymentModels.ReplicaSummary {
		return deploymentModels.GetReplicaSummary(pod, lastEventWarnings[pod.GetName()])
	})
}
