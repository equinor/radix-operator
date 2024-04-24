package utils

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func GetResourceRequirements(deployComponent v1.RadixCommonDeployComponent) corev1.ResourceRequirements {
	return BuildResourceRequirement(deployComponent.GetResources())
}

func BuildResourceRequirement(source *v1.ResourceRequirements) corev1.ResourceRequirements {
	defaultMemoryLimit := defaults.GetDefaultMemoryLimit()
	limits := mapResourceList(source.Limits)
	requests := mapResourceList(source.Requests)

	// LimitRanger will set a default Memory Limit of not specified
	// If the default is lower than requested, the Pod *will* break
	_, hasMemLimit := limits[corev1.ResourceMemory]
	memReq, hasMemRequest := requests[corev1.ResourceMemory]
	if hasMemRequest && !hasMemLimit && defaultMemoryLimit != nil {
		// if requested is higher than default limit, set limit
		if memReq.Cmp(*defaultMemoryLimit) == 1 {
			limits[corev1.ResourceMemory] = memReq.DeepCopy()
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

func mapResourceList(list v1.ResourceList) corev1.ResourceList {
	res := corev1.ResourceList{}
	for name, quantity := range list {
		if quantity != "" {
			val, err := resource.ParseQuantity(quantity)
			if err != nil {
				log.Warn().Err(err).Str(name, quantity).Stack().Msg("failed to parse value")
				continue
			}
			res[corev1.ResourceName(name)] = val
		}
	}

	return res
}
