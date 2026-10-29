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

// RadixApplicationValidator rejects RadixApplications missing the required annotation when Enforced is true
type RadixApplicationValidator struct {
	Enforced   bool
	Annotation string
}

func RegisterWithManager(mgr ctrl.Manager, v *RadixApplicationValidator) error {
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
	if !v.Enforced {
		return nil
	}
	if _, ok := obj.GetAnnotations()[v.Annotation]; !ok {
		return fmt.Errorf("RadixApplication %s/%s must have the annotation %q", obj.Namespace, obj.Name, v.Annotation)
	}
	return nil
}

var _ admission.Validator[*radixv1.RadixApplication] = &RadixApplicationValidator{}
