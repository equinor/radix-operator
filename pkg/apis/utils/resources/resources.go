package resources

import (
	corev1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
)

type ResourceOption func(resources *corev1.ResourceRequirements)

// WithMemoryMega sets memory limit and requests to ensure awailable memory
func WithMemoryMega(memory int64) ResourceOption {
	quantity := resourcev1.NewScaledQuantity(memory, resourcev1.Mega)
	return func(resources *corev1.ResourceRequirements) {
		if resources.Limits == nil {
			resources.Limits = corev1.ResourceList{}
		}
		if resources.Requests == nil {
			resources.Requests = corev1.ResourceList{}
		}

		resources.Limits[corev1.ResourceMemory] = *quantity
		resources.Requests[corev1.ResourceMemory] = *quantity
	}
}

// WithCPUMilli sets cpu requests without limit to ensure optimal performance when possible
func WithCPUMilli(cpu int64) ResourceOption {
	quantity := resourcev1.NewScaledQuantity(cpu, resourcev1.Milli)
	return func(resources *corev1.ResourceRequirements) {
		if resources.Requests == nil {
			resources.Requests = corev1.ResourceList{}
		}

		resources.Requests[corev1.ResourceCPU] = *quantity
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
