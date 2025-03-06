package applicationconfig

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"strings"
)

// GetEnvironmentFromRadixApplication Gets environment config with name envName from supplied RadixApplication
func GetEnvironmentFromRadixApplication(ra *v1.RadixApplication, envName string) *v1.Environment {
	if ra == nil {
		return nil
	}

	for _, environment := range ra.Spec.Environments {
		if strings.EqualFold(environment.Name, envName) {
			return &environment
		}
	}
	return nil
}
