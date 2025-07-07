package genericvalidator

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/utils/generic"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ webhook.CustomValidator = &AdmissionValidator[runtime.Object]{}


type AdmissionValidator[TObj runtime.Object] struct {
	CreateValidation Validator[TObj]
	UpdateValidation Validator[TObj]
	DeleteValidation Validator[TObj]
}

func NewGenericAdmissionValidator[TObj runtime.Object](createValidator Validator[TObj], updateValidator Validator[TObj], deleteValidator Validator[TObj]) *AdmissionValidator[TObj] {
	return &AdmissionValidator[TObj]{
		CreateValidation: createValidator,
		UpdateValidation: updateValidator,
		DeleteValidation: deleteValidator,
	}
}

func (v *AdmissionValidator[TObj]) Register(mgr manager.Manager, path string) {
	obj := generic.InstantiateGenericStruct[TObj]()
	mgr.GetWebhookServer().Register(path, admission.WithCustomValidator(mgr.GetScheme(), obj, v))
	log.Info().Str("path", path).Str("kind", obj.GetObjectKind().GroupVersionKind().Kind).Str("version", obj.GetObjectKind().GroupVersionKind().Version).Msg("registered admission validator")
}
func (v *AdmissionValidator[TObj]) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	request, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, err
	}

	opCtx := zerolog.Ctx(ctx).With().
		Str("operation", "create").
		Stringer("kind", obj.GetObjectKind().GroupVersionKind()).
		Str("name", request.Name).
		Str("namespace", request.Namespace).
		Logger().
		WithContext(ctx)

	return v.runValidation(opCtx, obj, v.CreateValidation)
}

func (v *AdmissionValidator[TObj]) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	request, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, err
	}

	opCtx := zerolog.Ctx(ctx).With().
		Str("operation", "update").
		Stringer("kind", newObj.GetObjectKind().GroupVersionKind()).
		Str("name", request.Name).
		Str("namespace", request.Namespace).
		Logger().
		WithContext(ctx)

	return v.runValidation(opCtx, newObj, v.UpdateValidation)
}

func (v *AdmissionValidator[TObj]) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	request, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, err
	}

	opCtx := zerolog.Ctx(ctx).With().
		Str("operation", "delete").
		Stringer("kind", obj.GetObjectKind().GroupVersionKind()).
		Str("name", request.Name).
		Str("namespace", request.Namespace).
		Logger().
		WithContext(ctx)

	return v.runValidation(opCtx, obj, v.DeleteValidation)
}

func (v *AdmissionValidator[TObj]) runValidation(ctx context.Context, obj runtime.Object, validator Validator[TObj]) (admission.Warnings, error) {
	log.Ctx(ctx).Debug().Msg("starting validation")
	tobj, ok := obj.(TObj)
	if !ok {
		log.Ctx(ctx).Error().Msg("unknown object type")
		return nil, nil
	}

	if validator == nil {
		return nil, nil
	}

	warnings, err := validator.Validate(ctx, tobj)
	log.Ctx(ctx).Info().Strs("warnings", warnings).Err(err).Msg("admission controll completed")
	return warnings, err
}
