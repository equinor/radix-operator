package deployment

import (
	"fmt"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	"strconv"
)

func updateComponentNode(component v1.RadixCommonComponent, node *v1.RadixNode) {
	if len(node.Gpu) <= 0 {
		node.Gpu = component.GetNode().Gpu
	}

	nodeGpuCount := node.GpuCount
	if len(nodeGpuCount) <= 0 {
		node.GpuCount = component.GetNode().GpuCount
		return
	}
	if gpuCount, err := strconv.Atoi(nodeGpuCount); err != nil || gpuCount <= 0 {
		log.Error(fmt.Sprintf("invalid environment node GPU count: %s in component %s", nodeGpuCount, component.GetName()))
		node.GpuCount = component.GetNode().GpuCount
	}
}
