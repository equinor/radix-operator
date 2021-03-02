package deployment

import (
	"strings"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// RadixComponentName defines values for radix-component-type label
type RadixComponentName string

// ExistInDeploymentSpec checks if RadixDeployment has any component or job with this name
func (t RadixComponentName) ExistInDeploymentSpec(rd *v1.RadixDeployment) bool {
	return t.ExistInDeploymentComponentList(rd) || t.ExistInDeploymentJobList(rd)
}

// ExistInDeploymentComponentList checks if RadixDeployment has any component with this name
func (t RadixComponentName) ExistInDeploymentComponentList(rd *v1.RadixDeployment) bool {
	for _, component := range rd.Spec.Components {
		if strings.EqualFold(component.Name, string(t)) {
			return true
		}
	}

	return false
}

// ExistInDeploymentJobList checks if RadixDeployment has any job with this name
func (t RadixComponentName) ExistInDeploymentJobList(rd *v1.RadixDeployment) bool {
	for _, job := range rd.Spec.Jobs {
		if strings.EqualFold(job.Name, string(t)) {
			return true
		}
	}

	return false
}
