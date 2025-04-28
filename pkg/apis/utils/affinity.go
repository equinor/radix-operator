package utils

import (
	"context"
	"slices"
	"strconv"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/runtime"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetAffinityForDeployComponent  Gets component pod specific affinity
func GetAffinityForDeployComponent(ctx context.Context, component radixv1.RadixCommonDeployComponent, appName string, componentName string) *corev1.Affinity {
	nodeAffinity := getNodeAffinityForDeployComponent(ctx, component)
	return &corev1.Affinity{
		PodAntiAffinity: getPodAntiAffinity(appName, componentName),
		NodeAffinity:    nodeAffinity,
	}

}

// GetAffinityForBatchJob  Gets batch job pod specific affinity
func GetAffinityForBatchJob(ctx context.Context, node *radixv1.RadixNode, nodeType *string, nodeArch string) *corev1.Affinity {
	return &corev1.Affinity{
		NodeAffinity: getNodeAffinityForBatchJob(ctx, node, nodeType, nodeArch),
	}
}

// GetAffinityForPipelineJob Gets pipeline job pod specific affinity
func GetAffinityForPipelineJob(nodeArch string) *corev1.Affinity {
	return &corev1.Affinity{
		NodeAffinity: getNodeAffinityForPipelineJob(nodeArch),
	}
}

func GetAffinityForOAuthAuxComponent() *corev1.Affinity {
	return &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: getNodeSelectorRequirementsForRuntimeEnvironment(string(radixv1.RuntimeArchitectureAmd64)),
					},
				},
			},
		},
	}
}

func GetAffinityForJobAPIAuxComponent() *corev1.Affinity {
	return &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: getNodeSelectorRequirementsForRuntimeEnvironment(string(radixv1.RuntimeArchitectureAmd64)),
					},
				},
			},
		},
	}
}

func getNodeAffinityForDeployComponent(ctx context.Context, component radixv1.RadixCommonDeployComponent) *corev1.NodeAffinity {
	var affinityNodeSelectorTerms []corev1.NodeSelectorTerm

	if selectorTerm := getNodeTypeAffinitySelectorTerm(component.GetRuntime().GetNodeType()); selectorTerm != nil {
		affinityNodeSelectorTerms = append(affinityNodeSelectorTerms, *selectorTerm)
	} else if affinity := getNodeAffinityForGPUNode(ctx, component.GetNode()); affinity != nil {
		return affinity
	}

	affinityNodeSelectorTerms = append(affinityNodeSelectorTerms, getRuntimeAffinitySelectorTerm(component))

	return &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: affinityNodeSelectorTerms,
		},
	}
}

func getRuntimeAffinitySelectorTerm(component radixv1.RadixCommonDeployComponent) corev1.NodeSelectorTerm {
	nodeArch, _ := runtime.GetArchitectureFromRuntime(component.GetRuntime())
	return corev1.NodeSelectorTerm{MatchExpressions: getNodeSelectorRequirementsForRuntimeEnvironment(nodeArch)}
}

func getNodeAffinityForBatchJob(ctx context.Context, node *radixv1.RadixNode, nodeType *string, nodeArch string) *corev1.NodeAffinity {
	var nodeSelectorTerms []corev1.NodeSelectorTerm
	if nodeSelectorTerm := getNodeTypeAffinitySelectorTerm(nodeType); nodeSelectorTerm != nil {
		nodeSelectorTerms = append(nodeSelectorTerms, *nodeSelectorTerm)
	} else if affinity := getNodeAffinityForGPUNode(ctx, node); affinity != nil { // TODO delete when NodeType is deployed and no Node is used
		return affinity
	}
	nodeSelectorTerms = append(nodeSelectorTerms, corev1.NodeSelectorTerm{
		MatchExpressions: slices.Concat(getNodeSelectorRequirementsForJobNodePool(), getNodeSelectorRequirementsForRuntimeEnvironment(nodeArch)),
	})
	return &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: nodeSelectorTerms},
	}
}

func getNodeAffinityForPipelineJob(nodeArch string) *corev1.NodeAffinity {
	return &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: slices.Concat(getNodeSelectorRequirementsForJobNodePool(), getNodeSelectorRequirementsForRuntimeEnvironment(nodeArch)),
				},
			},
		},
	}
}

func getPodAntiAffinity(appName string, componentName string) *corev1.PodAntiAffinity {
	return &corev1.PodAntiAffinity{
		PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
			{
				Weight:          1,
				PodAffinityTerm: getPodAffinityTerm(appName, componentName),
			},
		},
	}
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

func getNodeAffinityForGPUNode(ctx context.Context, radixNode *radixv1.RadixNode) *corev1.NodeAffinity {
	if !UseGPUNode(radixNode) {
		return nil
	}
	nodeSelectorTerm := &corev1.NodeSelectorTerm{}
	if err := addNodeSelectorRequirementForGpuCount(radixNode.GpuCount, nodeSelectorTerm); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Failed to add node selector requirement for GPU count")
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

func addNodeSelectorRequirement(nodeSelectorTerm *corev1.NodeSelectorTerm, key string, operator corev1.NodeSelectorOperator, values ...string) bool {
	if len(values) <= 0 {
		return false
	}
	nodeSelectorTerm.MatchExpressions = append(nodeSelectorTerm.MatchExpressions, corev1.NodeSelectorRequirement{Key: key, Operator: operator, Values: values})
	return true
}

func getNodeSelectorRequirementsForRuntimeEnvironment(nodeArch string) []corev1.NodeSelectorRequirement {
	return []corev1.NodeSelectorRequirement{
		{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
		{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{nodeArch}},
	}
}

func getNodeSelectorRequirementsForJobNodePool() []corev1.NodeSelectorRequirement {
	return []corev1.NodeSelectorRequirement{
		{Key: kube.RadixJobNodeLabel, Operator: corev1.NodeSelectorOpExists},
	}
}

func getNodeTypeAffinitySelectorTerm(nodeType *string) *corev1.NodeSelectorTerm {
	if nodeType == nil || len(*nodeType) == 0 {
		return nil
	}
	return &corev1.NodeSelectorTerm{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: runtime.NodeTypeAffinityKey, Operator: corev1.NodeSelectorOpIn, Values: []string{*nodeType}}}}
}
