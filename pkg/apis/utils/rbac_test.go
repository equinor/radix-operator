package utils_test

import (
	"slices"
	"testing"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
)

func Test_GetAppAdminRbacSubjects(t *testing.T) {

	tests := map[string]struct {
		groups                []string
		users                 []string
		defaultGroupsEnvValue string
		expectedGroups        []string
		expectedUsers         []string
	}{
		"groups": {
			groups:         []string{"group1", "group2"},
			expectedGroups: []string{"group1", "group2"},
		},
		"users": {
			users:         []string{"user1", "user2"},
			expectedUsers: []string{"user1", "user2"},
		},
		"combine groups and users": {
			groups:         []string{"group1", "group2"},
			users:          []string{"user1", "user2"},
			expectedGroups: []string{"group1", "group2"},
			expectedUsers:  []string{"user1", "user2"},
		},
		"use groups from env when groups not set in RR": {
			defaultGroupsEnvValue: "default1,default2",
			users:                 []string{"user1", "user2"},
			expectedGroups:        []string{"default1", "default2"},
			expectedUsers:         []string{"user1", "user2"},
		},
		"ignore groups from env when groups set in RR": {
			defaultGroupsEnvValue: "default1,default2",
			groups:                []string{"group1", "group2"},
			users:                 []string{"user1", "user2"},
			expectedGroups:        []string{"group1", "group2"},
			expectedUsers:         []string{"user1", "user2"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv(defaults.OperatorDefaultAppAdminGroupsEnvironmentVariable, test.defaultGroupsEnvValue)

			rr := &radixv1.RadixRegistration{
				Spec: radixv1.RadixRegistrationSpec{
					AdGroups: test.groups,
					AdUsers:  test.users,
				},
			}

			expectedSubjects := slices.Concat(
				slice.Map(test.expectedGroups, func(v string) rbacv1.Subject {
					return rbacv1.Subject{Kind: rbacv1.GroupKind, APIGroup: rbacv1.GroupName, Name: v}
				}),
				slice.Map(test.expectedUsers, func(v string) rbacv1.Subject {
					return rbacv1.Subject{Kind: rbacv1.UserKind, APIGroup: rbacv1.GroupName, Name: v}
				}),
			)
			actualSubjects := utils.GetAppAdminRbacSubjects(rr)
			assert.ElementsMatch(t, expectedSubjects, actualSubjects)
		})
	}
}

func Test_GetAppReaderRbacSubjects(t *testing.T) {

	tests := map[string]struct {
		groups                []string
		users                 []string
		defaultGroupsEnvValue string
		expectedGroups        []string
		expectedUsers         []string
	}{
		"groups": {
			groups:         []string{"group1", "group2"},
			expectedGroups: []string{"group1", "group2"},
		},
		"users": {
			users:         []string{"user1", "user2"},
			expectedUsers: []string{"user1", "user2"},
		},
		"combine groups and users": {
			groups:         []string{"group1", "group2"},
			users:          []string{"user1", "user2"},
			expectedGroups: []string{"group1", "group2"},
			expectedUsers:  []string{"user1", "user2"},
		},
		"do not use groups from env when groups not set in RR": {
			defaultGroupsEnvValue: "default1,default2",
			users:                 []string{"user1", "user2"},
			expectedUsers:         []string{"user1", "user2"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv(defaults.OperatorDefaultAppAdminGroupsEnvironmentVariable, test.defaultGroupsEnvValue)

			rr := &radixv1.RadixRegistration{
				Spec: radixv1.RadixRegistrationSpec{
					ReaderAdGroups: test.groups,
					ReaderAdUsers:  test.users,
				},
			}

			expectedSubjects := slices.Concat(
				slice.Map(test.expectedGroups, func(v string) rbacv1.Subject {
					return rbacv1.Subject{Kind: rbacv1.GroupKind, APIGroup: rbacv1.GroupName, Name: v}
				}),
				slice.Map(test.expectedUsers, func(v string) rbacv1.Subject {
					return rbacv1.Subject{Kind: rbacv1.UserKind, APIGroup: rbacv1.GroupName, Name: v}
				}),
			)
			actualSubjects := utils.GetAppReaderRbacSubjects(rr)
			assert.ElementsMatch(t, expectedSubjects, actualSubjects)
		})
	}
}
