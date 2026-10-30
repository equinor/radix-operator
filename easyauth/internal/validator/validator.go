package validator

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const Path = "/radix/v1/radixapplication/validate"

// +kubebuilder:webhook:name=radixapplication.validate.easyauth.radix.equinor.com,path=/radix/v1/radixapplication/validate,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,servicePort=443,groups=radix.equinor.com,resources=radixapplications,verbs=create;update,versions=v1,admissionReviewVersions={v1}

// RadixApplicationValidator rejects RadixApplications missing the required annotation when enforced is true
type RadixApplicationValidator struct {
	annotation string
	enforced   bool
}

// NewRadixApplicationValidator returns a validator that rejects RadixApplications without annotation when enforced is true
func NewRadixApplicationValidator(annotation string, enforced bool) *RadixApplicationValidator {
	return &RadixApplicationValidator{annotation: annotation, enforced: enforced}
}

// SetupWithManager registers the validating webhook with the Manager
func (v *RadixApplicationValidator) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &radixv1.RadixApplication{}).
		WithValidator(v).
		WithValidatorCustomPath(Path).
		Complete()
}

func (v *RadixApplicationValidator) ValidateCreate(_ context.Context, obj *radixv1.RadixApplication) (admission.Warnings, error) {
	return nil, v.validate(obj)
}

func (v *RadixApplicationValidator) ValidateUpdate(_ context.Context, _, newObj *radixv1.RadixApplication) (admission.Warnings, error) {
	return nil, v.validate(newObj)
}

func (v *RadixApplicationValidator) ValidateDelete(_ context.Context, _ *radixv1.RadixApplication) (admission.Warnings, error) {
	return nil, nil
}

func (v *RadixApplicationValidator) validate(obj *radixv1.RadixApplication) error {
	if !v.enforced {
		return nil
	}

	value, ok := obj.GetAnnotations()[v.annotation]
	if !ok {
		return fmt.Errorf("RadixApplication %s must have the annotation %q", obj.Name, v.annotation)
	}
	if value != "" {
		return fmt.Errorf("RadixApplication %s annotation %q must be empty", obj.Name, v.annotation)
	}

	return nil
}

var _ admission.Validator[*radixv1.RadixApplication] = &RadixApplicationValidator{}
