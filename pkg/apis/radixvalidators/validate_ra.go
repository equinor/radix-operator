package radixvalidators

import (
	"fmt"
	"regexp"

	radixv1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
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
