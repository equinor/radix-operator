package radixvalidators_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

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
	err := radixvalidators.CanRadixRegistrationBeInserted(client, validRR)

	assert.Nil(t, err)
}

func Test_valid_rr_returns_no_warnings(t *testing.T) {
	_, client := validRRSetup()
	validRR := createValidRR()
	_ = radixvalidators.CanRadixRegistrationBeInserted(client, validRR)
	warnings, err := radixvalidators.GetRadixRegistrationBeInsertedWarnings(client, validRR)
	assert.Empty(t, warnings)
	assert.Nil(t, err)
}

type updateRRFunc func(rr *v1.RadixRegistration)

func TestCanRadixApplicationBeInserted(t *testing.T) {
	var testScenarios = []struct {
		name     string
		updateRR updateRRFunc
	}{
		{"to long app name", func(rr *v1.RadixRegistration) {
			rr.Name = "way.toooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo.long-app-name"
		}},
		{"invalid app name", func(rr *v1.RadixRegistration) { rr.Name = "invalid,char.appname" }},
		{"empty app name", func(rr *v1.RadixRegistration) { rr.Name = "" }},
		{"empty ConfigurationItem", func(rr *v1.RadixRegistration) { rr.Spec.ConfigurationItem = "" }},
		{"ConfigurationItem is too long", func(rr *v1.RadixRegistration) { rr.Spec.ConfigurationItem = strings.Repeat("a", 101) }},
		{"invalid ssh url ending", func(rr *v1.RadixRegistration) { rr.Spec.CloneURL = "git@github.com:auser/go-roman.gitblabla" }},
		{"invalid ssh url start", func(rr *v1.RadixRegistration) { rr.Spec.CloneURL = "asdfasdgit@github.com:auser/go-roman.git" }},
		{"invalid ssh url https", func(rr *v1.RadixRegistration) { rr.Spec.CloneURL = "https://github.com/auser/go-roman" }},
		{"empty ssh url", func(rr *v1.RadixRegistration) { rr.Spec.CloneURL = "" }},
		{"invalid ad group lenght", func(rr *v1.RadixRegistration) { rr.Spec.AdGroups = []string{"7552642f-asdff-fs43-23sf-3ab8f3742c16"} }},
		{"invalid ad group name", func(rr *v1.RadixRegistration) { rr.Spec.AdGroups = []string{"fg_some_group_name"} }},
		{"empty ad group", func(rr *v1.RadixRegistration) { rr.Spec.AdGroups = []string{""} }},
		{"empty configBranch", func(rr *v1.RadixRegistration) { rr.Spec.ConfigBranch = "" }},
		{"invalid configBranch", func(rr *v1.RadixRegistration) { rr.Spec.ConfigBranch = "main.." }},
	}

	_, client := validRRSetup()

	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRR := createValidRR()
			testcase.updateRR(validRR)
			err := radixvalidators.CanRadixRegistrationBeInserted(client, validRR)

			assert.NotNil(t, err)
		})
	}

	t.Run("name already exist", func(t *testing.T) {
		validRR := createValidRR()
		client = radixfake.NewSimpleClientset(validRR)
		err := radixvalidators.CanRadixRegistrationBeInserted(client, validRR)

		assert.NotNil(t, err)
	})

	t.Run("repo already used by other app", func(t *testing.T) {
		validRR := createValidRR()
		client = radixfake.NewSimpleClientset(validRR)
		validRR = createValidRR()
		validRR.Name = "new-app"
		err := radixvalidators.CanRadixRegistrationBeInserted(client, validRR)

		assert.Nil(t, err)
		warnings, err := radixvalidators.GetRadixRegistrationBeInsertedWarnings(client, validRR)
		assert.Nil(t, err)
		assert.NotEmpty(t, warnings)
		assert.Equal(t, "Repository is used in other application(s)", warnings[0])
	})
}

func TestCanRadixApplicationBeUpdated(t *testing.T) {
	var testScenarios = []struct {
		name     string
		updateRR updateRRFunc
	}{
		{"to long app name", func(rr *v1.RadixRegistration) {
			rr.Name = "way.toooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo.long-app-name"
		}},
		{"invalid app name", func(rr *v1.RadixRegistration) { rr.Name = "invalid,char.appname" }},
		{"empty app name", func(rr *v1.RadixRegistration) { rr.Name = "" }},
		{"empty ConfigurationItem", func(rr *v1.RadixRegistration) { rr.Spec.ConfigurationItem = "" }},
		{"ConfigurationItem is too long", func(rr *v1.RadixRegistration) { rr.Spec.ConfigurationItem = strings.Repeat("a", 101) }},
		{"invalid ssh url ending", func(rr *v1.RadixRegistration) { rr.Spec.CloneURL = "git@github.com:auser/go-roman.gitblabla" }},
		{"invalid ssh url start", func(rr *v1.RadixRegistration) { rr.Spec.CloneURL = "asdfasdgit@github.com:auser/go-roman.git" }},
		{"invalid ssh url https", func(rr *v1.RadixRegistration) { rr.Spec.CloneURL = "https://github.com/auser/go-roman" }},
		{"empty ssh url", func(rr *v1.RadixRegistration) { rr.Spec.CloneURL = "" }},
		{"invalid ad group lenght", func(rr *v1.RadixRegistration) { rr.Spec.AdGroups = []string{"7552642f-asdff-fs43-23sf-3ab8f3742c16"} }},
		{"invalid ad group name", func(rr *v1.RadixRegistration) { rr.Spec.AdGroups = []string{"fg_some_group_name"} }},
		{"empty ad group", func(rr *v1.RadixRegistration) { rr.Spec.AdGroups = []string{""} }},
		{"empty configBranch", func(rr *v1.RadixRegistration) { rr.Spec.ConfigBranch = "" }},
		{"invalid configBranch", func(rr *v1.RadixRegistration) { rr.Spec.ConfigBranch = "main.." }},
	}

	_, client := validRRSetup()

	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRR := createValidRR()
			testcase.updateRR(validRR)
			err := radixvalidators.CanRadixRegistrationBeUpdated(validRR)

			assert.NotNil(t, err)
		})
	}

	t.Run("name already exist", func(t *testing.T) {
		validRR := createValidRR()
		client = radixfake.NewSimpleClientset(validRR)
		err := radixvalidators.CanRadixRegistrationBeUpdated(validRR)

		assert.Nil(t, err)
	})

	t.Run("repo already used by other app", func(t *testing.T) {
		validRR := createValidRR()
		client = radixfake.NewSimpleClientset(validRR)
		validRR = createValidRR()
		validRR.Name = "new-app"
		err := radixvalidators.CanRadixRegistrationBeUpdated(validRR)

		assert.Nil(t, err)
		warnings, err := radixvalidators.GetRadixRegistrationBeInsertedWarnings(client, validRR)
		assert.Nil(t, err)
		assert.NotEmpty(t, warnings)
		assert.Equal(t, "Repository is used in other application(s)", warnings[0])
	})
}

func TestCreateApplication_WithRadixConfigFullName(t *testing.T) {
	scenarios := []struct {
		radixConfigFullName string
		expectedError       bool
	}{
		{radixConfigFullName: "", expectedError: false},
		{radixConfigFullName: "radixconfig.yaml", expectedError: false},
		{radixConfigFullName: "a.yaml", expectedError: false},
		{radixConfigFullName: "abc/a.yaml", expectedError: false},
		{radixConfigFullName: "/abc/a.yaml", expectedError: false},
		{radixConfigFullName: " /abc/a.yaml ", expectedError: true},
		{radixConfigFullName: "/abc/de.f/a.yaml", expectedError: false},
		{radixConfigFullName: "abc\\de.f\\a.yaml", expectedError: true},
		{radixConfigFullName: "abc/d-e_f/radixconfig.yaml", expectedError: false},
		{radixConfigFullName: "abc/12.3abc/radixconfig.yaml", expectedError: false},
		{radixConfigFullName: ".yaml", expectedError: true},
		{radixConfigFullName: "radixconfig.yml", expectedError: false},
		{radixConfigFullName: "abc", expectedError: true},
		{radixConfigFullName: "ac", expectedError: true},
		{radixConfigFullName: "a", expectedError: true},
		{radixConfigFullName: "#radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "$radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "%radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "^radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "&radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "*radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "(radixconfig.yaml", expectedError: true},
		{radixConfigFullName: ")radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "+radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "=radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "'radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "]radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "[radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "{radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "}radixconfig.yaml", expectedError: true},
		{radixConfigFullName: ",radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "§radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "±radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "*radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "~radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "`radixconfig.yaml", expectedError: true},
		{radixConfigFullName: ">radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "<radixconfig.yaml", expectedError: true},
		{radixConfigFullName: "@radixconfig.yaml", expectedError: true},
	}
	for _, scenario := range scenarios {
		t.Run(fmt.Sprintf("Test for radixConfigFullName: '%s'", scenario.radixConfigFullName), func(t *testing.T) {
			err := radixvalidators.ValidateRadixConfigFullName(scenario.radixConfigFullName)
			if !scenario.expectedError {
				require.Nil(t, err)
				return
			}
			require.NotNil(t, err)
			assert.Equal(t, "invalid file name for radixconfig. See https://www.radix.equinor.com/references/reference-radix-config/ for more information", err.Error())
		})
	}
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
