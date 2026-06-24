package radixapplication

import (
	"context"
	"fmt"
	"regexp"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
)

var (
	validNameRegex = regexp.MustCompile(`^(([A-Za-z0-9][-._A-Za-z0-9]*)?[A-Za-z0-9])?$`)
)

func variableValidator(ctx context.Context, ra *radixv1.RadixApplication) ([]string, []error) {
	var errs []error

	for _, component := range ra.Spec.Components {
		err := validateRadixComponentVariables(&component)
		if len(err) > 0 {
			errs = append(errs, err...)
		}
	}
	for _, job := range ra.Spec.Jobs {
		err := validateRadixComponentVariables(&job)
		if len(err) > 0 {
			errs = append(errs, err...)
		}
	}
	return nil, errs
}

func validateRadixComponentVariables(component radixv1.RadixCommonComponent) []error {
	var errs []error
	if err := validateVariableNames(component.GetVariables()); err != nil {
		errs = append(errs, err...)
	}

	for _, envConfig := range component.GetEnvironmentConfig() {
		if err := validateVariableNames(envConfig.GetVariables()); err != nil {
			errs = append(errs, err...)
		}
	}
	return errs
}

func validateVariableNames(variables radixv1.EnvVarsMap) []error {
	var errs []error

	for envVarName := range variables {
		if len(envVarName) > 253 {
			errs = append(errs, fmt.Errorf("variable %s: %w", envVarName, ErrVariableNameCannotExceedMaxLength))
		}

		if !validNameRegex.MatchString(envVarName) {
			errs = append(errs, fmt.Errorf("variable %s: %w", envVarName, ErrVariableNameCannotContainIllegalCharacters))
		}
		if utils.IsRadixEnvVar(envVarName) {
			errs = append(errs, fmt.Errorf("variable %s: %w", envVarName, ErrVariableNameCannotStartWithReservedPrefix))
		}
	}
	return errs
}
