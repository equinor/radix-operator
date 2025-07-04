package validator

import (
	"context"
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/webhook/validator/radixregistration"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const RadixRegistrationValidatorWebhookPath = "/radix/v1/radixregistration/validation"

//+kubebuilder:webhook:path=/radix/v1/radixregistration/validation,mutating=false,failurePolicy=fail,sideEffects=None,groups=radix.equinor.com,resources=radixregistrations,verbs=create;update,versions=v1,name=validate.radix.equinor.com,admissionReviewVersions={v1}

type RadixRegistrationValidator struct {
	validator radixregistration.Validator
}

var _ admission.CustomValidator = &RadixRegistrationValidator{}

func NewRadixRegistrationValidator(ctx context.Context, client radixclient.Interface, requireAdGroups, requireConfigurationItem bool) *RadixRegistrationValidator {
	return &RadixRegistrationValidator{
		validator: radixregistration.CreateOnlineValidator(ctx, client, requireAdGroups, requireConfigurationItem),
	}
}

// ValidateCreate validates the object on creation..
func (v *RadixRegistrationValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	rr, ok := obj.(*radixv1.RadixRegistration)
	if !ok {
		return nil, fmt.Errorf("expected a RadixRegistration but got a %T", obj)
	}

	return v.validator.Validate(rr)
}

// ValidateUpdate validates the object on update.
func (v *RadixRegistrationValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	rr, ok := newObj.(*radixv1.RadixRegistration)
	if !ok {
		return nil, fmt.Errorf("expected a RadixRegistration but got a %T", newObj)
	}

	return v.validator.Validate(rr)
}

// ValidateDelete validates the object on deletion.
// The optional warnings will be added to the response as warning messages.
// Return an error if the object is invalid.
func (v *RadixRegistrationValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
