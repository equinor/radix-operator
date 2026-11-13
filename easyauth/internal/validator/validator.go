package validator

import (
	"context"
	"fmt"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/equinor/radix-operator/easyauth/internal/annotations"
	"github.com/equinor/radix-operator/easyauth/internal/oauth"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const Path = "/radix/v1/radixapplication/validate"

// RadixApplicationValidator validates the EasyAuth annotations of RadixApplications, and rejects RadixApplications
// missing the required annotation when enforced is true
type RadixApplicationValidator struct {
	defaultAllowedGroups []string
	enforced             bool
}

// NewRadixApplicationValidator returns a validator that requires annotations.EasyAuth to be empty, and rejects
// RadixApplications with annotations.EasyAuth that have no allowed groups in annotations.AllowedIdentities or
// defaultAllowedGroups, or invalid groups in annotations.AllowedIdentities. RadixApplications without
// annotations.EasyAuth are rejected when enforced is true.
func NewRadixApplicationValidator(defaultAllowedGroups []string, enforced bool) *RadixApplicationValidator {
	return &RadixApplicationValidator{
		defaultAllowedGroups: defaultAllowedGroups,
		enforced:             enforced,
	}
}

// SetupWithManager registers the validating webhook with the Manager
func (v *RadixApplicationValidator) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &radixv1.RadixApplication{}).
		WithValidator(v).
		WithValidatorCustomPath(Path).
		Complete()
}

func (v *RadixApplicationValidator) ValidateCreate(_ context.Context, obj *radixv1.RadixApplication) (admission.Warnings, error) {
	return v.validate(obj)
}

func (v *RadixApplicationValidator) ValidateUpdate(_ context.Context, _, newObj *radixv1.RadixApplication) (admission.Warnings, error) {
	return v.validate(newObj)
}

func (v *RadixApplicationValidator) ValidateDelete(_ context.Context, _ *radixv1.RadixApplication) (admission.Warnings, error) {
	return nil, nil
}

func (v *RadixApplicationValidator) validate(obj *radixv1.RadixApplication) (admission.Warnings, error) {
	objAnnotations := obj.GetAnnotations()
	value, ok := objAnnotations[annotations.EasyAuth]

	if !ok {
		if v.enforced {
			return nil, fmt.Errorf("RadixApplication %s must have the annotation %q", obj.Name, annotations.EasyAuth)
		}
		if _, ok := objAnnotations[annotations.AllowedIdentities]; ok {
			return nil, fmt.Errorf("RadixApplication %s annotation %q requires annotation %q", obj.Name, annotations.AllowedIdentities, annotations.EasyAuth)
		}
		return nil, nil
	}

	if value != "" {
		return nil, fmt.Errorf("RadixApplication %s annotation %q must be empty", obj.Name, annotations.EasyAuth)
	}

	groups := oauth.SplitIdentities(objAnnotations[annotations.AllowedIdentities])
	if len(groups) == 0 {
		groups = v.defaultAllowedGroups
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("RadixApplication %s must have the annotation %q, since no default allowed identities are configured", obj.Name, annotations.AllowedIdentities)
	}
	for _, group := range groups {
		if group == "" || group != strings.TrimSpace(group) {
			return nil, fmt.Errorf("RadixApplication %s annotation %q must be a comma separated list of groups without spaces around them, got %q", obj.Name, annotations.AllowedIdentities, group)
		}
	}

	return nil, nil
}

var _ admission.Validator[*radixv1.RadixApplication] = &RadixApplicationValidator{}
