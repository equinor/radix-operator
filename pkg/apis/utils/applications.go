package utils

import (
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"os"
)

// GetAdGroups Gets ad-groups from registration. If missing, gives default for cluster
func GetAdGroups(registration *v1.RadixRegistration) ([]string, error) {
	if registration.Spec.AdGroups == nil || len(registration.Spec.AdGroups) <= 0 {
		defaultGroup := os.Getenv(defaults.OperatorDefaultUserGroupEnvironmentVariable)
		if defaultGroup == "" {
			err := fmt.Errorf("cannot obtain ad-group as %s has not been set for the operator", defaults.OperatorDefaultUserGroupEnvironmentVariable)
			return []string{}, err
		}

		return []string{defaultGroup}, nil
	}

	return registration.Spec.AdGroups, nil
}
