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

func variableValidator(ctx context.Context, ra *radixv1.RadixApplication) (string, error) {

	for _, component := range ra.Spec.Components {
		err := validateRadixComponentVariables(&component)
		if err != nil {
			return "", err
		}
	}
	for _, job := range ra.Spec.Jobs {
		err := validateRadixComponentVariables(&job)
		if err != nil {
			return "", err
		}
	}
	return "", nil
}

func validateRadixComponentVariables(component radixv1.RadixCommonComponent) error {
	if err := validateVariableNames(component.GetVariables()); err != nil {
		return err
	}

	for _, envConfig := range component.GetEnvironmentConfig() {
		if err := validateVariableNames(envConfig.GetVariables()); err != nil {
			return err
		}
	}
	return nil
}

func validateVariableNames(variables radixv1.EnvVarsMap) error {
	for envVarName := range variables {
		if len(envVarName) > 253 {
			return fmt.Errorf("variable %s: %w", envVarName, ErrVariableNameCannotExceedMaxLength)
		}

		if !validNameRegex.MatchString(envVarName) {
			return fmt.Errorf("variable %s: %w", envVarName, ErrVariableNameCannotContainIllegalCharacters)
		}
		if utils.IsRadixEnvVar(envVarName) {
			return fmt.Errorf("variable %s: %w", envVarName, ErrVariableNameCannotStartWithReservedPrefix)
		}
	}
	return nil
}
