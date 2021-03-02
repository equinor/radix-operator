package deployment

import (
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RadixComponentName defines values for radix-component-type label
type RadixComponentName string

// NewRadixComponentNameFromLabels returns RadixComponentName from labels defined for the object
func NewRadixComponentNameFromLabels(object metav1.Object) (componentName RadixComponentName, ok bool) {
	var nameLabelValue string
	labels := object.GetLabels()

	nameLabelValue, labelOk := labels[kube.RadixComponentLabel]
	if !labelOk {
		return "", false
	}

	return RadixComponentName(nameLabelValue), true
}

// ExistInDeploymentSpec checks if RadixDeployment has any component or job with this name
func (t RadixComponentName) ExistInDeploymentSpec(rd *v1.RadixDeployment) bool {
	return t.ExistInDeploymentSpecComponentList(rd) || t.ExistInDeploymentSpecJobList(rd)
}

// ExistInDeploymentSpecComponentList checks if RadixDeployment has any component with this name
func (t RadixComponentName) ExistInDeploymentSpecComponentList(rd *v1.RadixDeployment) bool {
	for _, component := range rd.Spec.Components {
		if strings.EqualFold(component.Name, string(t)) {
			return true
		}
	}

	return false
}

// ExistInDeploymentSpecJobList checks if RadixDeployment has any job with this name
func (t RadixComponentName) ExistInDeploymentSpecJobList(rd *v1.RadixDeployment) bool {
	for _, job := range rd.Spec.Jobs {
		if strings.EqualFold(job.Name, string(t)) {
			return true
		}
	}

	return false
}
