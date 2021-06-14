package utils

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func GetResourceRequirements(deployComponent v1.RadixCommonDeployComponent) corev1.ResourceRequirements {
	return BuildResourceRequirement(deployComponent.GetResources())
}

func BuildResourceRequirement(source *v1.ResourceRequirements) corev1.ResourceRequirements {
	defaultLimits := map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceName("cpu"):    *defaults.GetDefaultCPULimit(),
		corev1.ResourceName("memory"): *defaults.GetDefaultMemoryLimit(),
	}

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

			if _, hasLimit := limits[resName]; !hasLimit {
				// There is no defined limit, but there is a request
				reqQuantity := requests[resName]
				if reqQuantity.Cmp(defaultLimits[resName]) == 1 {
					// Requested quantity is larger than the default limit
					// We use the requested value as the limit
					limits[resName] = requests[resName].DeepCopy()

					// TODO: If we introduce a hard limit, that should not be exceeded here
				}
			}
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
