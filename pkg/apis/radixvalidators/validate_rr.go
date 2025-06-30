package radixvalidators

import (
	"context"
	stderrors "errors"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
)

// RadixRegistrationValidator defines a validator function for a RadixRegistration
type RadixRegistrationValidator func(radixRegistration *v1.RadixRegistration) error

// FIX: Add unique AppID Validation, and optional required ADGroup,
// FIX: Add unique repo+configBranch combination
// FIX: Do not allow removing AppID, Creator or Owner from existing RadixRegistration

// RequireAdGroups validates that AdGroups contains minimum one item
func RequireAdGroups(rr *v1.RadixRegistration) error {
	if len(rr.Spec.AdGroups) == 0 {
		return ResourceNameCannotBeEmptyErrorWithMessage("AD groups")
	}

	return nil
}

// CanRadixRegistrationBeInserted Validates RR
func CanRadixRegistrationBeInserted(ctx context.Context, client radixclient.Interface, radixRegistration *v1.RadixRegistration, additionalValidators ...RadixRegistrationValidator) error {
	// cannot be used from admission control - returns the same radix reg that we try to validate
	return validateRadixRegistration(radixRegistration, additionalValidators...)
}

// CanRadixRegistrationBeUpdated Validates update of RR
func CanRadixRegistrationBeUpdated(radixRegistration *v1.RadixRegistration, additionalValidators ...RadixRegistrationValidator) error {
	return validateRadixRegistration(radixRegistration, additionalValidators...)
}

func validateRadixRegistration(radixRegistration *v1.RadixRegistration, validators ...RadixRegistrationValidator) error {
	var errs []error
	for _, v := range validators {
		if err := v(radixRegistration); err != nil {
			errs = append(errs, err)
		}
	}
	return stderrors.Join(errs...)
}
