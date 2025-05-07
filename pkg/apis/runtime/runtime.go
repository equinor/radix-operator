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
	ReplacedBy string `json:"replacedBy,omitempty"`
}

// NodeType holds information about a node type attributes
type NodeType struct {
	// Architecture is the architecture of the node type
	Architecture radixv1.RuntimeArchitecture `json:"architecture,omitempty"`
	// Deprecated is the deprecation information of the node type
	Deprecated *NodeTypeDeprecation `json:"deprecated,omitempty"`
}

const (
	// NodeTypeAffinityKey is the label key for the custom node type
	NodeTypeAffinityKey = "radix-nodetype"
	// NodeTypeTolerationKey is the taint key for the custom node type
	NodeTypeTolerationKey = "radix-nodetype"
)

// Node types and their attributes supported by Radix
var nodeTypes = map[string]NodeType{
	"memory-optimized-v1": {Architecture: radixv1.RuntimeArchitectureAmd64},
	"nvidia-v100-v1":      {Architecture: radixv1.RuntimeArchitectureAmd64},
}

// GetArchitectureFromRuntime returns architecture from Runtime.
// If Runtime is nil or Runtime.Architecture is empty then defaults.DefaultNodeSelectorArchitecture is returned with false flag
func GetArchitectureFromRuntime(runtime *radixv1.Runtime) (string, bool) {
	if nodeType := runtime.GetNodeType(); nodeType != nil {
		if attrs, ok := nodeTypes[*nodeType]; ok {
			return string(attrs.Architecture), true
		}
	}
	if runtime != nil && len(runtime.Architecture) > 0 {
		return string(runtime.Architecture), true
	}
	return defaults.DefaultNodeSelectorArchitecture, false

}

// GetArchitectureFromRuntimeOrDefault returns architecture from Runtime.
func GetArchitectureFromRuntimeOrDefault(runtime *radixv1.Runtime) string {
	architecture, _ := GetArchitectureFromRuntime(runtime)
	return architecture
}
