package radixregistration_test

import (
	"testing"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/webhook/validation/radixregistration"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Test unique appId validation

func Test_valid_rr_returns_true(t *testing.T) {
	client := createClient()
	validRR := createValidRR()

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
			name:                     "optional ConfigurationItem is empty",
			updateRR:                 func(rr *radixv1.RadixRegistration) { rr.Spec.ConfigurationItem = "" },
			requireAdGroups:          false,
			requireConfigurationItem: false,
			expectedWarnings:         nil,
			expectedError:            nil,
		},
		{
			name:                     "required ConfigurationItem is empty",
			updateRR:                 func(rr *radixv1.RadixRegistration) { rr.Spec.ConfigurationItem = "" },
			requireAdGroups:          false,
			requireConfigurationItem: true,
			expectedWarnings:         nil,
			expectedError:            radixregistration.ErrConfigurationItemIsRequired,
		},
		{
			name:                     "optional ad groups is empty",
			updateRR:                 func(rr *radixv1.RadixRegistration) { rr.Spec.AdGroups = nil },
			requireAdGroups:          false,
			requireConfigurationItem: false,
			expectedWarnings:         admission.Warnings{radixregistration.WarningAdGroupsShouldHaveAtleastOneItem},
			expectedError:            nil,
		},
		{
			name:                     "required ad groups is empty",
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
			validRR := createValidRR()
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
	validRR := createValidRR()
	validRR2 := createValidRR()
	client := createClient(validRR)
	validRR2.Name = "duplicate-app-id"
	validator := radixregistration.CreateOnlineValidator(client, false, false)

	_, err := validator.Validate(t.Context(), validRR2)

	assert.ErrorIs(t, err, radixregistration.ErrAppIdMustBeUnique)
}

func createValidRR() *radixv1.RadixRegistration {
	validRR, _ := utils.GetRadixRegistrationFromFile("testdata/radixregistration.yaml")
	return validRR
}

func createClient(initObjs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(radixv1.AddToScheme(scheme))

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).Build()
}
