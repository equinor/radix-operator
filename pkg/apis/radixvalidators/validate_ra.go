package radixvalidators

import (
	"context"
	"errors"

	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
)

var (
	requiredRadixApplicationValidators = []RadixApplicationValidator{}
)

// RadixApplicationValidator defines a validator function for a RadixApplication
type RadixApplicationValidator func(radixApplication *radixv1.RadixApplication) error

// CanRadixApplicationBeInserted Checks if application config is valid. Returns a single error, if this is the case
func CanRadixApplicationBeInserted(ctx context.Context, radixClient radixclient.Interface, app *radixv1.RadixApplication, dnsAliasConfig *dnsalias.DNSConfig, additionalValidators ...RadixApplicationValidator) error {

	validators := append(requiredRadixApplicationValidators, additionalValidators...)

	return validateRadixApplication(app, validators...)
}

// IsRadixApplicationValid Checks if application config is valid without server validation
func IsRadixApplicationValid(app *radixv1.RadixApplication, additionalValidators ...RadixApplicationValidator) error {
	validators := append(requiredRadixApplicationValidators, additionalValidators...)
	return validateRadixApplication(app, validators...)
}

func validateRadixApplication(radixApplication *radixv1.RadixApplication, validators ...RadixApplicationValidator) error {
	var errs []error
	for _, v := range validators {
		if err := v(radixApplication); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
