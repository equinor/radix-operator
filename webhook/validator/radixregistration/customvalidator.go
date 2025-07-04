package radixregistration

import (
	"context"
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const RadixRegistrationValidatorWebhookPath = "/radix/v1/radixregistration/validation"

//+kubebuilder:webhook:path=/radix/v1/radixregistration/validation,mutating=false,failurePolicy=fail,sideEffects=None,groups=radix.equinor.com,resources=radixregistrations,verbs=create;update,versions=v1,name=validate.radix.equinor.com,admissionReviewVersions={v1}

type radixRegistrationValidator struct {
	validator Validator
}

func NewRadixRegistrationCustomValidator(validator Validator) admission.CustomValidator {
	return &radixRegistrationValidator{
		validator: validator,
	}
}

// ValidateCreate validates the object on creation..
func (v *radixRegistrationValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	rr, ok := obj.(*radixv1.RadixRegistration)
	if !ok {
		return nil, fmt.Errorf("expected a RadixRegistration but got a %T", obj)
	}

	return v.validator.Validate(ctx, rr)
}

// ValidateUpdate validates the object on update.
func (v *radixRegistrationValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	rr, ok := newObj.(*radixv1.RadixRegistration)
	if !ok {
		return nil, fmt.Errorf("expected a RadixRegistration but got a %T", newObj)
	}

	return v.validator.Validate(ctx, rr)
}

// ValidateDelete validates the object on deletion.
// The optional warnings will be added to the response as warning messages.
// Return an error if the object is invalid.
func (v *radixRegistrationValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
