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

func Test_valid_ra_returns_true(t *testing.T) {
	_, client := validRASetup()
	validRA := createValidRA()
	isValid, err := radixvalidators.CanRadixApplicationBeInserted(client, validRA)

	assert.True(t, isValid)
	assert.Nil(t, err)
}
func Test_missing_rr(t *testing.T) {
	client := radixfake.NewSimpleClientset()
	validRA := createValidRA()

	isValid, err := radixvalidators.CanRadixApplicationBeInserted(client, validRA)

	assert.False(t, isValid)
	assert.NotNil(t, err)
}

type updateRAFunc func(rr *v1.RadixApplication)

func Test_invalid_ra(t *testing.T) {
	var testScenarios = []struct {
		name     string
		updateRA updateRAFunc
	}{
		{"to long app name", func(ra *v1.RadixApplication) {
			ra.Name = "way.toooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo.long-app-name"
		}},
		{"invalid app name", func(ra *v1.RadixApplication) { ra.Name = "invalid,char.appname" }},
		{"empty name", func(ra *v1.RadixApplication) { ra.Name = "" }},
		{"no related rr", func(ra *v1.RadixApplication) { ra.Name = "no related rr" }},
		{"var connected to non existing env", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentVariables = []v1.EnvVars{
				v1.EnvVars{
					Environment: "nonexistingenv",
					Variables: map[string]string{
						"DB_CON": "somedbcon",
					},
				},
			}
		}},
		{"invalid component name", func(ra *v1.RadixApplication) { ra.Spec.Components[0].Name = "invalid,char.appname" }},
		{"uppercase component name", func(ra *v1.RadixApplication) { ra.Spec.Components[0].Name = "invalidUPPERCASE.appname" }},
		{"invalid port specification. Nil value", func(ra *v1.RadixApplication) { ra.Spec.Components[0].Ports = nil }},
		{"invalid port specification. Empty value", func(ra *v1.RadixApplication) { ra.Spec.Components[0].Ports = []v1.ComponentPort{} }},
		{"invalid port name", func(ra *v1.RadixApplication) { ra.Spec.Components[0].Ports[0].Name = "invalid,char.appname" }},
		{"invalid number of replicas", func(ra *v1.RadixApplication) { ra.Spec.Components[0].Replicas = radixvalidators.MaxReplica + 1 }},
		{"invalid env name", func(ra *v1.RadixApplication) { ra.Spec.Environments[0].Name = "invalid,char.appname" }},
		{"invalid branch name", func(ra *v1.RadixApplication) { ra.Spec.Environments[0].Build.From = "invalid,char.appname" }},
		{"to long branch name", func(ra *v1.RadixApplication) {
			ra.Spec.Environments[0].Build.From = "way.toooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo.long-app-name"
		}},
		{"dns alias non existing component", func(ra *v1.RadixApplication) { ra.Spec.DNSAppAlias.Component = "non existing" }},
		{"dns alias non existing env", func(ra *v1.RadixApplication) { ra.Spec.DNSAppAlias.Environment = "non existing" }},
		{"resource limit unsupported resource", func(ra *v1.RadixApplication) { ra.Spec.Components[0].Resources.Limits["unsupportedResource"] = "250m" }},
		{"resource limit wrong format", func(ra *v1.RadixApplication) { ra.Spec.Components[0].Resources.Limits["memory"] = "asdfasd" }},
		{"resource request wrong format", func(ra *v1.RadixApplication) { ra.Spec.Components[0].Resources.Requests["memory"] = "asdfasd" }},
		{"resource request unsupported resource", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Requests["unsupportedResource"] = "250m"
		}},
	}

	_, client := validRASetup()
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := createValidRA()
			testcase.updateRA(validRA)
			isValid, err := radixvalidators.CanRadixApplicationBeInserted(client, validRA)

			assert.False(t, isValid)
			assert.NotNil(t, err)
		})
	}
}

func Test_ValidRALimitRequest_NoError(t *testing.T) {
	var testScenarios = []struct {
		name     string
		updateRA updateRAFunc
	}{
		{"resource memory correct format: 50", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50"
		}},
		{"resource limit correct format: 50T", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50T"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50T"
		}},
		{"resource limit correct format: 50G", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50G"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50G"
		}},
		{"resource limit correct format: 50M", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50M"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50M"
		}},
		{"resource limit correct format: 50k", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50k"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50k"
		}},
		{"resource limit correct format: 50Gi", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50Gi"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50Gi"
		}},
		{"resource limit correct format: 50Mi", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50Mi"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50Mi"
		}},
		{"resource limit correct format: 50Ki", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50Ki"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50Ki"
		}},
	}

	_, client := validRASetup()
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := createValidRA()
			testcase.updateRA(validRA)
			isValid, err := radixvalidators.CanRadixApplicationBeInserted(client, validRA)

			assert.True(t, isValid)
			assert.Nil(t, err)
		})
	}
}

func Test_InvalidRALimitRequest_Error(t *testing.T) {
	var testScenarios = []struct {
		name     string
		updateRA updateRAFunc
	}{
		{"resource limit incorrect format: 50MB", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50MB"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50MB"
		}},
		{"resource limit incorrect format: 50K", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50K"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50K"
		}},
	}

	_, client := validRASetup()
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := createValidRA()
			testcase.updateRA(validRA)
			isValid, err := radixvalidators.CanRadixApplicationBeInserted(client, validRA)

			assert.False(t, isValid)
			assert.NotNil(t, err)
		})
	}
}

func createValidRA() *v1.RadixApplication {
	validRA, _ := utils.GetRadixApplication("testdata/radixconfig.yaml")

	return validRA
}

func validRASetup() (kubernetes.Interface, radixclient.Interface) {
	validRR, _ := utils.GetRadixRegistrationFromFile("testdata/radixregistration.yaml")
	kubeclient := kubefake.NewSimpleClientset()
	client := radixfake.NewSimpleClientset(validRR)

	return kubeclient, client
}
