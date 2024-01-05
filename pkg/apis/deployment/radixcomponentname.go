package deployment

import (
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RadixComponentName defines values for radix-component label
type RadixComponentName string

// RadixComponentNameFromComponentLabel returns RadixComponentName from object's radix-component label
func RadixComponentNameFromComponentLabel(object metav1.Object) (componentName RadixComponentName, ok bool) {
	return radixComponentNameFromLabel(object, kube.RadixComponentLabel)
}

// RadixComponentNameFromAuxComponentLabel returns RadixComponentName from object's radix-aux-component label
func RadixComponentNameFromAuxComponentLabel(object metav1.Object) (componentName RadixComponentName, ok bool) {
	return radixComponentNameFromLabel(object, kube.RadixAuxiliaryComponentLabel)
}

func radixComponentNameFromLabel(object metav1.Object, label string) (componentName RadixComponentName, ok bool) {
	var nameLabelValue string
	labels := object.GetLabels()

	nameLabelValue, labelOk := labels[label]
	if !labelOk {
		return "", false
	}

	return RadixComponentName(nameLabelValue), true
}

// ExistInDeploymentSpec checks if RadixDeployment has any component or job with this name
func (t RadixComponentName) ExistInDeploymentSpec(rd *v1.RadixDeployment) bool {
	return t.ExistInDeploymentSpecComponentList(rd) || t.ExistInDeploymentSpecJobList(rd)
}

func (t RadixComponentName) GetCommonDeployComponent(rd *v1.RadixDeployment) v1.RadixCommonDeployComponent {
	if comp := t.findInDeploymentSpecComponentList(rd); comp != nil {
		return comp
	}

	if job := t.findInDeploymentSpecJobList(rd); job != nil {
		return job
	}

	return nil
}

// CommonDeployComponentHasPorts Checks id the deploy component has regular or schedule ports
func (t RadixComponentName) CommonDeployComponentHasPorts(rd *v1.RadixDeployment) bool {
	if comp := t.findInDeploymentSpecComponentList(rd); comp != nil {
		return len(comp.GetPorts()) > 0
	}
	for _, job := range rd.Spec.Jobs {
		if strings.EqualFold(job.Name, string(t)) {
			return len(job.GetPorts()) > 0 || job.SchedulerPort != nil
		}
	}
	return false
}

// ExistInDeploymentSpecComponentList checks if RadixDeployment has any component with this name
func (t RadixComponentName) ExistInDeploymentSpecComponentList(rd *v1.RadixDeployment) bool {
	return t.findInDeploymentSpecComponentList(rd) != nil
}

// ExistInDeploymentSpecJobList checks if RadixDeployment has any job with this name
func (t RadixComponentName) ExistInDeploymentSpecJobList(rd *v1.RadixDeployment) bool {
	return t.findInDeploymentSpecJobList(rd) != nil
}

func (t RadixComponentName) findInDeploymentSpecComponentList(rd *v1.RadixDeployment) v1.RadixCommonDeployComponent {
	for _, component := range rd.Spec.Components {
		if strings.EqualFold(component.Name, string(t)) {
			return &component
		}
	}

	return nil
}

func (t RadixComponentName) findInDeploymentSpecJobList(rd *v1.RadixDeployment) v1.RadixCommonDeployComponent {
	for _, job := range rd.Spec.Jobs {
		if strings.EqualFold(job.Name, string(t)) {
			return &job
		}
	}

	return nil
}
