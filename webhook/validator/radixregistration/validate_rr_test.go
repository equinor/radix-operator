package radixregistration_test

import (
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/equinor/radix-operator/webhook/validator/radixregistration"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

// Test Required/NotRequired AdGroups and ConfigurationItem
// Test Offline vs Online validation
// Test unique appId validation

func Test_valid_rr_returns_true(t *testing.T) {
	_, client := validRRSetup()
	validRR := createValidRR()

	validator := radixregistration.CreateOnlineValidator(t.Context(), client, true, true)
	warnings, err := validator.Validate(validRR)

	assert.Empty(t, warnings)
	assert.Nil(t, err)
}

type updateRRFunc func(rr *v1.RadixRegistration)

func TestCanRadixApplicationBeUpdated(t *testing.T) {
	_, client := validRRSetup()
	var testScenarios = []struct {
		name     string
		updateRR updateRRFunc
	}{
		{"empty ConfigurationItem", func(rr *v1.RadixRegistration) { rr.Spec.ConfigurationItem = "" }},
		{"empty ad groups", func(rr *v1.RadixRegistration) { rr.Spec.AdGroups = nil }},
	}

	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validator := radixregistration.CreateOnlineValidator(t.Context(), client, true, true)
			validRR := createValidRR()
			testcase.updateRR(validRR)
			warnings, err := validator.Validate(validRR)

			assert.Empty(t, warnings)
			assert.NotNil(t, err)
		})
	}

	t.Run("name already exist", func(t *testing.T) {
		validRR := createValidRR()
		client = radixfake.NewSimpleClientset(validRR)
		validator := radixregistration.CreateOnlineValidator(t.Context(), client, true, true)
		warnings, err := validator.Validate(validRR)

		assert.Empty(t, warnings)
		assert.Nil(t, err)
	})
}

func createValidRR() *v1.RadixRegistration {
	validRR, _ := utils.GetRadixRegistrationFromFile("testdata/radixregistration.yaml")
	return validRR
}

func validRRSetup() (kubernetes.Interface, radixclient.Interface) {
	kubeclient := kubefake.NewSimpleClientset()
	client := radixfake.NewSimpleClientset()

	return kubeclient, client
}
