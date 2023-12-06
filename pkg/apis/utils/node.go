package utils

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/equinor/radix-common/utils/pointers"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// UseGPUNode If RadixNode is defined to use GPU node
func UseGPUNode(node *radixv1.RadixNode) bool {
	if node == nil {
		return false
	}
	if includingGpus, excludingGpus := GetNodeGPULists(node.Gpu); len(includingGpus)+len(excludingGpus) == 0 {
		return false
	}
	if _, err := GetNodeGPUCount(node.GpuCount); err != nil {
		return false
	}
	return true
}

// GetNodeGPUCount Gets GPU count for the RadixNode
func GetNodeGPUCount(gpuCount string) (*int, error) {
	gpuCount = strings.TrimSpace(gpuCount)
	if len(gpuCount) == 0 {
		return nil, nil
	}
	gpuCountValue, err := strconv.Atoi(gpuCount)
	if err != nil || gpuCountValue <= 0 {
		return nil, fmt.Errorf("invalid node GPU count: %s", gpuCount)
	}
	return pointers.Ptr(gpuCountValue), nil
}

// GetNodeGPULists Get including and excluding GPU names for the node
func GetNodeGPULists(gpu string) ([]string, []string) {
	includingGpus := make([]string, 0)
	excludingGpus := make([]string, 0)
	for _, gpuItem := range strings.Split(strings.TrimSpace(gpu), ",") {
		gpuItem = strings.ToLower(strings.TrimSpace(gpuItem))
		if len(gpuItem) == 0 {
			continue
		}
		if strings.HasPrefix(gpuItem, "-") {
			gpuName := strings.TrimSpace(gpuItem[1:])
			if len(gpuName) > 0 {
				excludingGpus = append(excludingGpus, gpuName)
			}
			continue
		}
		includingGpus = append(includingGpus, gpuItem)
	}
	return includingGpus, excludingGpus
}
