package utils

import (
	"slices"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
)

// GetAffinityForDeployComponent  Gets component pod specific affinity
func GetAffinityForDeployComponent(component v1.RadixCommonDeployComponent, appName string, componentName string) *corev1.Affinity {
	return &corev1.Affinity{
		PodAntiAffinity: getPodAntiAffinity(appName, componentName),
		NodeAffinity:    getNodeAffinityForDeployComponent(component),
	}

}

// GetAffinityForBatchJob  Gets batch job pod specific affinity
func GetAffinityForBatchJob(job *v1.RadixDeployJobComponent, node *v1.RadixNode) *corev1.Affinity {
	return &corev1.Affinity{
		NodeAffinity: getNodeAffinityForBatchJob(job, node),
	}
}

// GetAffinityForPipelineJob Gets pipeline job pod specific affinity
func GetAffinityForPipelineJob() *corev1.Affinity {
	return &corev1.Affinity{
		NodeAffinity: getNodeAffinityForPipelineJob(),
	}
}

func GetAffinityForOAuthAuxComponent() *corev1.Affinity {
	return &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: getNodeSelectorRequirementsForRuntimeEnvironment(nil),
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
						MatchExpressions: getNodeSelectorRequirementsForRuntimeEnvironment(nil),
					},
				},
			},
		},
	}
}

func getNodeAffinityForDeployComponent(component v1.RadixCommonDeployComponent) *corev1.NodeAffinity {
	if affinity := getNodeAffinityForGPUNode(component.GetNode()); affinity != nil {
		return affinity
	}
	return &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: getNodeSelectorRequirementsForRuntimeEnvironment(component.GetRuntime()),
				},
			},
		},
	}
}

func getNodeAffinityForBatchJob(job *v1.RadixDeployJobComponent, node *v1.RadixNode) *corev1.NodeAffinity {
	if affinity := getNodeAffinityForGPUNode(node); affinity != nil {
		return affinity
	}
	return &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: slices.Concat(getNodeSelectorRequirementsForJobNodePool(), getNodeSelectorRequirementsForRuntimeEnvironment(job.Runtime)),
				},
			},
		},
	}
}

func getNodeAffinityForPipelineJob() *corev1.NodeAffinity {
	return &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: slices.Concat(getNodeSelectorRequirementsForJobNodePool(), getNodeSelectorRequirementsForRuntimeEnvironment(nil)),
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

func addNodeSelectorRequirement(nodeSelectorTerm *corev1.NodeSelectorTerm, key string, operator corev1.NodeSelectorOperator, values ...string) bool {
	if len(values) <= 0 {
		return false
	}
	nodeSelectorTerm.MatchExpressions = append(nodeSelectorTerm.MatchExpressions, corev1.NodeSelectorRequirement{Key: key, Operator: operator, Values: values})
	return true
}

func getNodeSelectorRequirementsForRuntimeEnvironment(runtime *v1.Runtime) []corev1.NodeSelectorRequirement {
	arch := defaults.DefaultNodeSelectorArchitecture
	if runtime != nil && len(runtime.Architecture) > 0 {
		arch = string(runtime.Architecture)
	}

	return []corev1.NodeSelectorRequirement{
		{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
		{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{arch}},
	}
}

func getNodeSelectorRequirementsForJobNodePool() []corev1.NodeSelectorRequirement {
	return []corev1.NodeSelectorRequirement{
		{Key: kube.RadixJobNodeLabel, Operator: corev1.NodeSelectorOpExists},
	}
}
