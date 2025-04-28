package runtime

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type nodeTypeAttrs struct {
	architecture radixv1.RuntimeArchitecture
}

// NodeType is the type of node
type NodeType string

const (
	// HighMemoryV1Node is the node type for high memory
	HighMemoryV1Node NodeType = "high-memory-v1"
	// GpuV1 is the node type for CPU
	GpuV1 NodeType = "gpu-v1"
)

var nodeTypes = map[NodeType]nodeTypeAttrs{
	HighMemoryV1Node: {architecture: radixv1.RuntimeArchitectureAmd64},
	GpuV1:            {architecture: radixv1.RuntimeArchitectureAmd64},
}

// GetArchitectureFromRuntime returns architecture from Runtime.
// If Runtime is nil or Runtime.Architecture is empty then defaults.DefaultNodeSelectorArchitecture is returned
func GetArchitectureFromRuntime(runtime *radixv1.Runtime) (string, bool) {
	if nodeType := runtime.GetNodeType(); nodeType != nil {
		if attrs, ok := nodeTypes[NodeType(*nodeType)]; ok {
			return string(attrs.architecture), true
		}
	}
	if runtime != nil && len(runtime.Architecture) > 0 {
		return string(runtime.Architecture), true
	}
	return defaults.DefaultNodeSelectorArchitecture, false
}
