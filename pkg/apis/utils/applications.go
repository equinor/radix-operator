package utils

import (
	"fmt"
	"os"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

// GetAppAdminRbacSubjects Get Role bindings for application admins
func GetAppAdminRbacSubjects(rr *v1.RadixRegistration) ([]rbacv1.Subject, error) {
	adGroups, err := getAdGroupsWithDefault(rr)
	if err != nil {
		return nil, err
	}

	return kube.GetRoleBindingSubjects(adGroups, rr.Spec.AdUsers), nil
}

// GetAppReaderRbacSubjects Get Role bindings for application readers
func GetAppReaderRbacSubjects(rr *v1.RadixRegistration) []rbacv1.Subject {
	return kube.GetRoleBindingSubjects(rr.Spec.ReaderAdGroups, rr.Spec.ReaderAdUsers)
}

func getAdGroupsWithDefault(registration *v1.RadixRegistration) ([]string, error) {
	if len(registration.Spec.AdGroups) > 0 {
		return registration.Spec.AdGroups, nil
	}

	defaultGroup := os.Getenv(defaults.OperatorDefaultUserGroupEnvironmentVariable)
	if defaultGroup != "" {
		return []string{defaultGroup}, nil
	}

	err := fmt.Errorf("cannot obtain ad-group as %s has not been set for the operator", defaults.OperatorDefaultUserGroupEnvironmentVariable)
	return []string{}, err
}
