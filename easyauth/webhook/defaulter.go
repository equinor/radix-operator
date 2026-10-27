package main

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// +kubebuilder:webhook:name=radixapplication.mutate.easyauth.radix.equinor.com,path=/radix/v1/radixapplication/mutate,mutating=true,failurePolicy=fail,sideEffects=None,groups=radix.equinor.com,resources=radixapplications,verbs=create;update,versions=v1,admissionReviewVersions={v1}

type ExternalCRDDefaulter struct {
	decoder *admission.Decoder
}

// InjectDecoder injects the decoder automatically
func (a *ExternalCRDDefaulter) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}

// Default implements the custom defaulting logic for the other project's CRD
func (a *ExternalCRDDefaulter) Default(ctx context.Context, obj *radixv1.RadixApplication) error {

	// 1. Mutate fields directly using strong Go types!
	// if cr.Spec.SomeField == "" {
	// 	cr.Spec.SomeField = "defaulted-by-my-operator"
	// }

	obj.Labels = map[string]string{
		"testMutate": obj.Name,
	}

	return nil
}

// Ensure interface compliance
var _ admission.Defaulter[*radixv1.RadixApplication] = &ExternalCRDDefaulter{}
