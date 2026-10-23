package radixbatchjob

import (
	"context"
	"errors"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/webhook/validation/genericvalidator"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type validatorFunc func(ctx context.Context, radixBatch *radixv1.RadixBatch) ([]string, []error)

type Validator struct {
	validators []validatorFunc
}

var _ genericvalidator.Validator[*radixv1.RadixBatch] = &Validator{}

func CreateValidator() *Validator {
	return &Validator{
		validators: []validatorFunc{
			createResourceRequirementsValidator(),
		},
	}
}

func (validator *Validator) Validate(ctx context.Context, radixBatch *radixv1.RadixBatch) (admission.Warnings, error) {
	var errs []error
	var warnings admission.Warnings
	for _, validate := range validator.validators {
		validatorWarnings, validatorErrors := validate(ctx, radixBatch)
		warnings = append(warnings, validatorWarnings...)
		errs = append(errs, validatorErrors...)
	}

	return warnings, errors.Join(errs...)
}
