package utils

import (
	"strings"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// UseGPUNode If RadixNode is defined to use GPU node
func UseGPUNode(node *radixv1.RadixNode) bool {
	return node != nil && len(strings.TrimSpace(node.Gpu)) > 0
}
