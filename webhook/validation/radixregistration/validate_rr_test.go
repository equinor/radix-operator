package radixregistration_test

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/webhook/validation/radixregistration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Test unique appId validation

func Test_valid_rr_returns_true(t *testing.T) {
	client := test.CreateClient()
	validRR := test.Load[*radixv1.RadixRegistration]("testdata/radixregistration.yaml")

	validator := radixregistration.CreateOnlineValidator(client, true, true)
	warnings, err := validator.Validate(t.Context(), validRR)

	assert.Empty(t, warnings)
	assert.Nil(t, err)
}

func TestCanRadixApplicationBeUpdated(t *testing.T) {
	var testScenarios = []struct {
		name                     string
		updateRR                 func(rr *radixv1.RadixRegistration)
		requireAdGroups          bool
		requireConfigurationItem bool
		expectedWarnings         admission.Warnings
		expectedError            error
	}{
		{
			name:                     "optional ConfigurationItem is empty is allowed",
			updateRR:                 func(rr *radixv1.RadixRegistration) { rr.Spec.ConfigurationItem = "" },
			requireAdGroups:          false,
			requireConfigurationItem: false,
			expectedWarnings:         nil,
			expectedError:            nil,
		},
		{
			name:                     "required ConfigurationItem is empty fails",
			updateRR:                 func(rr *radixv1.RadixRegistration) { rr.Spec.ConfigurationItem = "" },
			requireAdGroups:          false,
			requireConfigurationItem: true,
			expectedWarnings:         nil,
			expectedError:            radixregistration.ErrConfigurationItemIsRequired,
		},
		{
			name:                     "optional ad groups is empty returns warning",
			updateRR:                 func(rr *radixv1.RadixRegistration) { rr.Spec.AdGroups = nil },
			requireAdGroups:          false,
			requireConfigurationItem: false,
			expectedWarnings:         admission.Warnings{radixregistration.WarningAdGroupsShouldHaveAtleastOneItem},
			expectedError:            nil,
		},
		{
			name:                     "required ad groups is empty fails",
			updateRR:                 func(rr *radixv1.RadixRegistration) { rr.Spec.AdGroups = nil },
			requireAdGroups:          true,
			requireConfigurationItem: false,
			expectedWarnings:         nil,
			expectedError:            radixregistration.ErrAdGroupIsRequired,
		},
	}

	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validator := radixregistration.CreateOfflineValidator(testcase.requireAdGroups, testcase.requireConfigurationItem)
			validRR := test.Load[*radixv1.RadixRegistration]("testdata/radixregistration.yaml")
			testcase.updateRR(validRR)
			warnings, err := validator.Validate(t.Context(), validRR)

			assert.Equal(t, testcase.expectedWarnings, warnings)
			if testcase.expectedError == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, testcase.expectedError)
			}
		})
	}
}

func TestDuplicateAppIDMustFail(t *testing.T) {
	validRR2 := test.Load[*radixv1.RadixRegistration]("testdata/radixregistration.yaml")
	client := test.CreateClient("testdata/radixregistration.yaml")
	validRR2.Name = "duplicate-app-id"
	validator := radixregistration.CreateOnlineValidator(client, false, false)

	_, err := validator.Validate(t.Context(), validRR2)

	assert.ErrorIs(t, err, radixregistration.ErrAppIdMustBeUnique)
}

func TestNamespaceUsableValidator(t *testing.T) {
	var testScenarios = []struct {
		name          string
		namespaces    []client.Object
		expectedError error
	}{
		{
			name:          "no conflicting namespace returns no error",
			namespaces:    nil,
			expectedError: nil,
		},
		{
			name: "namespace with same name owned by same app returns no error",
			namespaces: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "testapp-app",
						Labels: map[string]string{kube.RadixAppLabel: "testapp"},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "namespace with same name owned by different app returns error",
			namespaces: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "testapp-app",
						Labels: map[string]string{kube.RadixAppLabel: "otherapp"},
					},
				},
			},
			expectedError: radixregistration.ErrEnvironmentNameIsNotAvailable,
		},
		{
			name: "namespace with same name without radix-app label returns error",
			namespaces: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "testapp-app",
					},
				},
			},
			expectedError: radixregistration.ErrEnvironmentNameIsNotAvailable,
		},
		{
			name: "namespace with different name returns no error",
			namespaces: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "otherapp-app",
						Labels: map[string]string{kube.RadixAppLabel: "otherapp"},
					},
				},
			},
			expectedError: nil,
		},
	}

	for _, ts := range testScenarios {
		t.Run(ts.name, func(t *testing.T) {
			client := test.CreateClient()
			for _, ns := range ts.namespaces {
				require.NoError(t, client.Create(t.Context(), ns))
			}
			validRR := test.Load[*radixv1.RadixRegistration]("testdata/radixregistration.yaml")
			validator := radixregistration.CreateOnlineValidator(client, false, false)

			warnings, err := validator.Validate(t.Context(), validRR)

			if ts.expectedError == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, ts.expectedError)
			}
			assert.Empty(t, warnings)
		})
	}
}
