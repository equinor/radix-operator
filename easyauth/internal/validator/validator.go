package validator

import (
	"context"
	"fmt"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/equinor/radix-operator/easyauth/internal/oauth"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const Path = "/radix/v1/radixapplication/validate"

// RadixApplicationValidator validates the EasyAuth annotations of RadixApplications, and rejects RadixApplications
// missing the required annotation when enforced is true
type RadixApplicationValidator struct {
	annotation                  string
	allowedIdentitiesAnnotation string
	defaultAllowedGroups        []string
	enforced                    bool
}

// NewRadixApplicationValidator returns a validator that requires annotation to be empty, and rejects RadixApplications
// with annotation that have no allowed groups in allowedIdentitiesAnnotation or defaultAllowedGroups, or invalid
// groups in allowedIdentitiesAnnotation. RadixApplications without annotation are rejected when enforced is true.
func NewRadixApplicationValidator(annotation, allowedIdentitiesAnnotation string, defaultAllowedGroups []string, enforced bool) *RadixApplicationValidator {
	return &RadixApplicationValidator{
		annotation:                  annotation,
		allowedIdentitiesAnnotation: allowedIdentitiesAnnotation,
		defaultAllowedGroups:        defaultAllowedGroups,
		enforced:                    enforced,
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
	annotations := obj.GetAnnotations()
	value, ok := annotations[v.annotation]

	if !ok {
		if v.enforced {
			return nil, fmt.Errorf("RadixApplication %s must have the annotation %q", obj.Name, v.annotation)
		}
		if _, ok := annotations[v.allowedIdentitiesAnnotation]; ok {
			return nil, fmt.Errorf("RadixApplication %s annotation %q requires annotation %q", obj.Name, v.allowedIdentitiesAnnotation, v.annotation)
		}
		return nil, nil
	}

	if value != "" {
		return nil, fmt.Errorf("RadixApplication %s annotation %q must be empty", obj.Name, v.annotation)
	}

	groups := oauth.SplitIdentities(annotations[v.allowedIdentitiesAnnotation])
	if len(groups) == 0 {
		groups = v.defaultAllowedGroups
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("RadixApplication %s must have the annotation %q, since no default allowed identities are configured", obj.Name, v.allowedIdentitiesAnnotation)
	}
	for _, group := range groups {
		if group == "" || group != strings.TrimSpace(group) {
			return nil, fmt.Errorf("RadixApplication %s annotation %q must be a comma separated list of groups without spaces around them, got %q", obj.Name, v.allowedIdentitiesAnnotation, group)
		}
	}

	return nil, nil
}

var _ admission.Validator[*radixv1.RadixApplication] = &RadixApplicationValidator{}
