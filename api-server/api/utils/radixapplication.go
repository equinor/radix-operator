package utils

import (
	"strings"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// GetComponentEnvironmentConfig Gets environment config of component
func GetComponentEnvironmentConfig(ra *radixv1.RadixApplication, envName, componentName string) radixv1.RadixCommonEnvironmentConfig {
	component := getRadixCommonComponentByName(ra, componentName)
	if component == nil {
		return nil
	}
	return getEnvironmentConfigByName(component, envName)
}

func getEnvironmentConfigByName(component radixv1.RadixCommonComponent, envName string) radixv1.RadixCommonEnvironmentConfig {
	for _, environment := range component.GetEnvironmentConfig() {
		if strings.EqualFold(environment.GetEnvironment(), envName) {
			return environment
		}
	}
	return nil
}

func getRadixCommonComponentByName(ra *radixv1.RadixApplication, name string) radixv1.RadixCommonComponent {
	for _, component := range ra.Spec.Components {
		if strings.EqualFold(component.Name, name) {
			return &component
		}
	}
	for _, jobComponent := range ra.Spec.Jobs {
		if strings.EqualFold(jobComponent.Name, name) {
			return &jobComponent
		}
	}
	return nil
}
