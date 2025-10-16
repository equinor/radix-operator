package radixapplication

import (
	"context"
	"errors"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/webhook/validation/genericvalidator"
	"github.com/rs/zerolog/log"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// validatorFunc defines a validatorFunc function for a RadixApplication
type validatorFunc func(ctx context.Context, radixApplication *radixv1.RadixApplication) (string, error)

type Validator struct {
	validators []validatorFunc
}

var _ genericvalidator.Validator[*radixv1.RadixApplication] = &Validator{}

func CreateOnlineValidator(client client.Client, dnsConfig *dnsalias.DNSConfig) *Validator {
	return &Validator{
		validators: []validatorFunc{
			externalDNSAliasValidator,
			createRRExistValidator(client),
			createDNSAliasValidator(client, dnsConfig),
		},
	}
}

func CreateOfflineValidator() Validator {
	return Validator{
		validators: []validatorFunc{
			externalDNSAliasValidator,
		},
	}
}

func (validator *Validator) Validate(ctx context.Context, ra *radixv1.RadixApplication) (admission.Warnings, error) {
	var errs []error
	var wrns admission.Warnings
	for _, v := range validator.validators {
		wrn, err := v(ctx, ra)
		if err != nil {
			errs = append(errs, err)
		}
		if wrn != "" {
			wrns = append(wrns, wrn)
		}
	}

	return wrns, errors.Join(errs...)
}

func createRRExistValidator(kubeClient client.Client) validatorFunc {
	return func(ctx context.Context, ra *radixv1.RadixApplication) (string, error) {
		err := kubeClient.Get(ctx, client.ObjectKey{Name: ra.Name}, &radixv1.RadixRegistration{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return "", ErrNoRadixApplication
			}

			log.Ctx(ctx).Error().Err(err).Msg("failed to list existing RadixRegistrations")
			return "", err // Something went wrong while listing existing RadixRegistrations, let the user try again
		}

		return "", nil
	}
}

func createDNSAliasValidator(kubeClient client.Client, dnsAliasConfig *dnsalias.DNSConfig) validatorFunc {
	return func(ctx context.Context, ra *radixv1.RadixApplication) (string, error) {
		var errs []error
		list := radixv1.RadixDNSAliasList{}
		err := kubeClient.List(ctx, &list)
		if err != nil {
			return "", err
		}
		uniqueAliasNames := make(map[string]struct{})
		for _, dnsAlias := range ra.Spec.DNSAlias {
			if _, ok := uniqueAliasNames[dnsAlias.Alias]; ok {
				errs = append(errs, ErrDuplicateDNSAlias)
			}
			uniqueAliasNames[dnsAlias.Alias] = struct{}{}
			if err = validateDNSAliasComponentAndEnvironmentAvailable(ra, dnsAlias.Alias, dnsAlias.Component, dnsAlias.Environment); err != nil {
				errs = append(errs, err)
				continue
			}
			if !doesComponentHaveAPublicPort(ra, dnsAlias.Component) {
				errs = append(errs, fmt.Errorf("component %s is not public. %w", dnsAlias.Component, ErrDNSAliasComponentIsNotMarkedAsPublic))
				continue
			}

			existingAliasForDifferentApp := slice.Any(list.Items, func(item radixv1.RadixDNSAlias) bool {
				return item.Spec.AppName != ra.Name && item.Name == dnsAlias.Alias
			})
			if existingAliasForDifferentApp {
				errs = append(errs, fmt.Errorf("dns alias %s is already used. %w", dnsAlias.Alias, ErrDNSAliasAlreadyUsedByAnotherApplication))
			}

			if reservingAppName, aliasReserved := dnsAliasConfig.ReservedAppDNSAliases[dnsAlias.Alias]; aliasReserved && reservingAppName != ra.Name {
				errs = append(errs, fmt.Errorf("dns alias %s is reserved. %w", dnsAlias.Alias, ErrDNSAliasReservedForRadixPlatformApplication))
			}
			if slice.Any(dnsAliasConfig.ReservedDNSAliases, func(reservedAlias string) bool { return reservedAlias == dnsAlias.Alias }) {
				errs = append(errs, fmt.Errorf("dns alias %s is reserved. %w", dnsAlias.Alias, ErrDNSAliasReservedForRadixPlatformService))
			}
		}
		return "", errors.Join(errs...)
	}
}

func externalDNSAliasValidator(ctx context.Context, ra *radixv1.RadixApplication) (string, error) {
	var errs []error

	distinctAlias := make(map[string]bool)
	for _, externalAlias := range ra.Spec.DNSExternalAlias {
		if _, ok := distinctAlias[externalAlias.Alias]; ok {
			errs = append(errs, fmt.Errorf("%s: %w", externalAlias.Alias, ErrExternalAliasDuplicate))
		}
		distinctAlias[externalAlias.Alias] = true

		if !doesEnvExistInRA(ra, externalAlias.Environment) {
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

func validateDNSAliasComponentAndEnvironmentAvailable(ra *radixv1.RadixApplication, dnsAlias, component, environment string) error {
	if !doesEnvExistInRA(ra, environment) {
		return fmt.Errorf("%s: %w", dnsAlias, ErrDNSAliasEnvironmentNotDefined)
	}
	if !doesComponentExistAndEnabled(ra, component, environment) {
		return fmt.Errorf("%s: %w", dnsAlias, ErrDNSAliasComponentNotDefinedOrDisabled)
	}
	return nil
}

func doesEnvExistInRA(app *radixv1.RadixApplication, name string) bool {
	return slice.Any(app.Spec.Environments, func(e radixv1.Environment) bool { return e.Name == name })
}

func doesComponentExistAndEnabled(app *radixv1.RadixApplication, componentName string, environment string) bool {
	for _, component := range app.Spec.Components {
		if component.Name == componentName {
			environmentConfig := component.GetEnvironmentConfigByName(environment)
			return component.GetEnabledForEnvironmentConfig(environmentConfig)
		}
	}
	return false
}

func doesComponentHaveAPublicPort(app *radixv1.RadixApplication, name string) bool {
	for _, component := range app.Spec.Components {
		if component.Name == name {
			//nolint:staticcheck
			return component.Public || component.PublicPort != ""
		}
	}
	return false
}
