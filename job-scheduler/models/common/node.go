package common

// +kubebuilder:object:generate=true

import radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// Node defines node attributes, where container should be scheduled
type Node struct {
	// Defines rules for allowed GPU types.
	//
	// required: false
	Gpu string `json:"gpu,omitempty"`

	// Defines minimum number of required GPUs.
	//
	// required: false
	GpuCount string `json:"gpuCount,omitempty"`
}

// MapToRadixNode maps the object to a RadixNode object
func (n *Node) MapToRadixNode() *radixv1.RadixNode {
	if n == nil {
		return nil
	}
	return &radixv1.RadixNode{
		Gpu:      n.Gpu,
		GpuCount: n.GpuCount,
	}
}
