package validator

import (
	"context"
	"errors"
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const RadixRegistrationValidatorWebhookPath = "/radix/v1/radixregistration/validater"

//+kubebuilder:webhook:path=/radix/v1/radixregistration/validater,mutating=false,failurePolicy=fail,sideEffects=None,groups=radix.equinor.com,resources=radixregistrations,verbs=create;update,versions=v1,name=validate.radixapplication.radix.equinor.com,admissionReviewVersions={v1}

type RadixRegistrationValidator struct {
	client radixclient.Interface
}

var ErrRadixRegistrationAppIdIsImmutable = errors.New("Radix AppID is immutable and cannot be modified after being set")
var _ admission.CustomValidator = &RadixRegistrationValidator{}

func NewRadixRegistrationValidator(client radixclient.Interface) *RadixRegistrationValidator {
	return &RadixRegistrationValidator{
		client: client,
	}
}

// validate admits a RadixRegistration if its valid
func (v *RadixRegistrationValidator) validate(ctx context.Context, rr *radixv1.RadixRegistration) (admission.Warnings, error) {
	warnings := admission.Warnings{
		"RadixRegistration validation started",
		"fun stuff",
	}
	err := radixvalidators.CanRadixRegistrationBeInserted(context.Background(), v.client, rr)
	if err != nil {
		return warnings, err
	}

	return warnings, nil
}

// ValidateCreate validates the object on creation..
func (v *RadixRegistrationValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	rr, ok := obj.(*radixv1.RadixRegistration)
	if !ok {
		return nil, fmt.Errorf("expected a RadixRegistration but got a %T", obj)
	}

	return v.validate(ctx, rr)
}

// ValidateUpdate validates the object on update.
func (v *RadixRegistrationValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	rr, ok := newObj.(*radixv1.RadixRegistration)
	if !ok {
		return nil, fmt.Errorf("expected a RadixRegistration but got a %T", newObj)
	}

	return v.validate(ctx, rr)
}

// ValidateDelete validates the object on deletion.
// The optional warnings will be added to the response as warning messages.
// Return an error if the object is invalid.
func (v *RadixRegistrationValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
