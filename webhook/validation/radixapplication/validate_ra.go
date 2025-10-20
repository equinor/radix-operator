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
	"k8s.io/apimachinery/pkg/api/resource"
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
			checkDeprecatedPublicUsage,
			createComponentValidator(),
			createExternalDNSAliasValidator(),
			createDNSAliasValidator(),
			createRRExistValidator(client),
			createDNSAliasAvailableValidator(client, dnsConfig),
		},
	}
}

func CreateOfflineValidator() Validator {
	return Validator{
		validators: []validatorFunc{
			checkDeprecatedPublicUsage,
			createComponentValidator(),
			createExternalDNSAliasValidator(),
			createDNSAliasValidator(),
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

func createDNSAliasAvailableValidator(kubeClient client.Client, dnsAliasConfig *dnsalias.DNSConfig) validatorFunc {
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

func checkDeprecatedPublicUsage(_ context.Context, ra *radixv1.RadixApplication) (string, error) {
	for _, component := range ra.Spec.Components {
		//nolint:staticcheck
		if component.Public {
			return fmt.Sprintf("component %s is using deprecated public field. use publicPort and ports.name instead", component.Name), nil
		}
	}
	return "", nil
}

func createComponentValidator() validatorFunc {
	return func(ctx context.Context, app *radixv1.RadixApplication) (string, error) {
		var wrns []string
		var errs []error
		for _, component := range app.Spec.Components {

			if component.Image != "" && (component.SourceFolder != "" || component.DockerfileName != "") {
				wrns = append(wrns, fmt.Sprintf("component %s: component image will take precedens. src and dockerfile will be ignored.", component.Name))
			}

			if err := validatePublicPort(component); err != nil {
				errs = append(errs, err)
			}

			// Common resource requirements
			if err := validateResourceRequirements(component.Resources); err != nil {
				errs = append(errs, err)
			}

			if err := validateMonitoring(&component); err != nil {
				errs = append(errs, err)
			}

			errs = append(errs, validateAuthentication(&component, app.Spec.Environments)...)

			if err := validateIdentity(component.Identity); err != nil {
				errs = append(errs, err)
			}

			if err := validateRuntime(component.Runtime); err != nil {
				errs = append(errs, err)
			}

			if err := validateNetwork(component.Network); err != nil {
				errs = append(errs, fmt.Errorf("invalid network configuration: %w", err))
			}

			if err := validateHealthChecks(component.HealthChecks); err != nil {
				errs = append(errs, fmt.Errorf("invalid health check configuration: %w", err))
			}

			for _, environment := range component.EnvironmentConfig {
				if err := validateComponentEnvironment(app, component, environment); err != nil {
					errs = append(errs, fmt.Errorf("invalid configuration for environment %s: %w", environment.Environment, err))
				}
			}

			if err := validateComponent(app, component); err != nil {
				errs = append(errs, fmt.Errorf("invalid configuration for component %s: %w", component.Name, err))
			}
		}

		return "", errors.Join(errs...)
	}
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

func validateResourceRequirements(resourceRequirements radixv1.ResourceRequirements) error {
	var errs []error
	limitQuantities := make(map[string]resource.Quantity)
	for name, value := range resourceRequirements.Limits {
		if len(value) > 0 {
			q, err := resource.ParseQuantity(value)
			if err != nil {
				errs = append(errs, fmt.Errorf("resource %s has invalid limit quantity %s: %w", name, value, err))
			} else {
				limitQuantities[name] = q
			}
		}
	}
	for name, value := range resourceRequirements.Requests {
		q, err := resource.ParseQuantity(value)
		if err != nil {
			errs = append(errs, fmt.Errorf("resource %s has invalid requested quantity %s: %w", name, value, err))
		}
		if limit, limitExist := limitQuantities[name]; limitExist && q.Cmp(limit) == 1 {
			errs = append(errs, fmt.Errorf("resource %s (req: %s, limit: %s): %w", name, q.String(), limit.String(), ErrRequestedResourceExceedsLimit))
		}
	}

	return errors.Join(errs...)
}

func validatePublicPort(component radixv1.RadixComponent) error {
	if component.PublicPort == "" {
		return nil
	}

	for _, port := range component.Ports {
		if port.Name == component.PublicPort {
			return nil // we found a match
		}
	}

	return fmt.Errorf("component %s: %w", component.Name, ErrNoPublicPortMarkedForComponent)
}

func validateMonitoring(component radixv1.RadixCommonComponent) error {
	if component.GetMonitoring() == nil || *component.GetMonitoring() == false {
		return nil // monitoring disabled
	}

	if len(component.GetPorts()) == 0 {
		return fmt.Errorf("component %s: %w", component.GetName(), ErrMonitoringNoPortsDefined)
	}

	monitoringConfig := component.GetMonitoringConfig()
	if monitoringConfig.PortName == "" {
		return nil // first port will be used
	}

	for _, p := range component.GetPorts() {
		if monitoringConfig.PortName == p.Name {
			return nil // we found a match
		}
	}

	return fmt.Errorf("component %s: %w", component.GetName(), ErrMonitoringNamedPortNotFound)
}
