package radixvalidators

import (
	"context"
	stderrors "errors"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RadixRegistrationValidator defines a validator function for a RadixRegistration
type RadixRegistrationValidator func(radixRegistration *radixv1.RadixRegistration) error

// ValidateRadixRegistration Validates update of RR
func ValidateRadixRegistration(radixRegistration *radixv1.RadixRegistration, additionalValidators ...RadixRegistrationValidator) error {
	return validateRadixRegistration(radixRegistration, additionalValidators...)
}

func validateRadixRegistration(radixRegistration *radixv1.RadixRegistration, validators ...RadixRegistrationValidator) error {
	var errs []error
	for _, v := range validators {
		if err := v(radixRegistration); err != nil {
			errs = append(errs, err)
		}
	}
	return stderrors.Join(errs...)
}

// RequireAdGroups validates that AdGroups contains minimum one item
func RequireAdGroups(rr *radixv1.RadixRegistration) error {
	if len(rr.Spec.AdGroups) == 0 {
		return ErrAdGroupIsRequired
	}

	return nil
}

func RequireConfigurationItem(rr *radixv1.RadixRegistration) error {
	if rr.Spec.ConfigurationItem == "" {
		return ErrConfigurationItemIsRequired
	}

	return nil
}

func CreateRequireUniqueAppIdValidator(ctx context.Context, client radixclient.Interface) RadixRegistrationValidator {
	return func(rr *radixv1.RadixRegistration) error {
		return validateUniqueAppId(ctx, client, rr)
	}
}

func validateUniqueAppId(ctx context.Context, client radixclient.Interface, rr *radixv1.RadixRegistration) error {
	if rr.Spec.AppID.IsZero() {
		return nil // AppID is not required
	}

	existingRRs, err := client.RadixV1().RadixRegistrations().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil // No existing RR with this AppID
	}

	for _, existingRR := range existingRRs.Items {
		if existingRR.Spec.AppID == rr.Spec.AppID && existingRR.Name != rr.Name {
			return ErrAppIdMustBeUnique
		}
	}

	return nil
}
