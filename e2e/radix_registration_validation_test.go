package e2e

import (
	"crypto/rand"
	"testing"
	"time"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestRadixRegistrationCloneURLValidation tests CloneURL validation rules
func TestRadixRegistrationCloneURLValidation(t *testing.T) {
	c := getClient(t)
	testCases := []struct {
		name        string
		cloneURL    string
		shouldError bool
	}{
		{
			name:        "valid GitHub SSH URL",
			cloneURL:    "git@github.com:equinor/radix-test.git",
			shouldError: false,
		},
		{
			name:        "valid GitHub SSH URL with hyphen",
			cloneURL:    "git@github.com:equinor/radix-test-app.git",
			shouldError: false,
		},
		{
			name:        "valid GitHub SSH URL with underscore",
			cloneURL:    "git@github.com:equinor/radix_test_app.git",
			shouldError: false,
		},
		{
			name:        "invalid - HTTPS URL",
			cloneURL:    "https://github.com/equinor/radix-test.git",
			shouldError: true,
		},
		{
			name:        "invalid - missing .git suffix",
			cloneURL:    "git@github.com:equinor/radix-test",
			shouldError: true,
		},
		{
			name:        "invalid - empty",
			cloneURL:    "",
			shouldError: true,
		},
		{
			name:        "invalid - not GitHub",
			cloneURL:    "git@gitlab.com:equinor/radix-test.git",
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rr := &v1.RadixRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cloneurl-validation",
				},
				Spec: v1.RadixRegistrationSpec{
					CloneURL:          tc.cloneURL,
					AdGroups:          []string{"test-group"},
					ConfigBranch:      "main",
					ConfigurationItem: "test-item",
				},
			}

			err := c.Create(t.Context(), rr, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for CloneURL: %s", tc.cloneURL)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid CloneURL: %s", tc.cloneURL)
			}
		})
	}
}

// TestRadixRegistrationConfigBranchValidation tests ConfigBranch validation rules
func TestRadixRegistrationConfigBranchValidation(t *testing.T) {
	c := getClient(t)
	testCases := []struct {
		name         string
		configBranch string
		shouldError  bool
	}{
		{
			name:         "valid - main",
			configBranch: "main",
			shouldError:  false,
		},
		{
			name:         "valid - master",
			configBranch: "master",
			shouldError:  false,
		},
		{
			name:         "valid - feature/test",
			configBranch: "feature/test",
			shouldError:  false,
		},
		{
			name:         "valid - release-1.0",
			configBranch: "release-1.0",
			shouldError:  false,
		},
		{
			name:         "invalid - starts with @",
			configBranch: "@",
			shouldError:  true,
		},
		{
			name:         "invalid - empty string",
			configBranch: "",
			shouldError:  true,
		},
		{
			name:         "invalid - starts with /",
			configBranch: "/main",
			shouldError:  true,
		},
		{
			name:         "invalid - ends with .lock",
			configBranch: "branch.lock",
			shouldError:  true,
		},
		{
			name:         "invalid - ends with .",
			configBranch: "branch.",
			shouldError:  true,
		},
		{
			name:         "invalid - contains /.",
			configBranch: "feature/.hidden",
			shouldError:  true,
		},
		{
			name:         "invalid - contains .lock/",
			configBranch: "feature.lock/test",
			shouldError:  true,
		},
		{
			name:         "invalid - contains ..",
			configBranch: "feature/../main",
			shouldError:  true,
		},
		{
			name:         "invalid - contains @{",
			configBranch: "feature@{0}",
			shouldError:  true,
		},
		{
			name:         "invalid - contains backslash",
			configBranch: "feature\\test",
			shouldError:  true,
		},
		{
			name:         "invalid - contains //",
			configBranch: "feature//test",
			shouldError:  true,
		},
		{
			name:         "invalid - contains ?",
			configBranch: "feature?test",
			shouldError:  true,
		},
		{
			name:         "invalid - contains *",
			configBranch: "feature*",
			shouldError:  true,
		},
		{
			name:         "invalid - contains [",
			configBranch: "feature[test]",
			shouldError:  true,
		},
		{
			name:         "invalid - contains space",
			configBranch: "feature test",
			shouldError:  true,
		},
		{
			name:         "invalid - contains ~",
			configBranch: "feature~test",
			shouldError:  true,
		},
		{
			name:         "invalid - contains ^",
			configBranch: "feature^test",
			shouldError:  true,
		},
		{
			name:         "invalid - contains :",
			configBranch: "feature:test",
			shouldError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rr := &v1.RadixRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-configbranch-validation",
				},
				Spec: v1.RadixRegistrationSpec{
					CloneURL:          "git@github.com:equinor/test-app.git",
					AdGroups:          []string{"test-group"},
					ConfigBranch:      tc.configBranch,
					ConfigurationItem: "test-item",
				},
			}

			err := c.Create(t.Context(), rr, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for ConfigBranch: %s", tc.configBranch)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid ConfigBranch: %s", tc.configBranch)
			}
		})
	}
}

// TestRadixRegistrationRadixConfigFullNameValidation tests RadixConfigFullName validation rules
func TestRadixRegistrationRadixConfigFullNameValidation(t *testing.T) {
	c := getClient(t)
	testCases := []struct {
		name                string
		radixConfigFullName string
		shouldError         bool
	}{
		{
			name:                "valid - radixconfig.yaml",
			radixConfigFullName: "radixconfig.yaml",
			shouldError:         false,
		},
		{
			name:                "valid - radixconfig.yml",
			radixConfigFullName: "radixconfig.yml",
			shouldError:         false,
		},
		{
			name:                "valid - nested path",
			radixConfigFullName: ".radix/radixconfig.yaml",
			shouldError:         false,
		},
		{
			name:                "valid - deep nested path",
			radixConfigFullName: "config/radix/app.yaml",
			shouldError:         false,
		},
		{
			name:                "valid - with underscores and hyphens",
			radixConfigFullName: "config/radix_app-config.yaml",
			shouldError:         false,
		},
		{
			name:                "valid - absolute path",
			radixConfigFullName: "/radixconfig.yaml",
			shouldError:         false,
		},
		{
			name:                "invalid - no extension",
			radixConfigFullName: "radixconfig",
			shouldError:         true,
		},
		{
			name:                "invalid - wrong extension",
			radixConfigFullName: "radixconfig.json",
			shouldError:         true,
		},
		{
			name:                "invalid - contains spaces",
			radixConfigFullName: "radix config.yaml",
			shouldError:         true,
		},
		{
			name:                "empty is valid (optional field)",
			radixConfigFullName: "",
			shouldError:         false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rr := &v1.RadixRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-radixconfig-validation",
				},
				Spec: v1.RadixRegistrationSpec{
					CloneURL:            "git@github.com:equinor/test-app.git",
					AdGroups:            []string{"test-group"},
					ConfigBranch:        "main",
					ConfigurationItem:   "test-item",
					RadixConfigFullName: tc.radixConfigFullName,
				},
			}

			err := c.Create(t.Context(), rr, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for RadixConfigFullName: %s", tc.radixConfigFullName)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid RadixConfigFullName: %s", tc.radixConfigFullName)
			}
		})
	}
}

// TestRadixRegistrationConfigurationItemValidation tests ConfigurationItem validation rules
func TestRadixRegistrationConfigurationItemValidation(t *testing.T) {
	c := getClient(t)
	testCases := []struct {
		name              string
		configurationItem string
		shouldError       bool
	}{
		{
			name:              "valid - short string",
			configurationItem: "12345",
			shouldError:       false,
		},
		{
			name:              "valid - max length (100 chars)",
			configurationItem: "1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890",
			shouldError:       false,
		},
		{
			name:              "invalid - exceeds max length (101 chars)",
			configurationItem: "12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901",
			shouldError:       true,
		},
		{
			name:              "valid - empty (if webhook allows it)",
			configurationItem: "",
			shouldError:       true, // Based on webhook validation requiring it
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rr := &v1.RadixRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-configitem-validation",
				},
				Spec: v1.RadixRegistrationSpec{
					CloneURL:          "git@github.com:equinor/test-app.git",
					AdGroups:          []string{"test-group"},
					ConfigBranch:      "main",
					ConfigurationItem: tc.configurationItem,
				},
			}

			err := c.Create(t.Context(), rr, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for ConfigurationItem with length: %d", len(tc.configurationItem))
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid ConfigurationItem with length: %d", len(tc.configurationItem))
			}
		})
	}
}

// TestRadixRegistrationAdGroupsValidation tests AdGroups validation rules
func TestRadixRegistrationAdGroupsValidation(t *testing.T) {
	c := getClient(t)
	testCases := []struct {
		name        string
		adGroups    []string
		shouldError bool
	}{
		{
			name:        "valid - single group",
			adGroups:    []string{"group1"},
			shouldError: false,
		},
		{
			name:        "valid - multiple groups",
			adGroups:    []string{"group1", "group2", "group3"},
			shouldError: false,
		},
		{
			name:        "invalid - empty array",
			adGroups:    []string{},
			shouldError: true, // Based on webhook validation
		},
		{
			name:        "invalid - nil",
			adGroups:    nil,
			shouldError: true, // Based on webhook validation
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rr := &v1.RadixRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-adgroups-validation",
				},
				Spec: v1.RadixRegistrationSpec{
					CloneURL:          "git@github.com:equinor/test-app.git",
					AdGroups:          tc.adGroups,
					ConfigBranch:      "main",
					ConfigurationItem: "test-item",
				},
			}

			err := c.Create(t.Context(), rr, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for AdGroups: %v", tc.adGroups)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid AdGroups: %v", tc.adGroups)
			}
		})
	}
}

// TestRadixRegistrationImmutableAppID tests that AppID is immutable
func TestRadixRegistrationImmutableAppID(t *testing.T) {
	c := getClient(t)
	// Create a RadixRegistration with AppID
	appID := v1.ULID{ULID: ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)}

	rr := &v1.RadixRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-immutable-appid",
		},
		Spec: v1.RadixRegistrationSpec{
			AppID:             appID,
			CloneURL:          "git@github.com:equinor/test-app.git",
			AdGroups:          []string{"test-group"},
			ConfigBranch:      "main",
			ConfigurationItem: "test-item",
		},
	}

	// Create the RadixRegistration
	err := c.Create(t.Context(), rr)
	require.NoError(t, err, "Should be able to create RadixRegistration")

	// Try to update AppID
	newAppID := v1.ULID{ULID: ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)}

	rr.Spec.AppID = newAppID
	err = c.Update(t.Context(), rr)

	// Should fail because AppID is immutable
	assert.Error(t, err, "Should not allow changing AppID")
	if err != nil {
		t.Logf("Got expected error when trying to change AppID: %v", err)
	}
	err = c.Delete(t.Context(), rr)
	require.NoError(t, err)
}

// TestRadixRegistrationImmutableCreator tests that Creator is immutable
func TestRadixRegistrationImmutableCreator(t *testing.T) {
	c := getClient(t)

	// Create a RadixRegistration with Creator
	rr := &v1.RadixRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-immutable-creator",
		},
		Spec: v1.RadixRegistrationSpec{
			CloneURL:          "git@github.com:equinor/test-app2.git",
			AdGroups:          []string{"test-group"},
			ConfigBranch:      "main",
			ConfigurationItem: "test-item",
			Creator:           "original@creator.com",
		},
	}

	// Create the RadixRegistration
	err := c.Create(t.Context(), rr)
	require.NoError(t, err, "Should be able to create RadixRegistration")

	// Try to update Creator
	rr.Spec.Creator = "new@creator.com"
	err = c.Update(t.Context(), rr)

	// Should fail because Creator is immutable
	assert.Error(t, err, "Should not allow changing Creator")
	if err != nil {
		t.Logf("Got expected error when trying to change Creator: %v", err)
	}
	err = c.Delete(t.Context(), rr)
	require.NoError(t, err)
}

// TestRadixRegistrationMutableFields tests that mutable fields can be updated
func TestRadixRegistrationMutableFields(t *testing.T) {
	c := getClient(t)

	// Create a RadixRegistration
	rr := &v1.RadixRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mutable-fields",
		},
		Spec: v1.RadixRegistrationSpec{
			CloneURL:          "git@github.com:equinor/test-app3.git",
			AdGroups:          []string{"test-group"},
			ConfigBranch:      "main",
			ConfigurationItem: "test-item",
			Owner:             "original@owner.com",
		},
	}

	// Create the RadixRegistration
	err := c.Create(t.Context(), rr)
	require.NoError(t, err, "Should be able to create RadixRegistration")

	// Update mutable fields
	rr.Spec.Owner = "new@owner.com"
	rr.Spec.AdGroups = []string{"test-group", "new-group"}
	rr.Spec.ConfigBranch = "develop"

	err = c.Update(t.Context(), rr)

	// Should succeed because these fields are mutable
	assert.NoError(t, err, "Should allow updating mutable fields")
	if err == nil {
		t.Log("Successfully updated mutable fields")
	}

	err = c.Delete(t.Context(), rr)
	require.NoError(t, err)
}

// TestRadixRegistrationUniqueAppID tests that AppID must be unique across RadixRegistrations
func TestRadixRegistrationUniqueAppID(t *testing.T) {
	c := getClient(t)
	// Create first RadixRegistration with AppID
	appID := v1.ULID{ULID: ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)}

	rr1 := &v1.RadixRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-unique-appid-1",
		},
		Spec: v1.RadixRegistrationSpec{
			AppID:             appID,
			CloneURL:          "git@github.com:equinor/test-app4.git",
			AdGroups:          []string{"test-group"},
			ConfigBranch:      "main",
			ConfigurationItem: "test-item",
		},
	}

	err := c.Create(t.Context(), rr1)
	require.NoError(t, err, "Should be able to create first RadixRegistration")

	// Try to create second RadixRegistration with same AppID
	rr2 := &v1.RadixRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-unique-appid-2",
		},
		Spec: v1.RadixRegistrationSpec{
			AppID:             appID, // Same AppID
			CloneURL:          "git@github.com:equinor/test-app5.git",
			AdGroups:          []string{"test-group"},
			ConfigBranch:      "main",
			ConfigurationItem: "test-item",
		},
	}

	err = c.Create(t.Context(), rr2)

	// Should fail because AppID must be unique
	assert.Error(t, err, "Should not allow duplicate AppID")
	if err != nil {
		t.Logf("Got expected error when trying to create duplicate AppID: %v", err)
	}
	err = c.Delete(t.Context(), rr1)
	require.NoError(t, err)
}

// TestRadixRegistrationEmptyAppIDNotUnique tests that empty AppID is allowed and not checked for uniqueness
func TestRadixRegistrationEmptyAppIDNotUnique(t *testing.T) {
	c := getClient(t)

	// Create two RadixRegistrations without AppID
	rr1 := &v1.RadixRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-no-appid-1",
		},
		Spec: v1.RadixRegistrationSpec{
			CloneURL:          "git@github.com:equinor/test-app6.git",
			AdGroups:          []string{"test-group"},
			ConfigBranch:      "main",
			ConfigurationItem: "test-item",
		},
	}

	err := c.Create(t.Context(), rr1)
	require.NoError(t, err, "Should be able to create RadixRegistration without AppID")

	rr2 := &v1.RadixRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-no-appid-2",
		},
		Spec: v1.RadixRegistrationSpec{
			CloneURL:          "git@github.com:equinor/test-app7.git",
			AdGroups:          []string{"test-group"},
			ConfigBranch:      "main",
			ConfigurationItem: "test-item",
		},
	}

	err = c.Create(t.Context(), rr2)

	// Should succeed because empty AppID is not checked for uniqueness
	require.NoError(t, err, "Should allow multiple RadixRegistrations without AppID")
	err = c.Delete(t.Context(), rr2)
	require.NoError(t, err)
	err = c.Delete(t.Context(), rr1)
	require.NoError(t, err)
	t.Log("Successfully created multiple RadixRegistrations without AppID")
}
