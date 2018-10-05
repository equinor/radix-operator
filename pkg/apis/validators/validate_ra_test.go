package validators

import (
	"testing"

	"github.com/statoil/radix-operator/pkg/apis/utils"
	"github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
)

func Test_valid_ra_returns_true(t *testing.T) {
	rr, _ := utils.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	client := fake.NewSimpleClientset(rr)
	ra, _ := utils.GetRadixApplication("testdata/radixconfig.yaml")
	isValid, errs := IsValidRadixApplication(client, ra)

	assert.True(t, isValid)
	assert.True(t, len(errs) == 0)
}

func Test_invalid_ra_returns_false(t *testing.T) {
	ra, _ := utils.GetRadixApplication("testdata/radixconfig.invalid.yaml")
	client := fake.NewSimpleClientset()
	isValid, errs := IsValidRadixApplication(client, ra)

	assert.False(t, isValid)
	assert.False(t, len(errs) == 0)
}

func Test_valid_variable_list(t *testing.T) {
	ra, _ := utils.GetRadixApplication("testdata/radixconfig.yaml")
	err := validateExistEnvForComponentVariables(ra)

	assert.Nil(t, err)
}

func Test_invalid_variable_list(t *testing.T) {
	ra, _ := utils.GetRadixApplication("testdata/radixconfig.invalid.yaml")
	err := validateExistEnvForComponentVariables(ra)

	assert.NotNil(t, err)
}

func Test_valid_app_name_registered(t *testing.T) {
	rr, _ := utils.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	client := fake.NewSimpleClientset(rr)
	ra, _ := utils.GetRadixApplication("testdata/radixconfig.yaml")
	err := validateDoesRRExist(client, ra.Name)

	assert.Nil(t, err)
}

func Test_invalid_app_name_registered(t *testing.T) {
	rr, _ := utils.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	client := fake.NewSimpleClientset(rr)
	ra, _ := utils.GetRadixApplication("testdata/radixconfig.yaml")
	ra.Name = "someotherapp"
	err := validateDoesRRExist(client, ra.Name)

	assert.NotNil(t, err)
}
