package models

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// ConvertResourceRequirements Convert resource requirements
func ConvertResourceRequirements(resources corev1.ResourceRequirements) ResourceRequirements {
	return ResourceRequirements{
		Limits:   getResources(resources.Limits),
		Requests: getResources(resources.Requests),
	}
}

// ConvertRadixResourceRequirements Convert resource requirements
func ConvertRadixResourceRequirements(resources v1.ResourceRequirements) ResourceRequirements {
	convertResource := func(resource v1.ResourceList) Resources {
		return Resources{CPU: resource[string(corev1.ResourceCPU)], Memory: resource[string(corev1.ResourceMemory)]}
	}

	return ResourceRequirements{
		Limits:   convertResource(resources.Limits),
		Requests: convertResource(resources.Requests),
	}
}

func getResources(resources corev1.ResourceList) Resources {
	resourceList := Resources{
		CPU:    getResource(resources.Cpu()),
		Memory: getResource(resources.Memory()),
	}
	return resourceList
}

func getResource(resource *resource.Quantity) string {
	if resource == nil {
		return ""
	}
	return resource.String()
}
