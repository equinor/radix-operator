package radixbatchjob

import (
	"context"
	"errors"
	"fmt"
	"slices"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	ErrCPUResourceRequirementTooHigh = errors.New("cpu resource requirement cannot exceed 1k")
	ErrInvalidResourceFormat         = errors.New("invalid resource format")
	ErrInvalidResourceType           = errors.New("invalid resource type")
	ErrNegativeResourceQuantity      = errors.New("resource quantity cannot be negative")
	ErrRequestedResourceExceedsLimit = errors.New("requested resource exceeds limit")
)

var (
	validResourceTypes = []string{"cpu", "memory"}
	maximumCPUQuantity = resource.MustParse("1k")
)

func createResourceRequirementsValidator() validatorFunc {
	return func(_ context.Context, radixBatch *radixv1.RadixBatch) ([]string, []error) {
		var errs []error
		for jobIndex, job := range radixBatch.Spec.Jobs {
			if job.Resources == nil {
				continue
			}

			errs = append(errs, validateResourceRequirements(job.Resources, jobIndex, job.Name)...)
		}

		return nil, errs
	}
}

func validateResourceRequirements(resources *radixv1.ResourceRequirements, jobIndex int, jobName string) []error {
	limits, errs := validateResourceList(resources.Limits, "limits", jobIndex, jobName)
	requests, requestErrors := validateResourceList(resources.Requests, "requests", jobIndex, jobName)
	errs = append(errs, requestErrors...)

	for resourceName, request := range requests {
		if limit, exists := limits[resourceName]; exists && request.Cmp(limit) > 0 {
			errs = append(errs, fmt.Errorf("job %q at index %d resource %s request %s exceeds limit %s: %w", jobName, jobIndex, resourceName, request.String(), limit.String(), ErrRequestedResourceExceedsLimit))
		}
	}

	return errs
}

func validateResourceList(resources radixv1.ResourceList, field string, jobIndex int, jobName string) (map[string]resource.Quantity, []error) {
	quantities := make(map[string]resource.Quantity, len(resources))
	var errs []error
	for resourceName, value := range resources {
		if !slices.Contains(validResourceTypes, resourceName) {
			errs = append(errs, fmt.Errorf("job %q at index %d resources.%s contains unsupported resource %q; only cpu and memory are allowed: %w", jobName, jobIndex, field, resourceName, ErrInvalidResourceType))
			continue
		}

		quantity, err := resource.ParseQuantity(value)
		if err != nil {
			errs = append(errs, fmt.Errorf("job %q at index %d resources.%s.%s has invalid quantity %q: %w", jobName, jobIndex, field, resourceName, value, ErrInvalidResourceFormat))
			continue
		}
		quantities[resourceName] = quantity

		if quantity.Sign() < 0 {
			errs = append(errs, fmt.Errorf("job %q at index %d resources.%s.%s has negative quantity %q: %w", jobName, jobIndex, field, resourceName, value, ErrNegativeResourceQuantity))
		}
		if resourceName == "cpu" && quantity.Cmp(maximumCPUQuantity) > 0 {
			errs = append(errs, fmt.Errorf("job %q at index %d resources.%s.cpu quantity %q is too high: %w", jobName, jobIndex, field, value, ErrCPUResourceRequirementTooHigh))
		}
	}

	return quantities, errs
}
