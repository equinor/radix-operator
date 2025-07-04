package radixregistration

import (
	"context"
	"errors"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type Validator interface {
	Validate(ctx context.Context, rr *radixv1.RadixRegistration) (admission.Warnings, error)
}

type compositeValidator struct {
	validators []Validator
}

func CreateOnlineValidator(client radixclient.Interface, requireAdGroups, requireConfigurationItem bool) Validator {
	return &compositeValidator{
		validators: []Validator{
			uniqueAppIdValidator{client: client},
			adGroupsValidator{required: requireAdGroups},
			configurationItemValidator{required: requireConfigurationItem},
		},
	}
}

func CreateOfflineValidator(requireAdGroups, requireConfigurationItem bool) Validator {
	return &compositeValidator{
		validators: []Validator{
			adGroupsValidator{required: requireAdGroups},
			configurationItemValidator{required: requireConfigurationItem},
		},
	}
}

func (validator *compositeValidator) Validate(ctx context.Context, rr *radixv1.RadixRegistration) (admission.Warnings, error) {
	var errs []error
	var wrns admission.Warnings
	for _, v := range validator.validators {
		wrn, err := v.Validate(ctx, rr)
		errs = append(errs, err)
		wrns = append(wrns, wrn...)
	}

	return wrns, errors.Join(errs...)
}

type adGroupsValidator struct {
	required bool
}

func (v adGroupsValidator) Validate(_ context.Context, rr *radixv1.RadixRegistration) (admission.Warnings, error) {
	if len(rr.Spec.AdGroups) == 0 && v.required {
		if v.required {
			return nil, ErrAdGroupIsRequired
		}
		return []string{WarningAdGroupsShouldHaveAtleastOneItem}, nil
	}

	return nil, nil
}

type configurationItemValidator struct {
	required bool
}

func (v configurationItemValidator) Validate(_ context.Context, rr *radixv1.RadixRegistration) (admission.Warnings, error) {
	if rr.Spec.ConfigurationItem == "" && v.required {
		return nil, ErrConfigurationItemIsRequired
	}

	return nil, nil
}

type uniqueAppIdValidator struct {
	client radixclient.Interface
}

func (v uniqueAppIdValidator) Validate(ctx context.Context, rr *radixv1.RadixRegistration) (admission.Warnings, error) {
	if rr.Spec.AppID.IsZero() {
		return nil, nil // AppID is not required
	}

	existingRRs, err := v.client.RadixV1().RadixRegistrations().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err // No existing RR with this AppID
	}

	for _, existingRR := range existingRRs.Items {
		if existingRR.Spec.AppID == rr.Spec.AppID && existingRR.Name != rr.Name {
			return nil, ErrAppIdMustBeUnique
		}
	}

	return nil, nil
}
