package runtime

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// NodeTypeDeprecation holds information about deprecation of a node type
type NodeTypeDeprecation struct {
	// Message is the deprecation message
	Message string `json:"message,omitempty"`
	// ReplacedBy is the node type that replaces this node type
	ReplacedBy NodeTypeIdentifier `json:"replacedBy,omitempty"`
}

// NodeType holds information about a node type attributes
type NodeType struct {
	// Architecture is the architecture of the node type
	Architecture radixv1.RuntimeArchitecture `json:"architecture,omitempty"`
	// Deprecated is the deprecation information of the node type
	Deprecated NodeTypeDeprecation `json:"deprecated,omitempty"`
}

// NodeTypeIdentifier is the type of node
type NodeTypeIdentifier string

const (
	// MemoryOptimizedV1Node is the node type for high memory v1
	MemoryOptimizedV1Node NodeTypeIdentifier = "memory-optimized-v1"
	// MemoryOptimizedV2Node is the node type for high memory v2
	MemoryOptimizedV2Node NodeTypeIdentifier = "memory-optimized-v2"
	// GpuV1 is the node type for CPU
	GpuV1 NodeTypeIdentifier = "gpu-a100-v1"
	// GpuV2 is the node type for CPU
	GpuV2 NodeTypeIdentifier = "gpu-a100-2"
	// GpuV3 is the node type for CPU
	GpuV3 NodeTypeIdentifier = "gpu-a100-4"
)

var nodeTypes = map[NodeTypeIdentifier]NodeType{
	MemoryOptimizedV1Node: {Architecture: radixv1.RuntimeArchitectureAmd64},
	GpuV1:                 {Architecture: radixv1.RuntimeArchitectureAmd64},
}

// GetArchitectureFromRuntime returns architecture from Runtime.
// If Runtime is nil or Runtime.Architecture is empty then defaults.DefaultNodeSelectorArchitecture is returned
func GetArchitectureFromRuntime(runtime *radixv1.Runtime) (string, bool) {
	if nodeType := runtime.GetNodeType(); nodeType != nil {
		if attrs, ok := nodeTypes[NodeTypeIdentifier(*nodeType)]; ok {
			return string(attrs.Architecture), true
		}
	}
	if runtime != nil && len(runtime.Architecture) > 0 {
		return string(runtime.Architecture), true
	}
	return defaults.DefaultNodeSelectorArchitecture, false
}
