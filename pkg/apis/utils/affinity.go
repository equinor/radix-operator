package utils

import (
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
)

// GetDeploymentPodSpecAffinity  Gets component pod specific affinity
func GetDeploymentPodSpecAffinity(node *v1.RadixNode, appName string, componentName string) *corev1.Affinity {
	return &corev1.Affinity{
		PodAntiAffinity: getPodAffinity(appName, componentName),
		NodeAffinity:    getNodeAffinityForGPUNode(node),
	}
}

// GetScheduledJobPodSpecAffinity  Gets job-component pod specific affinity
func GetScheduledJobPodSpecAffinity(node *v1.RadixNode) *corev1.Affinity {
	nodeAffinity := getNodeAffinity(node)
	if nodeAffinity == nil {
		return nil
	}
	return &corev1.Affinity{
		NodeAffinity: nodeAffinity,
	}
}

// GetPipelineJobPodSpecAffinity Gets pipeline job pod specific affinity
func GetPipelineJobPodSpecAffinity() *corev1.Affinity {
	return &corev1.Affinity{NodeAffinity: getNodeAffinityForJobNode()}
}

func getPodAffinity(appName string, componentName string) *corev1.PodAntiAffinity {
	return &corev1.PodAntiAffinity{
		PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
			{
				Weight:          1,
				PodAffinityTerm: getPodAffinityTerm(appName, componentName),
			},
		},
	}
}

func getNodeAffinity(node *v1.RadixNode) *corev1.NodeAffinity {
	if nodeAffinity := getNodeAffinityForGPUNode(node); nodeAffinity != nil {
		return nodeAffinity
	}
	return getNodeAffinityForJobNode()
}

func getPodAffinityTerm(appName string, componentName string) corev1.PodAffinityTerm {
	matchExpressions := []metav1.LabelSelectorRequirement{
		{
			Key:      kube.RadixAppLabel,
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{appName},
		},
	}
	if len(componentName) > 0 {
		matchExpressions = append(matchExpressions, metav1.LabelSelectorRequirement{
			Key:      kube.RadixComponentLabel,
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{componentName},
		})
	}
	return corev1.PodAffinityTerm{
		LabelSelector: &metav1.LabelSelector{
			MatchExpressions: matchExpressions,
		},
		TopologyKey: corev1.LabelHostname,
	}
}

func getNodeAffinityForGPUNode(radixNode *v1.RadixNode) *corev1.NodeAffinity {
	if !UseGPUNode(radixNode) {
		return nil
	}
	nodeSelectorTerm := &corev1.NodeSelectorTerm{}
	if err := addNodeSelectorRequirementForGpuCount(radixNode.GpuCount, nodeSelectorTerm); err != nil {
		log.Error().Err(err).Msg("Failed to add node selector requirement for GPU count")
		// TODO: should the error be returned to caller
		return nil
	}
	addNodeSelectorRequirementForGpu(radixNode.Gpu, nodeSelectorTerm)
	if len(nodeSelectorTerm.MatchExpressions) > 0 {
		return &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{*nodeSelectorTerm}},
		}
	}
	return nil
}

func addNodeSelectorRequirementForGpu(gpu string, nodeSelectorTerm *corev1.NodeSelectorTerm) {
	includingGpus, excludingGpus := GetNodeGPULists(gpu)
	if len(includingGpus)+len(excludingGpus) == 0 {
		return
	}
	addNodeSelectorRequirement(nodeSelectorTerm, kube.RadixGpuLabel, corev1.NodeSelectorOpIn, includingGpus...)
	addNodeSelectorRequirement(nodeSelectorTerm, kube.RadixGpuLabel, corev1.NodeSelectorOpNotIn, excludingGpus...)
}

func addNodeSelectorRequirementForGpuCount(gpuCount string, nodeSelectorTerm *corev1.NodeSelectorTerm) error {
	gpuCountValue, err := GetNodeGPUCount(gpuCount)
	if err != nil {
		return err
	}
	if gpuCountValue == nil {
		return nil
	}
	addNodeSelectorRequirement(nodeSelectorTerm, kube.RadixGpuCountLabel, corev1.NodeSelectorOpGt, strconv.Itoa((*gpuCountValue)-1))
	return nil
}

func getNodeAffinityForJobNode() *corev1.NodeAffinity {
	return &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: kube.RadixJobNodeLabel, Operator: corev1.NodeSelectorOpExists}}},
			},
		}}
}

func addNodeSelectorRequirement(nodeSelectorTerm *corev1.NodeSelectorTerm, key string, operator corev1.NodeSelectorOperator, values ...string) bool {
	if len(values) <= 0 {
		return false
	}
	nodeSelectorTerm.MatchExpressions = append(nodeSelectorTerm.MatchExpressions, corev1.NodeSelectorRequirement{Key: key, Operator: operator, Values: values})
	return true
}
