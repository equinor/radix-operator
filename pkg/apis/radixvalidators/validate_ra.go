package radixvalidators

import (
	"fmt"
	"regexp"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
)

func CanRadixApplicationBeInserted(client radixclient.Interface, app *radixv1.RadixApplication) (bool, error) {
	isValid, errs := CanRadixApplicationBeInsertedErrors(client, app)
	if isValid {
		return true, nil
	}

	return false, ConcatErrors(errs)
}

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

	dnsErrors := validateDnsAppAlias(app)
	if len(dnsErrors) > 0 {
		errs = append(errs, dnsErrors...)
	}

	if len(errs) <= 0 {
		return true, nil
	}
	return false, errs
}

func validateDnsAppAlias(app *radixv1.RadixApplication) []error {
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

func validateComponents(app *radixv1.RadixApplication) []error {
	errs := []error{}
	for _, component := range app.Spec.Components {
		err := validateRequiredResourceName("component name", component.Name)
		if err != nil {
			errs = append(errs, err)
		}

		err = validateReplica(component.Replicas)
		if err != nil {
			errs = append(errs, err)
		}

		errList := validateResourceRequirements(&component.Resources)
		if errList != nil && len(errList) > 0 {
			errs = append(errs, errList...)
		}

		for _, port := range component.Ports {
			err := validateRequiredResourceName("port name", port.Name)
			if err != nil {
				errs = append(errs, err)
			}
		}

		for _, variable := range component.EnvironmentVariables {
			if !doesEnvExist(app, variable.Environment) {
				err = fmt.Errorf("Env %s refered to by component variable %s is not defined", variable.Environment, component.Name)
				errs = append(errs, err)
			}
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
		regex := "^[0-9]+[MG]i$"
		re := regexp.MustCompile(regex)

		isValid := re.MatchString(value)
		if !isValid {
			return fmt.Errorf("Format of memory resource requirement %s (value %s) is wrong. Must match regex '%s'", name, value, regex)
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
