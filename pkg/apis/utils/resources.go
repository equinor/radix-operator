package utils

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func GetResourceRequirements(deployComponent v1.RadixCommonDeployComponent) corev1.ResourceRequirements {
	return BuildResourceRequirement(deployComponent.GetResources())
}

func BuildResourceRequirement(source *v1.ResourceRequirements) corev1.ResourceRequirements {
	// if you only set limit, it will use the same values for request
	limits := corev1.ResourceList{}
	requests := corev1.ResourceList{}

	for name, limit := range source.Limits {
		if limit != "" {
			resName := corev1.ResourceName(name)
			limits[resName], _ = resource.ParseQuantity(limit)
		}

		// TODO: We probably should check some hard limit that cannot by exceeded here
	}

	for name, req := range source.Requests {
		if req != "" {
			resName := corev1.ResourceName(name)
			requests[resName], _ = resource.ParseQuantity(req)
		}
	}

	if len(limits) <= 0 && len(requests) <= 0 {
		return corev1.ResourceRequirements{}
	}

	req := corev1.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
	}

	return req
}
