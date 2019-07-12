package radixvalidators

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
)

// CanRadixApplicationBeInserted Checks if application config is valid. Returns a single error, if this is the case
func CanRadixApplicationBeInserted(client radixclient.Interface, app *radixv1.RadixApplication) (bool, error) {
	isValid, errs := CanRadixApplicationBeInsertedErrors(client, app)
	if isValid {
		return true, nil
	}

	return false, ConcatErrors(errs)
}

// CanRadixApplicationBeInsertedErrors Checks if application config is valid. Returns list of errors, if present
func CanRadixApplicationBeInsertedErrors(client radixclient.Interface, app *radixv1.RadixApplication) (bool, []error) {
	errs := []error{}
	err := validateAppName(app.Name)
	if err != nil {
		errs = append(errs, err)
	}

	componentErrs := validateComponents(app)
	if len(componentErrs) > 0 {
		errs = append(errs, componentErrs...)
	}

	err = validateEnvNames(app)
	if err != nil {
		errs = append(errs, err)
	}

	err = validateBranchNames(app)
	if err != nil {
		errs = append(errs, err)
	}

	err = validateDoesRRExist(client, app.Name)
	if err != nil {
		errs = append(errs, err)
	}

	dnsErrors := validateDNSAppAlias(app)
	if len(dnsErrors) > 0 {
		errs = append(errs, dnsErrors...)
	}

	dnsErrors = validateDNSExternalAlias(app)
	if len(dnsErrors) > 0 {
		errs = append(errs, dnsErrors...)
	}

	if len(errs) <= 0 {
		return true, nil
	}
	return false, errs
}

// RAContainsOldPublic Checks to see if the radix config is using the deprecated config for public port
func RAContainsOldPublic(app *radixv1.RadixApplication) bool {
	for _, component := range app.Spec.Components {
		if component.Public {
			return true
		}
	}
	return false
}

func validateDNSAppAlias(app *radixv1.RadixApplication) []error {
	errs := []error{}
	alias := app.Spec.DNSAppAlias
	if alias.Component == "" && alias.Environment == "" {
		return errs
	}

	if !doesEnvExist(app, alias.Environment) {
		errs = append(errs, fmt.Errorf("Env %s refered to by dnsAppAlias is not defined", alias.Environment))
	}
	if !doesComponentExist(app, alias.Component) {
		errs = append(errs, fmt.Errorf("Component %s refered to by dnsAppAlias is not defined", alias.Component))
	}
	return errs
}

func validateDNSExternalAlias(app *radixv1.RadixApplication) []error {
	errs := []error{}

	distinctAlias := make(map[string]bool)

	for _, externalAlias := range app.Spec.DNSExternalAlias {
		if externalAlias.Alias == "" && externalAlias.Component == "" && externalAlias.Environment == "" {
			return errs
		}

		distinctAlias[externalAlias.Alias] = true

		if externalAlias.Alias == "" {
			errs = append(errs, errors.New("External alias cannot be empty"))
		}

		if !doesEnvExist(app, externalAlias.Environment) {
			errs = append(errs, fmt.Errorf("Env %s refered to by dnsExternalAlias is not defined", externalAlias.Environment))
		}
		if !doesComponentExist(app, externalAlias.Component) {
			errs = append(errs, fmt.Errorf("Component %s refered to by dnsExternalAlias is not defined", externalAlias.Component))
		}

		if !doesComponentHaveAPublicPort(app, externalAlias.Component) {
			errs = append(errs, fmt.Errorf("Component %s refered to by dnsExternalAlias is not marked as public", externalAlias.Component))
		}
	}

	if len(distinctAlias) < len(app.Spec.DNSExternalAlias) {
		errs = append(errs, errors.New("Cannot have duplicate aliases for dnsExternalAlias"))
	}

	return errs
}

func validateComponents(app *radixv1.RadixApplication) []error {
	errs := []error{}
	for _, component := range app.Spec.Components {
		err := validateRequiredResourceName("component name", component.Name)
		if err != nil {
			errs = append(errs, err)
		}

		errList := validatePorts(component)
		if errList != nil && len(errList) > 0 {
			errs = append(errs, errList...)
		}

		for _, environment := range component.EnvironmentConfig {
			if !doesEnvExist(app, environment.Environment) {
				err = fmt.Errorf("Env %s refered to by component %s is not defined", environment.Environment, component.Name)
				errs = append(errs, err)
			}

			err = validateReplica(environment.Replicas)
			if err != nil {
				errs = append(errs, err)
			}

			errList = validateResourceRequirements(&environment.Resources)
			if errList != nil && len(errList) > 0 {
				errs = append(errs, errList...)
			}
		}
	}

	return errs
}

func validatePorts(component radixv1.RadixComponent) []error {
	errs := []error{}

	if component.Ports == nil || len(component.Ports) == 0 {
		err := fmt.Errorf("Port specification cannot be empty for %s", component.Name)
		errs = append(errs, err)
	}

	for _, port := range component.Ports {
		err := validateRequiredResourceName("port name", port.Name)
		if err != nil {
			errs = append(errs, err)
		}
	}

	publicPortName := component.PublicPort
	if publicPortName != "" {
		matchingPortName := 0
		for _, port := range component.Ports {
			if strings.EqualFold(port.Name, publicPortName) {
				matchingPortName++
			}
		}
		if matchingPortName < 1 {
			errs = append(errs, fmt.Errorf("%s port name is required for public component %s", publicPortName, component.Name))
		}
		if matchingPortName > 1 {
			errs = append(errs, fmt.Errorf("There are %d ports with name %s for component %s. Only 1 is allowed", matchingPortName, publicPortName, component.Name))
		}
	}

	return errs
}

func validateResourceRequirements(resourceRequirements *radixv1.ResourceRequirements) []error {
	errs := []error{}
	if resourceRequirements == nil {
		return errs
	}

	for name, value := range resourceRequirements.Requests {
		err := validateQuantity(name, value)
		if err != nil {
			errs = append(errs, err)
		}
	}
	for name, value := range resourceRequirements.Limits {
		err := validateQuantity(name, value)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func validateQuantity(name, value string) error {
	if name == "memory" {
		_, err := resource.ParseQuantity(value)
		if err != nil {
			return fmt.Errorf("Format of memory resource requirement %s (value %s) is wrong. Value must be a valid Kubernetes quantity", name, value)
		}
	} else if name == "cpu" {
		regex := "^[0-9]+m$"
		re := regexp.MustCompile(regex)

		isValid := re.MatchString(value)
		if !isValid {
			return fmt.Errorf("Format of cpu resource requirement %s (value %s) is wrong. Must match regex '%s'", name, value, regex)
		}
	} else {
		return fmt.Errorf("Only support resource requirement type 'memory' and 'cpu' (not '%s')", name)
	}

	return nil
}

func validateBranchNames(app *radixv1.RadixApplication) error {
	for _, env := range app.Spec.Environments {
		if env.Build.From == "" {
			continue
		}

		err := validateLabelName("branch from", env.Build.From)
		if err != nil {
			return err
		}
	}
	return nil
}

func validateEnvNames(app *radixv1.RadixApplication) error {
	for _, env := range app.Spec.Environments {
		err := validateRequiredResourceName("env name", env.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func validateLabelName(resourceName, value string) error {
	if len(value) > 253 {
		return fmt.Errorf("%s (%s) max length is 253", resourceName, value)
	}

	re := regexp.MustCompile("^(([A-Za-z0-9][-A-Za-z0-9.]*)?[A-Za-z0-9])?$")

	isValid := re.MatchString(value)
	if isValid {
		return nil
	}
	return fmt.Errorf("%s %s can only consist of alphanumeric characters, '.' and '-'", resourceName, value)
}

func doesComponentExist(app *radixv1.RadixApplication, name string) bool {
	for _, component := range app.Spec.Components {
		if component.Name == name {
			return true
		}
	}
	return false
}

func doesEnvExist(app *radixv1.RadixApplication, name string) bool {
	for _, env := range app.Spec.Environments {
		if env.Name == name {
			return true
		}
	}
	return false
}

func doesComponentHaveAPublicPort(app *radixv1.RadixApplication, name string) bool {
	for _, component := range app.Spec.Components {
		if component.Name == name {
			if component.Public || component.PublicPort != "" {
				return true
			}

			return false
		}
	}
	return false
}
