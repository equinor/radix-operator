package utils

import (
	"os"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

// GetAppAdminRbacSubjects Get Role bindings for application admins
func GetAppAdminRbacSubjects(rr *radixv1.RadixRegistration) []rbacv1.Subject {
	adGroups := getAdAdminGroupsWithDefault(rr)

	return getRoleBindingSubjects(adGroups, rr.Spec.AdUsers)
}

// GetAppReaderRbacSubjects Get Role bindings for application readers
func GetAppReaderRbacSubjects(rr *radixv1.RadixRegistration) []rbacv1.Subject {
	return getRoleBindingSubjects(rr.Spec.ReaderAdGroups, rr.Spec.ReaderAdUsers)
}

func getAdAdminGroupsWithDefault(registration *radixv1.RadixRegistration) []string {
	if len(registration.Spec.AdGroups) > 0 {
		return registration.Spec.AdGroups
	}

	var groups []string
	for _, group := range strings.Split(os.Getenv(defaults.OperatorDefaultAppAdminGroupsEnvironmentVariable), ",") {
		if group := strings.TrimSpace(group); len(group) > 0 {
			groups = append(groups, group)
		}
	}

	return groups
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
