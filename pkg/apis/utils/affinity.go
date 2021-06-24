package utils

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

func GetPodSpecAffinity(radixComponent v1.RadixCommonDeployComponent) *corev1.Affinity {
	node := radixComponent.GetNode()
	if node == nil {
		return nil
	}

	affinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{}},
		},
	}

	addGpuNodeSelectorTerms(node, affinity.NodeAffinity)

	if len(affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms) > 0 {
		return affinity
	}
	return nil
}

func addGpuNodeSelectorTerms(node *v1.RadixNode, nodeAffinity *corev1.NodeAffinity) {
	nodeSelectorTerm := corev1.NodeSelectorTerm{}

	if node != nil {
		addNodeSelectorRequirementForGpu(node.Gpu, &nodeSelectorTerm)
		addNodeSelectorRequirementForGpuCount(node.GpuCount, &nodeSelectorTerm)
	}

	if len(nodeSelectorTerm.MatchExpressions) > 0 {
		nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, nodeSelectorTerm)
	}
}

func addNodeSelectorRequirementForGpu(gpu string, nodeSelectorTerm *corev1.NodeSelectorTerm) {
	nodeGpuValue := strings.ReplaceAll(gpu, " ", "")
	if len(nodeGpuValue) == 0 {
		return
	}
	nodeGpuList := strings.Split(nodeGpuValue, ",")
	if len(nodeGpuList) == 0 {
		return
	}
	includingGpus, excludingGpus := getGpuLists(nodeGpuList)
	addNodeSelectorRequirement(nodeSelectorTerm, kube.RadixGpuLabel, corev1.NodeSelectorOpIn, includingGpus...)
	addNodeSelectorRequirement(nodeSelectorTerm, kube.RadixGpuLabel, corev1.NodeSelectorOpNotIn, excludingGpus...)
}

func addNodeSelectorRequirementForGpuCount(gpuCount string, nodeSelectorTerm *corev1.NodeSelectorTerm) {
	gpuCount = strings.ReplaceAll(gpuCount, " ", "")
	if len(gpuCount) == 0 {
		return
	}
	gpuCountValue, err := strconv.Atoi(gpuCount)
	if err != nil || gpuCountValue <= 0 {
		log.Error(fmt.Sprintf("invalid node GPU count: %s", gpuCount))
		return
	}
	values := strconv.Itoa(gpuCountValue - 1)
	addNodeSelectorRequirement(nodeSelectorTerm, kube.RadixGpuCountLabel, corev1.NodeSelectorOpGt, values)
}

func getGpuLists(nodeGpuList []string) ([]string, []string) {
	includingGpus := make([]string, 0)
	excludingGpus := make([]string, 0)
	for _, gpu := range nodeGpuList {
		if strings.HasPrefix(gpu, "-") {
			excludingGpus = append(excludingGpus, strings.ToLower(gpu[1:]))
			continue
		}
		includingGpus = append(includingGpus, strings.ToLower(gpu))
	}
	return includingGpus, excludingGpus
}

func addNodeSelectorRequirement(nodeSelectorTerm *corev1.NodeSelectorTerm, key string, operator corev1.NodeSelectorOperator, values ...string) {
	if len(values) <= 0 {
		return
	}
	nodeSelectorTerm.MatchExpressions = append(nodeSelectorTerm.MatchExpressions, corev1.NodeSelectorRequirement{Key: key, Operator: operator, Values: values})
}
