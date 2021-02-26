package deployment

import (
	"strings"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// RadixComponentType defines values for radix-component-type label
type RadixComponentType string

// ExistInDeploymentSpec checks if RadixDeployment has a component/job of this type with with the specified name
func (t RadixComponentType) ExistInDeploymentSpec(rd *v1.RadixDeployment, name string) bool {
	switch t {
	case RadixDeploymentComponent:
		return nameExistInDeploymentComponentList(rd, name)
	case RadixDeploymentJob:
		return nameExistInDeploymentJobList(rd, name)
	default:
		return false
	}
}

func nameExistInDeploymentComponentList(rd *v1.RadixDeployment, name string) bool {
	for _, component := range rd.Spec.Components {
		if strings.EqualFold(component.Name, name) {
			return true
		}
	}

	return false
}

func nameExistInDeploymentJobList(rd *v1.RadixDeployment, name string) bool {
	for _, job := range rd.Spec.Jobs {
		if strings.EqualFold(job.Name, name) {
			return true
		}
	}

	return false
}

// Radix component types used as values for label radix-component-type
const (
	RadixDeploymentComponent RadixComponentType = "component"
	RadixDeploymentJob       RadixComponentType = "job"
)
