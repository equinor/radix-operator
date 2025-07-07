package genericvalidator

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type Validator[TObj runtime.Object] interface {
	Validate(ctx context.Context, obj TObj) (warnings admission.Warnings, err error)
}

// ValidatorFunc is a function type that implements the Validator interface.
// It can be used to create inline validators without needing to define a separate struct.
type ValidatorFunc[TObj runtime.Object] func(ctx context.Context, obj TObj) (warnings admission.Warnings, err error)

func (f ValidatorFunc[TObj]) Validate(ctx context.Context, obj TObj) (warnings admission.Warnings, err error) {
	return f(ctx, obj)
}
