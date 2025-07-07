package genericvalidator_test

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/webhook/validation/genericvalidator"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestNewGenericValidator(t *testing.T) {
	validatorFn := genericvalidator.ValidatorFunc[*v1.RadixRegistration](func(ctx context.Context, obj *v1.RadixRegistration) (admission.Warnings, error) {
		return admission.Warnings{"some warnings"}, errors.New("some error")
	})

	genVal := genericvalidator.NewGenericAdmissionValidator(validatorFn, nil, nil)

	ctx := admission.NewContextWithRequest(t.Context(), admission.Request{})
	actualWarnings, err := genVal.ValidateCreate(ctx, &v1.RadixRegistration{})
	expectedWarnings := admission.Warnings{"some warnings"}

	assert.Equal(t, expectedWarnings, actualWarnings)
	assert.EqualError(t, err, "some error")
}
