package resources

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
)

func GetResourceRequirements(deployComponent radixv1.RadixCommonDeployComponent) corev1.ResourceRequirements {
	return BuildResourceRequirement(deployComponent.GetResources()) // New(WithDefaults(), WithSource(deployComponent.GetResources()))
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

func BuildResourceRequirement(source *radixv1.ResourceRequirements) corev1.ResourceRequirements {
	defaultMemoryLimit := defaults.GetDefaultMemoryLimit()
	// defaultLimits := map[corev1.ResourceName]resourcev1.Quantity{
	// 	corev1.ResourceName("memory"): defaultMemoryLimit,
	// }

	// if you only set limit, it will use the same values for request
	limits := corev1.ResourceList{}
	requests := corev1.ResourceList{}

	for name, limit := range source.Limits {
		if limit != "" {
			val, err := resourcev1.ParseQuantity(limit)
			if err != nil {
				log.Warn().Err(err).Str(name, limit).Stack().Msg("failed to parse value")
				continue
			}

			limits[corev1.ResourceName(name)] = val
		}
	}
	for name, request := range source.Requests {
		if request != "" {
			val, err := resourcev1.ParseQuantity(request)
			if err != nil {
				log.Warn().Err(err).Str(name, request).Stack().Msg("failed to parse value")
				continue
			}
			requests[corev1.ResourceName(name)] = val
		}
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

	// for name, req := range source.Requests {
	// 	if req != "" {
	// 		resName := corev1.ResourceName(name)
	// 		requests[resName], _ = resourcev1.ParseQuantity(req)
	//
	// 		if _, hasLimit := limits[resName]; !hasLimit {
	// 			if _, ok := defaultLimits[resName]; !ok {
	// 				continue // No default limit for this resource
	// 			}
	// 			// There is no defined limit, but there is a request
	// 			reqQuantity := requests[resName]
	// 			if reqQuantity.Cmp(defaultLimits[resName]) == 1 {
	// 				// Requested quantity is larger than the default limit
	// 				// We use the requested value as the limit
	// 				limits[resName] = requests[resName].DeepCopy()
	//
	// 				// TODO: If we introduce a hard limit, that should not be exceeded here
	// 			}
	// 		}
	// 	}
	// }

	if len(limits) <= 0 && len(requests) <= 0 {
		return corev1.ResourceRequirements{}
	}

	req := corev1.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
	}

	return req
}
