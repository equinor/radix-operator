package utils

import (
	"fmt"
	"os"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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

// GetAppAdminRbacSubjects Get Role bindings for application admin
func GetAppAdminRbacSubjects(rr *v1.RadixRegistration) ([]rbacv1.Subject, error) {
	adGroups, err := GetAdGroups(rr)
	if err != nil {
		return nil, err
	}
	subjects := kube.GetRoleBindingGroups(adGroups)
	return subjects, nil
}
