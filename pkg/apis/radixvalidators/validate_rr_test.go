package radixvalidators_test

import (
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func Test_valid_rr_returns_true(t *testing.T) {
	_, client := validRRSetup()
	validRR := createValidRR()
	err := radixvalidators.ValidateRadixRegistration(validRR, radixvalidators.RequireAdGroups, radixvalidators.RequireConfigurationItem, radixvalidators.CreateRequireUniqueAppIdValidator(t.Context(), client))
	assert.Nil(t, err)
}

type updateRRFunc func(rr *v1.RadixRegistration)

func TestCanRadixApplicationBeUpdated(t *testing.T) {
	_, client := validRRSetup()
	var testScenarios = []struct {
		name                 string
		updateRR             updateRRFunc
		additionalValidators []radixvalidators.RadixRegistrationValidator
	}{
		{"empty ConfigurationItem", func(rr *v1.RadixRegistration) { rr.Spec.ConfigurationItem = "" }, []radixvalidators.RadixRegistrationValidator{radixvalidators.RequireAdGroups, radixvalidators.RequireConfigurationItem, radixvalidators.CreateRequireUniqueAppIdValidator(t.Context(), client)}},
		{"empty ad groups", func(rr *v1.RadixRegistration) { rr.Spec.AdGroups = nil }, []radixvalidators.RadixRegistrationValidator{radixvalidators.RequireAdGroups, radixvalidators.RequireConfigurationItem, radixvalidators.CreateRequireUniqueAppIdValidator(t.Context(), client)}},
	}

	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRR := createValidRR()
			testcase.updateRR(validRR)
			err := radixvalidators.ValidateRadixRegistration(validRR, testcase.additionalValidators...)

			assert.NotNil(t, err)
		})
	}

	t.Run("name already exist", func(t *testing.T) {
		validRR := createValidRR()
		client = radixfake.NewSimpleClientset(validRR)
		err := radixvalidators.ValidateRadixRegistration(validRR)

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
