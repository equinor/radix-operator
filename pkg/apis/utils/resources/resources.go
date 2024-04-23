package resources

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
)

func GetResourceRequirements(deployComponent radixv1.RadixCommonDeployComponent) corev1.ResourceRequirements {
	return New(WithDefaults(), WithSource(deployComponent.GetResources()))
}

type ResourceOption func(resources *corev1.ResourceRequirements)

func WithMemory(memory string) ResourceOption {
	return func(resources *corev1.ResourceRequirements) {
		mem, err := resourcev1.ParseQuantity(memory)
		if err != nil {
			log.Warn().Err(err).Str("memory", memory).Stack().Msg("failed to parse memory")
			return
		}

		if resources.Limits == nil {
			resources.Limits = corev1.ResourceList{}
		}
		if resources.Requests == nil {
			resources.Requests = corev1.ResourceList{}
		}

		resources.Limits[corev1.ResourceMemory] = mem
		resources.Requests[corev1.ResourceMemory] = mem
	}
}

func WithCPU(cpu string) ResourceOption {
	return func(resources *corev1.ResourceRequirements) {
		c, err := resourcev1.ParseQuantity(cpu)
		if err != nil {
			log.Warn().Err(err).Str("cpu", cpu).Stack().Msg("failed to parse cpu")
			return
		}

		if resources.Requests == nil {
			resources.Requests = corev1.ResourceList{}
		}

		resources.Requests[corev1.ResourceCPU] = c
	}
}

func WithDefaults() ResourceOption {
	return func(resources *corev1.ResourceRequirements) {
		if resources.Limits == nil {
			resources.Limits = corev1.ResourceList{}
		}
		if resources.Requests == nil {
			resources.Requests = corev1.ResourceList{}
		}

		// TODO: Only set memory limit if request is set, and the deault is higher?

		if defaultMemory := defaults.GetDefaultMemoryLimit(); defaultMemory != nil {
			resources.Limits[corev1.ResourceMemory] = *defaultMemory
			resources.Requests[corev1.ResourceMemory] = *defaultMemory
		}

		// TODO: Only set default CPU if missing?
		if defaultCPU := defaults.GetDefaultCPURequest(); defaultCPU != nil {
			resources.Requests[corev1.ResourceCPU] = *defaultCPU
		}
	}
}

func WithSource(source *radixv1.ResourceRequirements) ResourceOption {
	return func(resources *corev1.ResourceRequirements) {
		if source == nil {
			return
		}

		if source.Limits != nil {
			if resources.Limits == nil {
				resources.Limits = corev1.ResourceList{}
			}

			for key, value := range source.Limits {
				v, err := resourcev1.ParseQuantity(value)
				if err != nil {
					log.Warn().Err(err).Str(key, value).Stack().Msg("failed to parse value")
					continue
				}

				resources.Limits[corev1.ResourceName(key)] = v.DeepCopy()
			}
		}

		if source.Requests != nil {
			if resources.Requests == nil {
				resources.Requests = corev1.ResourceList{}
			}

			for key, value := range source.Requests {
				v, err := resourcev1.ParseQuantity(value)
				if err != nil {
					log.Warn().Err(err).Str(key, value).Stack().Msg("failed to parse value")
					continue
				}

				resources.Requests[corev1.ResourceName(key)] = v.DeepCopy()
			}
		}
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

	// TODO: Enforce equal memory requests and limits when all RadixDeployments are newer
	// If memory limit is set, enforce equal memory requests
	// if limitVal, hasLimit := resources.Limits[corev1.ResourceMemory]; hasLimit {
	// 	resources.Requests[corev1.ResourceMemory] = limitVal.DeepCopy()
	// }

	return resources
}
