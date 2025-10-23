package radixapplication

import (
	"context"
	"errors"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createDNSAliasAvailableValidator(kubeClient client.Client, reservedDNSAliases []string, reservedDNSAppAliases map[string]string) validatorFunc {
	return func(ctx context.Context, ra *radixv1.RadixApplication) (string, error) {
		var errs []error
		list := radixv1.RadixDNSAliasList{}
		err := kubeClient.List(ctx, &list)
		if err != nil {
			return "", err
		}

		for _, dnsAlias := range ra.Spec.DNSAlias {

			existingAliasForDifferentApp := slice.Any(list.Items, func(item radixv1.RadixDNSAlias) bool {
				return item.Spec.AppName != ra.Name && item.Name == dnsAlias.Alias
			})
			if existingAliasForDifferentApp {
				errs = append(errs, fmt.Errorf("dns alias %s is already used. %w", dnsAlias.Alias, ErrDNSAliasAlreadyUsedByAnotherApplication))
			}

			if reservingAppName, aliasReserved := reservedDNSAppAliases[dnsAlias.Alias]; aliasReserved && reservingAppName != ra.Name {
				errs = append(errs, fmt.Errorf("dns alias %s is reserved. %w", dnsAlias.Alias, ErrDNSAliasReservedForRadixPlatformApplication))
			}

			if slice.Any(reservedDNSAliases, func(reservedAlias string) bool { return reservedAlias == dnsAlias.Alias }) {
				errs = append(errs, fmt.Errorf("dns alias %s is reserved. %w", dnsAlias.Alias, ErrDNSAliasReservedForRadixPlatformService))
			}
		}
		return "", errors.Join(errs...)
	}
}

func createDNSAliasValidator() validatorFunc {
	return func(ctx context.Context, ra *radixv1.RadixApplication) (string, error) {
		var errs []error

		for _, dnsAlias := range ra.Spec.DNSAlias {
			if err := validateDNSAliasComponentAndEnvironmentAvailable(ra, dnsAlias.Alias, dnsAlias.Component, dnsAlias.Environment); err != nil {
				errs = append(errs, err)
				continue
			}
			if !doesComponentHaveAPublicPort(ra, dnsAlias.Component) {
				errs = append(errs, fmt.Errorf("component %s is not public. %w", dnsAlias.Component, ErrDNSAliasComponentIsNotMarkedAsPublic))
				continue
			}
		}
		return "", errors.Join(errs...)
	}
}

func createExternalDNSAliasValidator() validatorFunc {
	return func(ctx context.Context, ra *radixv1.RadixApplication) (string, error) {
		var errs []error

		for _, externalAlias := range ra.Spec.DNSExternalAlias {
			if !doesEnvExist(ra, externalAlias.Environment) {
				errs = append(errs, fmt.Errorf("%s: %w", externalAlias.Alias, ErrExternalAliasEnvironmentNotDefined))
			}
			if !doesComponentExistAndEnabled(ra, externalAlias.Component, externalAlias.Environment) {
				errs = append(errs, fmt.Errorf("%s: %w", externalAlias.Alias, ErrExternalAliasComponentNotDefined))
			}

			if !doesComponentHaveAPublicPort(ra, externalAlias.Component) {
				errs = append(errs, fmt.Errorf("%s: %w", externalAlias.Alias, ErrExternalAliasComponentNotMarkedAsPublic))
			}
		}
		return "", errors.Join(errs...)
	}
}

func validateDNSAliasComponentAndEnvironmentAvailable(ra *radixv1.RadixApplication, dnsAlias, component, environment string) error {
	if !doesEnvExist(ra, environment) {
		return fmt.Errorf("%s: %w", dnsAlias, ErrDNSAliasEnvironmentNotDefined)
	}
	if !doesComponentExistAndEnabled(ra, component, environment) {
		return fmt.Errorf("%s: %w", dnsAlias, ErrDNSAliasComponentNotDefinedOrDisabled)
	}
	return nil
}
