package radixregistration

import (
	"context"
	"errors"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// RadixRegistrationValidator defines a validator function for a RadixRegistration
type RadixRegistrationValidator func(radixRegistration *radixv1.RadixRegistration) (string, error)

type Validator struct {
	validators []RadixRegistrationValidator
}

func CreateOnlineValidator(ctx context.Context, client radixclient.Interface, requireAdGroups, requireConfigurationItem bool) Validator {
	return Validator{
		validators: []RadixRegistrationValidator{
			CreateRequireUniqueAppIdValidator(ctx, client),
			CreateRequireAdGroupsValidator(requireAdGroups),
			CreateRequireConfigurationItemValidator(requireConfigurationItem),
		},
	}
}

func CreateOfflineValidator(requireAdGroups, requireConfigurationItem bool) Validator {
	return Validator{
		validators: []RadixRegistrationValidator{
			CreateRequireAdGroupsValidator(requireAdGroups),
			CreateRequireConfigurationItemValidator(requireConfigurationItem),
		},
	}
}

func (validator *Validator) Validate(rr *radixv1.RadixRegistration) (admission.Warnings, error) {
	var errs []error
	var wrns admission.Warnings
	for _, v := range validator.validators {
		wrn, err := v(rr)
		if err != nil {
			errs = append(errs, err)
		}
		if wrn != "" {
			wrns = append(wrns, wrn)
		}
	}

	return wrns, errors.Join(errs...)
}

// RequireAdGroups validates that AdGroups contains minimum one item
func CreateRequireAdGroupsValidator(required bool) RadixRegistrationValidator {
	return func(rr *radixv1.RadixRegistration) (string, error) {
		if len(rr.Spec.AdGroups) == 0 && required {
			return "", ErrAdGroupIsRequired
		}

		if len(rr.Spec.AdGroups) == 0 && !required {
			return WarningAdGroupsShouldHaveAtleastOneItem, nil
		}

		return "", nil
	}
}

func CreateRequireConfigurationItemValidator(required bool) RadixRegistrationValidator {
	return func(rr *radixv1.RadixRegistration) (string, error) {
		if rr.Spec.ConfigurationItem == "" && required {
			return "", ErrConfigurationItemIsRequired
		}

		return "", nil
	}
}

func CreateRequireUniqueAppIdValidator(ctx context.Context, client radixclient.Interface) RadixRegistrationValidator {
	return func(rr *radixv1.RadixRegistration) (string, error) {
		if rr.Spec.AppID.IsZero() {
			return "", nil // AppID is not required
		}

		existingRRs, err := client.RadixV1().RadixRegistrations().List(ctx, metav1.ListOptions{})
		if err != nil {
			return "", nil // No existing RR with this AppID
		}

		for _, existingRR := range existingRRs.Items {
			if existingRR.Spec.AppID == rr.Spec.AppID && existingRR.Name != rr.Name {
				return "", ErrAppIdMustBeUnique
			}
		}

		return "", nil
	}
}
