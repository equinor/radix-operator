package common

// +kubebuilder:object:generate=true

import (
	"reflect"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// Runtime defines the component or job's target runtime requirements
type Runtime struct {
	// CPU architecture
	//
	// example: amd64
	Architecture string `json:"architecture"`

	// Defines the node type for the component. It is a node-pool label and taint, where the component's or job's pods will be scheduled.
	// More info: https://www.radix.equinor.com/radix-config#nodetype
	// +kubebuilder:validation:MaxLength=120
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9]*)?[a-z0-9])?$
	// +optional
	NodeType *string `json:"nodeType,omitempty"`
}

// MapToRadixRuntime maps the object to a RadixV1 Runtime object
func (runtime *Runtime) MapToRadixRuntime() *radixv1.Runtime {
	if runtime == nil {
		return nil
	}
	return &radixv1.Runtime{
		Architecture: radixv1.RuntimeArchitecture(runtime.Architecture),
		NodeType:     runtime.NodeType,
	}
}

// RuntimeTransformer is a mergo transformer for the Runtime struct
type RuntimeTransformer struct{}

// Transformer implements the mergo.Transformer interface
func (transformer RuntimeTransformer) Transformer(t reflect.Type) func(dst, src reflect.Value) error {
	if t != reflect.TypeOf(new(Runtime)) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		if dst.Kind() != reflect.Ptr || src.Kind() != reflect.Ptr {
			return nil
		}
		if !src.IsNil() {
			dst.Set(src)
		}
		return nil
	}
}
