package validators

import (
	"testing"

	"github.com/statoil/radix-operator/pkg/apis/utils"
	"github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
)

func Test_valid_rr_returns_true(t *testing.T) {
	client := fake.NewSimpleClientset()
	rr, _ := utils.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	isValid, errs := IsValidRadixRegistration(client, rr)

	assert.True(t, isValid)
	assert.True(t, len(errs) == 0)
}

func Test_invalid_rr_returns_false(t *testing.T) {
	rr, _ := utils.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	client := fake.NewSimpleClientset(rr)
	rr.Spec.CloneURL = "not correct clone url"
	rr.Spec.AdGroups = []string{"incorrect_ad_group"}
	isValid, errs := IsValidRadixRegistration(client, rr)

	assert.False(t, isValid)
	assert.False(t, len(errs) == 0)
}

func Test_valid_app_name(t *testing.T) {
	client := fake.NewSimpleClientset()
	err := validateDoesNameAlreadyExist(client, "coolapp")

	assert.Nil(t, err)
}

func Test_invalid_app_name(t *testing.T) {
	rr, _ := utils.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	client := fake.NewSimpleClientset(rr)
	err := validateDoesNameAlreadyExist(client, "testapp")

	assert.NotNil(t, err)
}

func Test_valid_ssh_url(t *testing.T) {
	err := validateGitSSHUrl("git@github.com:Statoil/radix-web-console.git")

	assert.Nil(t, err)
}

func Test_valid_ssh_url_priv_repo(t *testing.T) {
	err := validateGitSSHUrl("git@github.com:auser/go-roman.git")

	assert.Nil(t, err)
}

func Test_invalid_ssh_url_ending(t *testing.T) {
	err := validateGitSSHUrl("git@github.com:auser/go-roman.gitblabla")

	assert.NotNil(t, err)
}

func Test_invalid_ssh_url_start(t *testing.T) {
	err := validateGitSSHUrl("asdfasdgit@github.com:auser/go-roman.git")

	assert.NotNil(t, err)
}

func Test_invalid_ssh_url_https(t *testing.T) {
	err := validateGitSSHUrl("https://github.com/auser/go-roman")

	assert.NotNil(t, err)
}

func Test_valid_ad_group_reference(t *testing.T) {
	err := validateAdGroups([]string{"7552642f-as34-fs43-23sf-3ab8f3742c16", "7552642f-as34-234d-23sf-3ab8f3742c16"})

	assert.Nil(t, err)
}

func Test_invalid_ad_group_reference_length(t *testing.T) {
	err := validateAdGroups([]string{"7552642f-asdff-fs43-23sf-3ab8f3742c16"})

	assert.NotNil(t, err)
}

func Test_invalid_ad_group_reference_name(t *testing.T) {
	err := validateAdGroups([]string{"fg_some_group_name"})

	assert.NotNil(t, err)
}
