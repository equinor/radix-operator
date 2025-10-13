package common

// +kubebuilder:object:generate=true

import (
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type ResourceList map[string]string

// Resources describes the compute resource requirements.
type Resources struct {
	// Limits describes the maximum amount of compute resources allowed.
	//
	// required: false
	Limits ResourceList `json:"limits,omitempty"`

	// Requests describes the minimum amount of compute resources required.
	// If Requests is omitted for a container, it defaults to Limits if
	// that is explicitly specified, otherwise to an implementation-defined value.
	//
	// required: false
	Requests ResourceList `json:"requests,omitempty"`
}

// MapToRadixResourceRequirements maps the object to a ResourceRequirements object
func (r *Resources) MapToRadixResourceRequirements() *radixv1.ResourceRequirements {
	if r == nil {
		return nil
	}

	return &radixv1.ResourceRequirements{
		Limits:   radixv1.ResourceList(r.Limits),
		Requests: radixv1.ResourceList(r.Requests),
	}
}
