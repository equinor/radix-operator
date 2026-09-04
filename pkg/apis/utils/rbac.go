package utils

import (
	"github.com/equinor/radix-operator/pkg/apis/config2"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

// GetAppAdminRbacSubjects Get Role bindings for application admins
func GetAppAdminRbacSubjects(cfg config2.Config, rr *radixv1.RadixRegistration) []rbacv1.Subject {
	adGroups := getAdAdminGroupsWithDefault(cfg, rr)

	return getRoleBindingSubjects(adGroups, rr.Spec.AdUsers)
}

// GetAppReaderRbacSubjects Get Role bindings for application readers
func GetAppReaderRbacSubjects(rr *radixv1.RadixRegistration) []rbacv1.Subject {
	return getRoleBindingSubjects(rr.Spec.ReaderAdGroups, rr.Spec.ReaderAdUsers)
}

func getAdAdminGroupsWithDefault(cfg config2.Config, registration *radixv1.RadixRegistration) []string {
	if len(registration.Spec.AdGroups) > 0 {
		return registration.Spec.AdGroups
	}

	return cfg.Operator.DefaultAppAdminGroups
}

func getRoleBindingSubjects(groups, users []string) []rbacv1.Subject {
	var subjects []rbacv1.Subject
	for _, group := range groups {
		subjects = append(subjects, rbacv1.Subject{
			Kind:     rbacv1.GroupKind,
			Name:     group,
			APIGroup: rbacv1.GroupName,
		})
	}
	for _, user := range users {
		subjects = append(subjects, rbacv1.Subject{
			Kind:     rbacv1.UserKind,
			Name:     user,
			APIGroup: rbacv1.GroupName,
		})
	}
	return subjects
}
