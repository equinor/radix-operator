package validators

import (
	"fmt"

	radixv1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func IsValidRadixApplication(client radixclient.Interface, app *radixv1.RadixApplication) (bool, []error) {
	errs := []error{}
	err := validateAppName(app.Name)
	if err != nil {
		errs = append(errs, err)
	}

	err = validateExistEnvForComponentVariables(app)
	if err != nil {
		errs = append(errs, err)
	}

	err = validateDoesRRExist(client, app.Name)
	if err != nil {
		errs = append(errs, err)
	}

	return len(errs) <= 0, errs
}

func validateDoesRRExist(client radixclient.Interface, appName string) error {
	rr, err := client.RadixV1().RadixRegistrations("default").Get(appName, metav1.GetOptions{})
	if rr == nil {
		return fmt.Errorf("No app registered with that name %s", appName)
	}
	if err != nil {
		return fmt.Errorf("Could not get app registration obj %s", appName)
	}
	return nil
}

func validateExistEnvForComponentVariables(app *radixv1.RadixApplication) error {
	for _, component := range app.Spec.Components {
		for _, variable := range component.EnvironmentVariables {
			if !doesEnvExist(app, variable.Environment) {
				return fmt.Errorf("Env %s refered to by component variable %s is not defined", variable.Environment, component.Name)
			}
		}
	}

	return nil
}

func doesEnvExist(app *radixv1.RadixApplication, name string) bool {
	for _, env := range app.Spec.Environments {
		if env.Name == name {
			return true
		}
	}
	return false
}
