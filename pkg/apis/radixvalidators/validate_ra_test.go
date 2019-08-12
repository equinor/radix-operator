package radixvalidators_test

import (
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/errors"
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
	validRAFirstComponentName := "app"

	wayTooLongName := "waytoooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooolongname"
	tooLongPortName := "abcdefghijklmnop"
	invalidResourceName := "invalid,char.resourcename"
	invalidVariableName := "invalid:variable"
	noReleatedRRAppName := "no related rr"
	noExistingEnvironment := "nonexistingenv"
	invalidUpperCaseResourceName := "invalidUPPERCASE.resourcename"
	nonExistingComponent := "non existing"
	unsupportedResource := "unsupportedResource"
	invalidResourceValue := "asdfasd"

	var testScenarios = []struct {
		name          string
		expectedError error
		updateRA      updateRAFunc
	}{
		{"no error", nil, func(ra *v1.RadixApplication) {}},
		{"too long app name", radixvalidators.InvalidAppNameLengthError(wayTooLongName), func(ra *v1.RadixApplication) { ra.Name = wayTooLongName }},
		{"invalid app name", radixvalidators.InvalidAppNameError(invalidResourceName), func(ra *v1.RadixApplication) { ra.Name = invalidResourceName }},
		{"empty name", radixvalidators.AppNameCannotBeEmptyError(), func(ra *v1.RadixApplication) { ra.Name = "" }},
		{"no related rr", radixvalidators.NoRegistrationExistsForApplicationError(noReleatedRRAppName), func(ra *v1.RadixApplication) { ra.Name = noReleatedRRAppName }},
		{"non existing env for component", radixvalidators.EnvironmentReferencedByComponentDoesNotExistError(noExistingEnvironment, validRAFirstComponentName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig = []v1.RadixEnvironmentConfig{
				v1.RadixEnvironmentConfig{
					Environment: noExistingEnvironment,
				},
			}
		}},
		{"invalid component name", radixvalidators.InvalidResourceNameError("component name", invalidResourceName), func(ra *v1.RadixApplication) { ra.Spec.Components[0].Name = invalidResourceName }},
		{"uppercase component name", radixvalidators.InvalidResourceNameError("component name", invalidUpperCaseResourceName), func(ra *v1.RadixApplication) { ra.Spec.Components[0].Name = invalidUpperCaseResourceName }},
		{"invalid port specification. Nil value", radixvalidators.PortSpecificationCannotBeEmptyForComponentError(validRAFirstComponentName), func(ra *v1.RadixApplication) { ra.Spec.Components[0].Ports = nil }},
		{"invalid port specification. Empty value", radixvalidators.PortSpecificationCannotBeEmptyForComponentError(validRAFirstComponentName), func(ra *v1.RadixApplication) { ra.Spec.Components[0].Ports = []v1.ComponentPort{} }},
		{"invalid port name", radixvalidators.InvalidResourceNameError("port name", invalidResourceName), func(ra *v1.RadixApplication) { ra.Spec.Components[0].Ports[0].Name = invalidResourceName }},
		{"too long port name", radixvalidators.InvalidPortNameLengthError(tooLongPortName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].PublicPort = tooLongPortName
			ra.Spec.Components[0].Ports[0].Name = tooLongPortName
		}},
		{"invalid secret name", radixvalidators.InvalidResourceNameError("secret name", invalidVariableName), func(ra *v1.RadixApplication) { ra.Spec.Components[1].Secrets[0] = invalidVariableName }},
		{"too long secret name", radixvalidators.InvalidResourceNameLengthError("secret name", wayTooLongName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[1].Secrets[0] = wayTooLongName
		}},
		{"invalid environment variable name", radixvalidators.InvalidResourceNameError("environment variable name", invalidVariableName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[1].EnvironmentConfig[0].Variables[invalidVariableName] = "Any value"
		}},
		{"too long environment variable name", radixvalidators.InvalidResourceNameLengthError("environment variable name", wayTooLongName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[1].EnvironmentConfig[0].Variables[wayTooLongName] = "Any value"
		}},
		{"invalid number of replicas", radixvalidators.InvalidNumberOfReplicaError(radixvalidators.MaxReplica + 1), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Replicas = radixvalidators.MaxReplica + 1
		}},
		{"invalid env name", radixvalidators.InvalidResourceNameError("env name", invalidResourceName), func(ra *v1.RadixApplication) { ra.Spec.Environments[0].Name = invalidResourceName }},
		{"invalid branch name", radixvalidators.InvalidResourceNameError("branch from", invalidResourceName), func(ra *v1.RadixApplication) { ra.Spec.Environments[0].Build.From = invalidResourceName }},
		{"too long branch name", radixvalidators.InvalidResourceNameLengthError("branch from", wayTooLongName), func(ra *v1.RadixApplication) {
			ra.Spec.Environments[0].Build.From = wayTooLongName
		}},
		{"dns alias non existing component", radixvalidators.ComponentForDNSAppAliasNotDefinedError(nonExistingComponent), func(ra *v1.RadixApplication) { ra.Spec.DNSAppAlias.Component = nonExistingComponent }},
		{"dns alias non existing env", radixvalidators.EnvForDNSAppAliasNotDefinedError(noExistingEnvironment), func(ra *v1.RadixApplication) { ra.Spec.DNSAppAlias.Environment = noExistingEnvironment }},
		{"dns external alias non existing component", radixvalidators.ComponentForDNSExternalAliasNotDefinedError(nonExistingComponent), func(ra *v1.RadixApplication) {
			ra.Spec.DNSExternalAlias = []v1.ExternalAlias{
				v1.ExternalAlias{
					Alias:       "some.alias.com",
					Component:   nonExistingComponent,
					Environment: ra.Spec.Environments[0].Name,
				},
			}
		}},
		{"dns external alias non existing environment", radixvalidators.EnvForDNSExternalAliasNotDefinedError(noExistingEnvironment), func(ra *v1.RadixApplication) {
			ra.Spec.DNSExternalAlias = []v1.ExternalAlias{
				v1.ExternalAlias{
					Alias:       "some.alias.com",
					Component:   ra.Spec.Components[0].Name,
					Environment: noExistingEnvironment,
				},
			}
		}},
		{"dns external alias non existing alias", radixvalidators.ExternalAliasCannotBeEmptyError(), func(ra *v1.RadixApplication) {
			ra.Spec.DNSExternalAlias = []v1.ExternalAlias{
				v1.ExternalAlias{
					Component:   ra.Spec.Components[0].Name,
					Environment: ra.Spec.Environments[0].Name,
				},
			}
		}},
		{"dns external alias with no public port", radixvalidators.ComponentForDNSExternalAliasIsNotMarkedAsPublicError(validRAFirstComponentName), func(ra *v1.RadixApplication) {
			// Backward compatible setting
			ra.Spec.Components[0].Public = false
			ra.Spec.Components[0].PublicPort = ""
			ra.Spec.DNSExternalAlias = []v1.ExternalAlias{
				v1.ExternalAlias{
					Alias:       "some.alias.com",
					Component:   ra.Spec.Components[0].Name,
					Environment: ra.Spec.Environments[0].Name,
				},
			}
		}},
		{"duplicate dns external alias", radixvalidators.DuplicateExternalAliasError(), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Public = true
			ra.Spec.DNSExternalAlias = []v1.ExternalAlias{
				v1.ExternalAlias{
					Alias:       "duplicate.alias.com",
					Component:   ra.Spec.Components[0].Name,
					Environment: ra.Spec.Environments[0].Name,
				},
				v1.ExternalAlias{
					Alias:       "duplicate.alias.com",
					Component:   ra.Spec.Components[0].Name,
					Environment: ra.Spec.Environments[0].Name,
				},
			}
		}},
		{"resource limit unsupported resource", radixvalidators.InvalidResourceError(unsupportedResource), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits[unsupportedResource] = "250m"
		}},
		{"memory resource limit wrong format", radixvalidators.MemoryResourceRequirementFormatError(invalidResourceValue), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = invalidResourceValue
		}},
		{"memory resource request wrong format", radixvalidators.MemoryResourceRequirementFormatError(invalidResourceValue), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = invalidResourceValue
		}},
		{"cpu resource limit wrong format", radixvalidators.CPUResourceRequirementFormatError(invalidResourceValue), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["cpu"] = invalidResourceValue
		}},
		{"cpu resource request wrong format", radixvalidators.CPUResourceRequirementFormatError(invalidResourceValue), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["cpu"] = invalidResourceValue
		}},
		{"resource request unsupported resource", radixvalidators.InvalidResourceError(unsupportedResource), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests[unsupportedResource] = "250m"
		}},
	}

	_, client := validRASetup()
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := createValidRA()
			testcase.updateRA(validRA)
			isValid, errs := radixvalidators.CanRadixApplicationBeInsertedErrors(client, validRA)

			if testcase.expectedError != nil {
				assert.False(t, isValid)
				assert.NotNil(t, errs)

				assert.Truef(t, errors.Contains(errs, testcase.expectedError), "Expected error is not contained in list of errors")
			} else {
				assert.True(t, isValid)
				assert.Nil(t, errs)
			}
		})
	}
}

func Test_ValidRALimitRequest_NoError(t *testing.T) {
	var testScenarios = []struct {
		name     string
		updateRA updateRAFunc
	}{
		{"resource memory correct format: 50", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50"
		}},
		{"resource limit correct format: 50T", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50T"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50T"
		}},
		{"resource limit correct format: 50G", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50G"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50G"
		}},
		{"resource limit correct format: 50M", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50M"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50M"
		}},
		{"resource limit correct format: 50k", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50k"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50k"
		}},
		{"resource limit correct format: 50Gi", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50Gi"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50Gi"
		}},
		{"resource limit correct format: 50Mi", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50Mi"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50Mi"
		}},
		{"resource limit correct format: 50Ki", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50Ki"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50Ki"
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
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50MB"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50MB"
		}},
		{"resource limit incorrect format: 50K", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50K"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50K"
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

func Test_PublicPort(t *testing.T) {
	var testScenarios = []struct {
		name       string
		updateRA   updateRAFunc
		isValid    bool
		isErrorNil bool
	}{
		{
			name: "matching port name for public component, old public does not exist",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Components[0].PublicPort = "http"
				ra.Spec.Components[0].Ports[0].Name = "http"
				ra.Spec.Components[0].Public = false
			},
			isValid:    true,
			isErrorNil: true,
		},
		{
			// For backwards compatibility
			name: "matching port name for public component, old public exists (ignored)",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Components[0].PublicPort = "http"
				ra.Spec.Components[0].Ports[0].Name = "http"
				ra.Spec.Components[0].Public = true
			},
			isValid:    true,
			isErrorNil: true,
		},
		{
			name: "port name is irrelevant for non-public component if old public does not exist",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Components[0].PublicPort = ""
				ra.Spec.Components[0].Ports[0].Name = "test"
				ra.Spec.Components[0].Public = false
			},
			isValid:    true,
			isErrorNil: true,
		},
		{
			// For backwards compatibility
			name: "old public is used if it exists and new publicPort does not exist",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Components[0].PublicPort = ""
				ra.Spec.Components[0].Ports[0].Name = "test"
				ra.Spec.Components[0].Public = true
			},
			isValid:    true,
			isErrorNil: true,
		},
		{
			name: "missing port name for public component, old public does not exist",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Components[0].PublicPort = "http"
				ra.Spec.Components[0].Ports[0].Name = "test"
				ra.Spec.Components[0].Public = false
			},
			isValid:    false,
			isErrorNil: false,
		},
		{
			// For backwards compatibility
			name: "missing port name for public component, old public exists (ignored)",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Components[0].PublicPort = "http"
				ra.Spec.Components[0].Ports[0].Name = "test"
				ra.Spec.Components[0].Public = true
			},
			isValid:    false,
			isErrorNil: false,
		},
		{
			name: "duplicate port name for public component, old public does not exist",
			updateRA: func(ra *v1.RadixApplication) {
				newPorts := []v1.ComponentPort{
					{
						Name: "http",
						Port: 8080,
					},
					{
						Name: "http",
						Port: 1234,
					},
				}
				ra.Spec.Components[0].Ports = newPorts
				ra.Spec.Components[0].PublicPort = "http"
				ra.Spec.Components[0].Public = false
			},
			isValid:    false,
			isErrorNil: false,
		},
		{
			// For backwards compatibility
			name: "duplicate port name for public component, old public exists (ignored)",
			updateRA: func(ra *v1.RadixApplication) {
				newPorts := []v1.ComponentPort{
					{
						Name: "http",
						Port: 8080,
					},
					{
						Name: "http",
						Port: 1234,
					},
				}
				ra.Spec.Components[0].Ports = newPorts
				ra.Spec.Components[0].PublicPort = "http"
				ra.Spec.Components[0].Public = true
			},
			isValid:    false,
			isErrorNil: false,
		},
	}

	_, client := validRASetup()
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := createValidRA()
			testcase.updateRA(validRA)
			isValid, err := radixvalidators.CanRadixApplicationBeInserted(client, validRA)
			isErrorNil := false
			if err == nil {
				isErrorNil = true
			}

			assert.Equal(t, testcase.isValid, isValid)
			assert.Equal(t, testcase.isErrorNil, isErrorNil)
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
