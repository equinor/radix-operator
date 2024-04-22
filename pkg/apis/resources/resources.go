package resources

import (
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
)

type ResourceOption func(resources *corev1.ResourceRequirements)

func WithMemory(memory string) ResourceOption {
	mem, err := resourcev1.ParseQuantity(memory)
	if err != nil {
		log.Error().Err(err).Str("memory", memory).Stack().Msg("failed to parse memory")
	}

	return func(resources *corev1.ResourceRequirements) {
		if resources.Limits == nil {
			resources.Limits = corev1.ResourceList{}
		}

		resources.Limits[corev1.ResourceMemory] = mem
	}
}
func WithCPU(cpu string) ResourceOption {
	c, err := resourcev1.ParseQuantity(cpu)
	if err != nil {
		log.Error().Err(err).Str("cpu", cpu).Stack().Msg("failed to parse cpu")
	}

	return func(resources *corev1.ResourceRequirements) {
		if resources.Requests == nil {
			resources.Requests = corev1.ResourceList{}
		}

		resources.Requests[corev1.ResourceCPU] = c
	}
}

func New(options ...ResourceOption) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}

	for _, o := range options {
		o(&resources)
	}

	// If request is higher than limit, set limit equal to request
	for key, requestVal := range resources.Requests {
		if limitVal, hasLimit := resources.Limits[key]; hasLimit {
			if requestVal.Cmp(limitVal) == 1 {
				resources.Limits[key] = requestVal.DeepCopy()
			}
		}
	}

	// If memory limit is set, enforce equal memory requests
	if limitVal, hasLimit := resources.Limits[corev1.ResourceMemory]; hasLimit {
		resources.Requests[corev1.ResourceMemory] = limitVal.DeepCopy()
	}

	return resources
}
