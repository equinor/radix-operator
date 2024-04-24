package utils

import (
	"context"
	"errors"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func GetResourceRequirements(ctx context.Context, deployComponent v1.RadixCommonDeployComponent) corev1.ResourceRequirements {
	return BuildResourceRequirement(ctx, deployComponent.GetResources())
}

func BuildResourceRequirement(ctx context.Context, source *v1.ResourceRequirements) corev1.ResourceRequirements {
	defaultMemoryLimit := defaults.GetDefaultMemoryLimit()
	limits, err := mapResourceList(source.Limits)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("failed to map source limits")
	}
	requests, err := mapResourceList(source.Requests)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("failed to map source limits")
	}

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

func mapResourceList(list v1.ResourceList) (corev1.ResourceList, error) {
	res := corev1.ResourceList{}
	var errs []error

	for name, quantity := range list {
		if quantity != "" {
			val, err := resource.ParseQuantity(quantity)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to parse %s value %v: %w", name, quantity, err))
				continue
			}
			res[corev1.ResourceName(name)] = val
		}
	}

	return res, errors.Join(errs...)
}
