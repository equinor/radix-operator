package radixvalidators_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/errors"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

type updateRAFunc func(rr *v1.RadixApplication)

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

func Test_application_name_casing_is_validated(t *testing.T) {
	mixedCaseName := "Radix-Test-APPLICATION"
	lowerCaseName := "radix-test-application"
	upperCaseName := "RADIX-TEST-APPLICATION"
	expectedName := "radix-test-application"

	var testScenarios = []struct {
		name          string
		expectedError error
		updateRa      updateRAFunc
	}{
		{"Mixed case name", radixvalidators.ApplicationNameNotLowercaseError(mixedCaseName), func(ra *v1.RadixApplication) { ra.Name = mixedCaseName }},
		{"Lower case name", radixvalidators.ApplicationNameNotLowercaseError(lowerCaseName), func(ra *v1.RadixApplication) { ra.Name = lowerCaseName }},
		{"Upper case name", radixvalidators.ApplicationNameNotLowercaseError(upperCaseName), func(ra *v1.RadixApplication) { ra.Name = upperCaseName }},
	}

	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := createValidRA()
			testcase.updateRa(validRA)
			isValid, err := radixvalidators.IsApplicationNameLowercase(validRA.Name)

			if err != nil {
				assert.False(t, isValid)
				assert.NotNil(t, err)
				assert.True(t, testcase.expectedError.Error() == err.Error())
				assert.True(t, strings.ToLower(validRA.Name) == expectedName)
			} else {
				assert.True(t, isValid)
				assert.Nil(t, err)
			}
		})
	}
}

func Test_invalid_ra(t *testing.T) {
	validRAFirstComponentName := "app"
	validRAFirstJobName := "job"
	validRASecondComponentName := "redis"
	validRAComponentNameApp2 := "app2"

	wayTooLongName := "waytoooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooolongname"
	tooLongPortName := "abcdefghijklmnop"
	invalidBranchName := "/master"
	invalidResourceName := "invalid,char.resourcename"
	oauthAuxSuffixComponentName := fmt.Sprintf("app-%s", defaults.OAuthProxyAuxiliaryComponentSuffix)
	oauthAuxSuffixJobName := fmt.Sprintf("job-%s", defaults.OAuthProxyAuxiliaryComponentSuffix)
	invalidVariableName := "invalid:variable"
	noReleatedRRAppName := "no related rr"
	noExistingEnvironment := "nonexistingenv"
	invalidUpperCaseResourceName := "invalidUPPERCASE.resourcename"
	nonExistingComponent := "nonexisting"
	unsupportedResource := "unsupportedResource"
	invalidResourceValue := "asdfasd"
	conflictingVariableName := "some-variable"
	invalidCertificateVerification := v1.VerificationType("obviously_an_invalid_value")
	name50charsLong := "a123456789a123456789a123456789a123456789a123456789"

	var testScenarios = []struct {
		name          string
		expectedError error
		updateRA      updateRAFunc
	}{
		{"no error", nil, func(ra *v1.RadixApplication) {}},
		{"too long app name", radixvalidators.InvalidAppNameLengthError(wayTooLongName), func(ra *v1.RadixApplication) {
			ra.Name = wayTooLongName
		}},
		{"invalid app name", radixvalidators.InvalidLowerCaseAlphaNumericDashResourceNameError("app name", invalidResourceName), func(ra *v1.RadixApplication) {
			ra.Name = invalidResourceName
		}},
		{"empty name", radixvalidators.AppNameCannotBeEmptyError(), func(ra *v1.RadixApplication) {
			ra.Name = ""
		}},
		{"no related rr", radixvalidators.NoRegistrationExistsForApplicationError(noReleatedRRAppName), func(ra *v1.RadixApplication) {
			ra.Name = noReleatedRRAppName
		}},
		{"non existing env for component", radixvalidators.EnvironmentReferencedByComponentDoesNotExistError(noExistingEnvironment, validRAFirstComponentName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig = []v1.RadixEnvironmentConfig{
				{
					Environment: noExistingEnvironment,
				},
			}
		}},
		{"invalid component name", radixvalidators.InvalidLowerCaseAlphaNumericDashResourceNameError("component name", invalidResourceName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Name = invalidResourceName
		}},
		{"uppercase component name", radixvalidators.InvalidLowerCaseAlphaNumericDashResourceNameError("component name", invalidUpperCaseResourceName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Name = invalidUpperCaseResourceName
		}},
		{"duplicate component name", radixvalidators.DuplicateComponentOrJobNameError([]string{validRAFirstComponentName}), func(ra *v1.RadixApplication) {
			ra.Spec.Components = append(ra.Spec.Components, *ra.Spec.Components[0].DeepCopy())
		}},
		{"component name with oauth auxiliary name suffix", radixvalidators.ComponentNameReservedSuffixError(oauthAuxSuffixComponentName, "component", defaults.OAuthProxyAuxiliaryComponentSuffix), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Name = oauthAuxSuffixComponentName
		}},
		{"invalid port specification. Nil value", radixvalidators.PortSpecificationCannotBeEmptyForComponentError(validRAFirstComponentName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Ports = nil
		}},
		{"invalid port specification. Empty value", radixvalidators.PortSpecificationCannotBeEmptyForComponentError(validRAFirstComponentName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Ports = []v1.ComponentPort{}
		}},
		{"invalid port name", radixvalidators.InvalidLowerCaseAlphaNumericDashResourceNameError("port name", invalidResourceName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Ports[0].Name = invalidResourceName
		}},
		{"too long port name", radixvalidators.InvalidPortNameLengthError(tooLongPortName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].PublicPort = tooLongPortName
			ra.Spec.Components[0].Ports[0].Name = tooLongPortName
		}},
		{"invalid build secret name", radixvalidators.InvalidResourceNameError("build secret name", invalidVariableName), func(ra *v1.RadixApplication) {
			ra.Spec.Build = &v1.BuildSpec{
				Secrets: []string{invalidVariableName},
			}
		}},
		{"too long build secret name", radixvalidators.InvalidStringValueMaxLengthError("build secret name", wayTooLongName, 253), func(ra *v1.RadixApplication) {
			ra.Spec.Build = &v1.BuildSpec{
				Secrets: []string{wayTooLongName},
			}
		}},
		{"invalid secret name", radixvalidators.InvalidResourceNameError("secret name", invalidVariableName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[1].Secrets[0] = invalidVariableName
		}},
		{"too long secret name", radixvalidators.InvalidStringValueMaxLengthError("secret name", wayTooLongName, 253), func(ra *v1.RadixApplication) {
			ra.Spec.Components[1].Secrets[0] = wayTooLongName
		}},
		{"invalid environment variable name", radixvalidators.InvalidResourceNameError("environment variable name", invalidVariableName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[1].EnvironmentConfig[0].Variables[invalidVariableName] = "Any value"
		}},
		{"too long environment variable name", radixvalidators.InvalidStringValueMaxLengthError("environment variable name", wayTooLongName, 253), func(ra *v1.RadixApplication) {
			ra.Spec.Components[1].EnvironmentConfig[0].Variables[wayTooLongName] = "Any value"
		}},
		{"conflicting variable and secret name", radixvalidators.SecretNameConflictsWithEnvironmentVariable(validRASecondComponentName, conflictingVariableName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[1].EnvironmentConfig[0].Variables[conflictingVariableName] = "Any value"
			ra.Spec.Components[1].Secrets[0] = conflictingVariableName
		}},
		{"invalid common environment variable name", radixvalidators.InvalidResourceNameError("environment variable name", invalidVariableName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[1].Variables[invalidVariableName] = "Any value"
		}},
		{"too long common environment variable name", radixvalidators.InvalidStringValueMaxLengthError("environment variable name", wayTooLongName, 253), func(ra *v1.RadixApplication) {
			ra.Spec.Components[1].Variables[wayTooLongName] = "Any value"
		}},
		{"conflicting common variable and secret name", radixvalidators.SecretNameConflictsWithEnvironmentVariable(validRASecondComponentName, conflictingVariableName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[1].Variables[conflictingVariableName] = "Any value"
			ra.Spec.Components[1].Secrets[0] = conflictingVariableName
		}},
		{"conflicting common variable and secret name when not environment config", radixvalidators.SecretNameConflictsWithEnvironmentVariable(validRASecondComponentName, conflictingVariableName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[1].Variables[conflictingVariableName] = "Any value"
			ra.Spec.Components[1].Secrets[0] = conflictingVariableName
			ra.Spec.Components[1].EnvironmentConfig = nil
		}},
		{"invalid number of replicas", radixvalidators.InvalidNumberOfReplicaError(radixvalidators.MaxReplica + 1), func(ra *v1.RadixApplication) {
			*ra.Spec.Components[0].EnvironmentConfig[0].Replicas = radixvalidators.MaxReplica + 1
		}},
		{"invalid env name", radixvalidators.InvalidLowerCaseAlphaNumericDashResourceNameError("env name", invalidResourceName), func(ra *v1.RadixApplication) {
			ra.Spec.Environments[0].Name = invalidResourceName
		}},
		{"invalid branch name", radixvalidators.InvalidBranchNameError(invalidBranchName), func(ra *v1.RadixApplication) {
			ra.Spec.Environments[0].Build.From = invalidBranchName
		}},
		{"too long branch name", radixvalidators.InvalidStringValueMaxLengthError("branch from", wayTooLongName, 253), func(ra *v1.RadixApplication) {
			ra.Spec.Environments[0].Build.From = wayTooLongName
		}},
		{"dns app alias non existing component", radixvalidators.ComponentForDNSAppAliasNotDefinedError(nonExistingComponent), func(ra *v1.RadixApplication) {
			ra.Spec.DNSAppAlias.Component = nonExistingComponent
		}},
		{"dns app alias non existing env", radixvalidators.EnvForDNSAppAliasNotDefinedError(noExistingEnvironment), func(ra *v1.RadixApplication) {
			ra.Spec.DNSAppAlias.Environment = noExistingEnvironment
		}},
		{"dns alias is empty", radixvalidators.ResourceNameCannotBeEmptyError("dnsAlias component"), func(ra *v1.RadixApplication) {
			ra.Spec.DNSAlias[0].Component = ""
		}},
		{"dns alias is empty", radixvalidators.ResourceNameCannotBeEmptyError("dnsAlias environment"), func(ra *v1.RadixApplication) {
			ra.Spec.DNSAlias[0].Environment = ""
		}},
		{"dns alias is invalid", radixvalidators.InvalidLowerCaseAlphaNumericDashResourceNameError("dnsAlias component", "component.abc"), func(ra *v1.RadixApplication) {
			ra.Spec.DNSAlias[0].Component = "component.abc"
		}},
		{"dns alias is invalid", radixvalidators.InvalidLowerCaseAlphaNumericDashResourceNameError("dnsAlias environment", "environment.abc"), func(ra *v1.RadixApplication) {
			ra.Spec.DNSAlias[0].Environment = "environment.abc"
		}},
		{"dns alias non existing component", radixvalidators.ComponentForDNSAliasNotDefinedError(nonExistingComponent), func(ra *v1.RadixApplication) {
			ra.Spec.DNSAlias[0].Component = nonExistingComponent
		}},
		{"dns alias non existing env", radixvalidators.EnvForDNSAliasNotDefinedError(noExistingEnvironment), func(ra *v1.RadixApplication) {
			ra.Spec.DNSAlias[0].Environment = noExistingEnvironment
		}},
		{"dns alias domain is empty", radixvalidators.ResourceNameCannotBeEmptyError("dnsAlias domain"), func(ra *v1.RadixApplication) {
			ra.Spec.DNSAlias[0].Domain = ""
		}},
		{"dns alias domain is invalid", radixvalidators.InvalidLowerCaseAlphaNumericDashResourceNameError("dnsAlias domain", "my.domain"), func(ra *v1.RadixApplication) {
			ra.Spec.DNSAlias[0].Domain = "my.domain"
		}},
		{"dns alias domain is invalid", radixvalidators.DuplicateDomainForDNSAliasError("my-domain"), func(ra *v1.RadixApplication) {
			ra.Spec.DNSAlias = append(ra.Spec.DNSAlias, ra.Spec.DNSAlias[0])
		}},
		{"dns alias with no public port", radixvalidators.ComponentForDNSAliasIsNotMarkedAsPublicError(validRAComponentNameApp2), func(ra *v1.RadixApplication) {
			ra.Spec.Components[3].PublicPort = ""
			ra.Spec.Components[3].Public = false
			ra.Spec.DNSAlias[0] = v1.DNSAlias{
				Domain:      "my-domain",
				Component:   ra.Spec.Components[3].Name,
				Environment: ra.Spec.Environments[0].Name,
			}
		}},
		{"dns external alias non existing component", radixvalidators.ComponentForDNSExternalAliasNotDefinedError(nonExistingComponent), func(ra *v1.RadixApplication) {
			ra.Spec.DNSExternalAlias = []v1.ExternalAlias{
				{
					Alias:       "some.alias.com",
					Component:   nonExistingComponent,
					Environment: ra.Spec.Environments[0].Name,
				},
			}
		}},
		{"dns external alias non existing environment", radixvalidators.EnvForDNSExternalAliasNotDefinedError(noExistingEnvironment), func(ra *v1.RadixApplication) {
			ra.Spec.DNSExternalAlias = []v1.ExternalAlias{
				{
					Alias:       "some.alias.com",
					Component:   ra.Spec.Components[0].Name,
					Environment: noExistingEnvironment,
				},
			}
		}},
		{"dns external alias non existing alias", radixvalidators.ExternalAliasCannotBeEmptyError(), func(ra *v1.RadixApplication) {
			ra.Spec.DNSExternalAlias = []v1.ExternalAlias{
				{
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
				{
					Alias:       "some.alias.com",
					Component:   ra.Spec.Components[0].Name,
					Environment: ra.Spec.Environments[0].Name,
				},
			}
		}},
		{"duplicate dns external alias", radixvalidators.DuplicateExternalAliasError(), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Public = true
			ra.Spec.DNSExternalAlias = []v1.ExternalAlias{
				{
					Alias:       "duplicate.alias.com",
					Component:   ra.Spec.Components[0].Name,
					Environment: ra.Spec.Environments[0].Name,
				},
				{
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
		{"memory resource request larger than limit", radixvalidators.ResourceRequestOverLimitError("memory", "249Mi", "250Ki"), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "250Ki"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "249Mi"
		}},
		{"cpu resource limit wrong format", radixvalidators.CPUResourceRequirementFormatError(invalidResourceValue), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["cpu"] = invalidResourceValue
		}},
		{"cpu resource request wrong format", radixvalidators.CPUResourceRequirementFormatError(invalidResourceValue), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["cpu"] = invalidResourceValue
		}},
		{"cpu resource request larger than limit", radixvalidators.ResourceRequestOverLimitError("cpu", "251m", "250m"), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["cpu"] = "250m"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["cpu"] = "251m"
		}},
		{"resource request unsupported resource", radixvalidators.InvalidResourceError(unsupportedResource), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests[unsupportedResource] = "250m"
		}},
		{"common resource limit unsupported resource", radixvalidators.InvalidResourceError(unsupportedResource), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits[unsupportedResource] = "250m"
		}},
		{"common resource request unsupported resource", radixvalidators.InvalidResourceError(unsupportedResource), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Requests[unsupportedResource] = "250m"
		}},
		{"common memory resource limit wrong format", radixvalidators.MemoryResourceRequirementFormatError(invalidResourceValue), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = invalidResourceValue
		}},
		{"common memory resource request wrong format", radixvalidators.MemoryResourceRequirementFormatError(invalidResourceValue), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Requests["memory"] = invalidResourceValue
		}},
		{"common cpu resource limit wrong format", radixvalidators.CPUResourceRequirementFormatError(invalidResourceValue), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["cpu"] = invalidResourceValue
		}},
		{"common cpu resource request wrong format", radixvalidators.CPUResourceRequirementFormatError(invalidResourceValue), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Requests["cpu"] = invalidResourceValue
		}},
		{"cpu resource limit is empty", nil, func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["cpu"] = ""
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["cpu"] = "251m"
		}},
		{"cpu resource limit not set", nil, func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["cpu"] = "251m"
		}},
		{"memory resource limit is empty", nil, func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = ""
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "249Mi"
		}},
		{"memory resource limit not set", nil, func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "249Mi"
		}},
		{"wrong public image config", radixvalidators.PublicImageComponentCannotHaveSourceOrDockerfileSet(validRAFirstComponentName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Image = "redis:5.0-alpine"
			ra.Spec.Components[0].SourceFolder = "./api"
			ra.Spec.Components[0].DockerfileName = ".Dockerfile"
		}},
		{"missing environment config for dynamic tag", radixvalidators.ComponentWithDynamicTagRequiresTagInEnvironmentConfig(validRAFirstComponentName), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Image = "radixcanary.azurecr.io/my-private-image:{imageTagName}"
			ra.Spec.Components[0].EnvironmentConfig = []v1.RadixEnvironmentConfig{}
		}},
		{"missing dynamic tag config for mapped environment", radixvalidators.ComponentWithDynamicTagRequiresTagInEnvironmentConfigForEnvironment(validRAFirstComponentName, "dev"), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Image = "radixcanary.azurecr.io/my-private-image:{imageTagName}"
			ra.Spec.Components[0].EnvironmentConfig[0].ImageTagName = ""
			ra.Spec.Components[0].EnvironmentConfig = append(ra.Spec.Components[0].EnvironmentConfig, v1.RadixEnvironmentConfig{
				Environment:  "dev",
				ImageTagName: "",
			})
		}},
		{"inconcistent dynamic tag config for environment", radixvalidators.ComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTag(validRAFirstComponentName, "prod"), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Image = "radixcanary.azurecr.io/my-private-image:some-tag"
			ra.Spec.Components[0].EnvironmentConfig[0].ImageTagName = "any-tag"
		}},
		{"invalid verificationType for component", radixvalidators.InvalidVerificationType(string(invalidCertificateVerification)), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Authentication = &v1.Authentication{
				ClientCertificate: &v1.ClientCertificate{
					Verification: &invalidCertificateVerification,
				},
			}
		}},
		{"invalid verificationType for environment", radixvalidators.InvalidVerificationType(string(invalidCertificateVerification)), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Authentication = &v1.Authentication{
				ClientCertificate: &v1.ClientCertificate{
					Verification: &invalidCertificateVerification,
				},
			}
		}},
		{"duplicate job name", radixvalidators.DuplicateComponentOrJobNameError([]string{validRAFirstJobName}), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs = append(ra.Spec.Jobs, *ra.Spec.Jobs[0].DeepCopy())
		}},
		{"job name with oauth auxiliary name suffix", radixvalidators.ComponentNameReservedSuffixError(oauthAuxSuffixJobName, "job", defaults.OAuthProxyAuxiliaryComponentSuffix), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Name = oauthAuxSuffixJobName
		}},
		{"invalid job secret name", radixvalidators.InvalidResourceNameError("secret name", invalidVariableName), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Secrets[0] = invalidVariableName
		}},
		{"too long job secret name", radixvalidators.InvalidStringValueMaxLengthError("secret name", wayTooLongName, 253), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Secrets[0] = wayTooLongName
		}},
		{"invalid job common environment variable name", radixvalidators.InvalidResourceNameError("environment variable name", invalidVariableName), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Variables[invalidVariableName] = "Any value"
		}},
		{"too long job common environment variable name", radixvalidators.InvalidStringValueMaxLengthError("environment variable name", wayTooLongName, 253), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Variables[wayTooLongName] = "Any value"
		}},
		{"invalid job environment variable name", radixvalidators.InvalidResourceNameError("environment variable name", invalidVariableName), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Variables[invalidVariableName] = "Any value"
		}},
		{"too long job environment variable name", radixvalidators.InvalidStringValueMaxLengthError("environment variable name", wayTooLongName, 253), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Variables[wayTooLongName] = "Any value"
		}},
		{"conflicting job variable and secret name", radixvalidators.SecretNameConflictsWithEnvironmentVariable("job", conflictingVariableName), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Variables[conflictingVariableName] = "Any value"
			ra.Spec.Jobs[0].Secrets[0] = conflictingVariableName
		}},
		{"non existing env for job", radixvalidators.EnvironmentReferencedByComponentDoesNotExistError(noExistingEnvironment, validRAFirstJobName), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig = []v1.RadixJobComponentEnvironmentConfig{
				{
					Environment: noExistingEnvironment,
				},
			}
		}},
		{"scheduler port is not set", radixvalidators.SchedulerPortCannotBeEmptyForJobError(validRAFirstJobName), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].SchedulerPort = nil
		}},
		{"payload is empty struct", radixvalidators.PayloadPathCannotBeEmptyForJobError(validRAFirstJobName), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Payload = &v1.RadixJobComponentPayload{}
		}},
		{"payload path is empty string", radixvalidators.PayloadPathCannotBeEmptyForJobError(validRAFirstJobName), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Payload = &v1.RadixJobComponentPayload{Path: ""}
		}},

		{"job resource limit unsupported resource", radixvalidators.InvalidResourceError(unsupportedResource), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits[unsupportedResource] = "250m"
		}},
		{"job memory resource limit wrong format", radixvalidators.MemoryResourceRequirementFormatError(invalidResourceValue), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = invalidResourceValue
		}},
		{"job memory resource request wrong format", radixvalidators.MemoryResourceRequirementFormatError(invalidResourceValue), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = invalidResourceValue
		}},
		{"job memory resource request larger than limit", radixvalidators.ResourceRequestOverLimitError("memory", "249Mi", "250Ki"), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "250Ki"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "249Mi"
		}},
		{"job cpu resource limit wrong format", radixvalidators.CPUResourceRequirementFormatError(invalidResourceValue), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["cpu"] = invalidResourceValue
		}},
		{"job cpu resource request wrong format", radixvalidators.CPUResourceRequirementFormatError(invalidResourceValue), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["cpu"] = invalidResourceValue
		}},
		{"job cpu resource request larger than limit", radixvalidators.ResourceRequestOverLimitError("cpu", "251m", "250m"), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["cpu"] = "250m"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["cpu"] = "251m"
		}},
		{"job resource request unsupported resource", radixvalidators.InvalidResourceError(unsupportedResource), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests[unsupportedResource] = "250m"
		}},
		{"job common resource limit unsupported resource", radixvalidators.InvalidResourceError(unsupportedResource), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits[unsupportedResource] = "250m"
		}},
		{"job common resource request unsupported resource", radixvalidators.InvalidResourceError(unsupportedResource), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Requests[unsupportedResource] = "250m"
		}},
		{"job common memory resource limit wrong format", radixvalidators.MemoryResourceRequirementFormatError(invalidResourceValue), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = invalidResourceValue
		}},
		{"job common memory resource request wrong format", radixvalidators.MemoryResourceRequirementFormatError(invalidResourceValue), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Requests["memory"] = invalidResourceValue
		}},
		{"job common cpu resource limit wrong format", radixvalidators.CPUResourceRequirementFormatError(invalidResourceValue), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["cpu"] = invalidResourceValue
		}},
		{"job common cpu resource request wrong format", radixvalidators.CPUResourceRequirementFormatError(invalidResourceValue), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Requests["cpu"] = invalidResourceValue
		}},
		{"job cpu resource limit is empty", nil, func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["cpu"] = ""
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["cpu"] = "251m"
		}},
		{"job cpu resource limit not set", nil, func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["cpu"] = "251m"
		}},
		{"job memory resource limit is empty", nil, func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = ""
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "249Mi"
		}},
		{"job memory resource limit not set", nil, func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "249Mi"
		}},
		{"job wrong public image config", radixvalidators.PublicImageComponentCannotHaveSourceOrDockerfileSet(validRAFirstJobName), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Image = "redis:5.0-alpine"
			ra.Spec.Jobs[0].SourceFolder = "./api"
			ra.Spec.Jobs[0].DockerfileName = ".Dockerfile"
		}},
		{"job missing environment config for dynamic tag", radixvalidators.ComponentWithDynamicTagRequiresTagInEnvironmentConfig(validRAFirstJobName), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Image = "radixcanary.azurecr.io/my-private-image:{imageTagName}"
			ra.Spec.Jobs[0].EnvironmentConfig = []v1.RadixJobComponentEnvironmentConfig{}
		}},
		{"job missing dynamic tag config for mapped environment", radixvalidators.ComponentWithDynamicTagRequiresTagInEnvironmentConfigForEnvironment(validRAFirstJobName, "dev"), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Image = "radixcanary.azurecr.io/my-private-image:{imageTagName}"
			ra.Spec.Jobs[0].EnvironmentConfig[0].ImageTagName = ""
			ra.Spec.Jobs[0].EnvironmentConfig = append(ra.Spec.Jobs[0].EnvironmentConfig, v1.RadixJobComponentEnvironmentConfig{
				Environment:  "dev",
				ImageTagName: "",
			})
		}},
		{"job inconcistent dynamic tag config for environment", radixvalidators.ComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTag(validRAFirstJobName, "dev"), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Image = "radixcanary.azurecr.io/my-private-image:some-tag"
			ra.Spec.Jobs[0].EnvironmentConfig[0].ImageTagName = "any-tag"
		}},
		{"too long app name together with env name", fmt.Errorf("summary length of app name and environment together should not exceed 62 characters"), func(ra *v1.RadixApplication) {
			ra.Name = name50charsLong
			ra.Spec.Environments = append(ra.Spec.Environments, v1.Environment{Name: "extra-14-chars"})
		}},
		{"missing OAuth clientId for dev env - common OAuth config", radixvalidators.OAuthClientIdEmptyError(validRAFirstComponentName, "dev"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].Authentication.OAuth2 = &v1.OAuth2{}
		}},
		{"missing OAuth clientId for prod env - environmentConfig OAuth config", radixvalidators.OAuthClientIdEmptyError(validRAFirstComponentName, "prod"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.ClientID = ""
		}},
		{"OAuth path prefix is root", radixvalidators.OAuthProxyPrefixIsRootError(validRAFirstComponentName, "prod"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.ProxyPrefix = "/"
		}},
		{"invalid OAuth session store type", radixvalidators.OAuthSessionStoreTypeInvalidError(validRAFirstComponentName, "prod", "invalid-store"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SessionStoreType = "invalid-store"
		}},
		{"missing OAuth redisStore property", radixvalidators.OAuthRedisStoreEmptyError(validRAFirstComponentName, "prod"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.RedisStore = nil
		}},
		{"missing OAuth redis connection URL", radixvalidators.OAuthRedisStoreConnectionURLEmptyError(validRAFirstComponentName, "prod"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.RedisStore.ConnectionURL = ""
		}},
		{"no error when skipDiscovery=true and login, redeem and jwks urls set", nil, func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.OIDC = &v1.OAuth2OIDC{
				SkipDiscovery: commonUtils.BoolPtr(true),
				JWKSURL:       "jwksurl",
			}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.LoginURL = "loginurl"
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.RedeemURL = "redeemurl"
		}},
		{"error when skipDiscovery=true and missing loginUrl", radixvalidators.OAuthLoginUrlEmptyError(validRAFirstComponentName, "prod"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.OIDC = &v1.OAuth2OIDC{
				SkipDiscovery: commonUtils.BoolPtr(true),
				JWKSURL:       "jwksurl",
			}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.RedeemURL = "redeemurl"
		}},
		{"error when skipDiscovery=true and missing redeemUrl", radixvalidators.OAuthRedeemUrlEmptyError(validRAFirstComponentName, "prod"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.OIDC = &v1.OAuth2OIDC{
				SkipDiscovery: commonUtils.BoolPtr(true),
				JWKSURL:       "jwksurl",
			}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.LoginURL = "loginurl"
		}},
		{"error when skipDiscovery=true and missing redeemUrl", radixvalidators.OAuthOidcJwksUrlEmptyError(validRAFirstComponentName, "prod"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.OIDC = &v1.OAuth2OIDC{
				SkipDiscovery: commonUtils.BoolPtr(true),
			}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.LoginURL = "loginurl"
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.RedeemURL = "redeemurl"
		}},
		{"valid OAuth configuration for session store cookie and cookieStore.minimal=true", nil, func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SessionStoreType = v1.SessionStoreCookie
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.CookieStore = &v1.OAuth2CookieStore{Minimal: commonUtils.BoolPtr(true)}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetAuthorizationHeader = commonUtils.BoolPtr(false)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetXAuthRequestHeaders = commonUtils.BoolPtr(false)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie = &v1.OAuth2Cookie{
				Expire:  "1h",
				Refresh: "0s",
			}
		}},
		{"error when cookieStore.minimal=true and SetAuthorizationHeader=true", radixvalidators.OAuthCookieStoreMinimalIncorrectSetAuthorizationHeaderError(validRAFirstComponentName, "prod"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SessionStoreType = v1.SessionStoreCookie
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.CookieStore = &v1.OAuth2CookieStore{Minimal: commonUtils.BoolPtr(true)}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetAuthorizationHeader = commonUtils.BoolPtr(true)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetXAuthRequestHeaders = commonUtils.BoolPtr(false)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie = &v1.OAuth2Cookie{
				Expire:  "1h",
				Refresh: "0s",
			}
		}},
		{"error when cookieStore.minimal=true and SetXAuthRequestHeaders=true", radixvalidators.OAuthCookieStoreMinimalIncorrectSetXAuthRequestHeadersError(validRAFirstComponentName, "prod"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SessionStoreType = v1.SessionStoreCookie
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.CookieStore = &v1.OAuth2CookieStore{Minimal: commonUtils.BoolPtr(true)}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetAuthorizationHeader = commonUtils.BoolPtr(false)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetXAuthRequestHeaders = commonUtils.BoolPtr(true)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie = &v1.OAuth2Cookie{
				Expire:  "1h",
				Refresh: "0s",
			}
		}},
		{"error when cookieStore.minimal=true and Cookie.Refresh>0", radixvalidators.OAuthCookieStoreMinimalIncorrectCookieRefreshIntervalError(validRAFirstComponentName, "prod"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SessionStoreType = v1.SessionStoreCookie
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.CookieStore = &v1.OAuth2CookieStore{Minimal: commonUtils.BoolPtr(true)}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetAuthorizationHeader = commonUtils.BoolPtr(false)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetXAuthRequestHeaders = commonUtils.BoolPtr(false)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie = &v1.OAuth2Cookie{
				Expire:  "1h",
				Refresh: "1s",
			}
		}},
		{"invalid OAuth cookie same site", radixvalidators.OAuthCookieSameSiteInvalidError(validRAFirstComponentName, "prod", "invalid-samesite"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.SameSite = "invalid-samesite"
		}},
		{"invalid OAuth cookie expire timeframe", radixvalidators.OAuthCookieExpireInvalidError(validRAFirstComponentName, "prod", "invalid-expire"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Expire = "invalid-expire"
		}},
		{"negative OAuth cookie expire timeframe", radixvalidators.OAuthCookieExpireInvalidError(validRAFirstComponentName, "prod", "-1s"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Expire = "-1s"
		}},
		{"invalid OAuth cookie refresh time frame", radixvalidators.OAuthCookieRefreshInvalidError(validRAFirstComponentName, "prod", "invalid-refresh"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Refresh = "invalid-refresh"
		}},
		{"negative OAuth cookie refresh time frame", radixvalidators.OAuthCookieRefreshInvalidError(validRAFirstComponentName, "prod", "-1s"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Refresh = "-1s"
		}},
		{"oauth cookie expire equals refresh", radixvalidators.OAuthCookieRefreshMustBeLessThanExpireError(validRAFirstComponentName, "prod"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Expire = "1h"
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Refresh = "1h"
		}},
		{"oauth cookie expire less than refresh", radixvalidators.OAuthCookieRefreshMustBeLessThanExpireError(validRAFirstComponentName, "prod"), func(rr *v1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Expire = "30m"
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Refresh = "1h"
		}},
		{"duplicate name in job/component boundary", radixvalidators.DuplicateComponentOrJobNameError([]string{validRAFirstComponentName}), func(ra *v1.RadixApplication) {
			job := *ra.Spec.Jobs[0].DeepCopy()
			job.Name = validRAFirstComponentName
			ra.Spec.Jobs = append(ra.Spec.Jobs, job)
		}},
		{"no mask size postfix in egress rule destination", radixvalidators.DuplicateComponentOrJobNameError([]string{validRAFirstComponentName}), func(ra *v1.RadixApplication) {
			job := *ra.Spec.Jobs[0].DeepCopy()
			job.Name = validRAFirstComponentName
			ra.Spec.Jobs = append(ra.Spec.Jobs, job)
		}},
		{"identity.azure.clientId cannot be empty for component", radixvalidators.ResourceNameCannotBeEmptyError("identity.azure.clientId"), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Identity.Azure.ClientId = " "
		}},
		{"identity.azure.clientId cannot be empty for component environment config", radixvalidators.ResourceNameCannotBeEmptyError("identity.azure.clientId"), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Identity.Azure.ClientId = " "
		}},
		{"identity.azure.clientId cannot be empty for job", radixvalidators.ResourceNameCannotBeEmptyError("identity.azure.clientId"), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Identity.Azure.ClientId = " "
		}},
		{"identity.azure.clientId cannot be empty for job environment config", radixvalidators.ResourceNameCannotBeEmptyError("identity.azure.clientId"), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Identity.Azure.ClientId = " "
		}},

		{"invalid identity.azure.clientId for component", radixvalidators.InvalidUUIDError("identity.azure.clientId", "1111-22-33-44"), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Identity.Azure.ClientId = "1111-22-33-44"
		}},
		{"invalid identity.azure.clientId for component environment config", radixvalidators.InvalidUUIDError("identity.azure.clientId", "1111-22-33-44"), func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Identity.Azure.ClientId = "1111-22-33-44"
		}},
		{"invalid identity.azure.clientId for job", radixvalidators.InvalidUUIDError("identity.azure.clientId", "1111-22-33-44"), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Identity.Azure.ClientId = "1111-22-33-44"
		}},
		{"invalid identity.azure.clientId for job environment config", radixvalidators.InvalidUUIDError("identity.azure.clientId", "1111-22-33-44"), func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Identity.Azure.ClientId = "1111-22-33-44"
		}},
	}

	_, client := validRASetup()
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := createValidRA()
			testcase.updateRA(validRA)
			isValid, errs := radixvalidators.CanRadixApplicationBeInsertedErrors(client, validRA, nil, nil)

			if testcase.expectedError != nil {
				assert.False(t, isValid)
				assert.NotNil(t, errs)
				assert.Truef(t, errors.Contains(errs, testcase.expectedError), fmt.Sprintf("Expected error is not contained in list of errors. \n Expected:\n'%s'\n Actual:\n%s", testcase.expectedError, errors.Concat(errs)))
				return
			}
			assert.True(t, isValid)
			assert.Nil(t, errs)
		})
	}
}

func Test_ValidRAComponentLimitRequest_NoError(t *testing.T) {
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
		{"common resource memory correct format: 50", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50"
		}},
		{"common resource limit correct format: 50T", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50T"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50T"
		}},
		{"common resource limit correct format: 50G", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50G"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50G"
		}},
		{"common resource limit correct format: 50M", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50M"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50M"
		}},
		{"common resource limit correct format: 50k", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50k"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50k"
		}},
		{"common resource limit correct format: 50Gi", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50Gi"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50Gi"
		}},
		{"common resource limit correct format: 50Mi", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50Mi"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50Mi"
		}},
		{"common resource limit correct format: 50Ki", func(ra *v1.RadixApplication) {
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

func Test_ValidRAJobLimitRequest_NoError(t *testing.T) {
	var testScenarios = []struct {
		name     string
		updateRA updateRAFunc
	}{
		{"resource memory correct format: 50", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50"
		}},
		{"resource limit correct format: 50T", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50T"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50T"
		}},
		{"resource limit correct format: 50G", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50G"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50G"
		}},
		{"resource limit correct format: 50M", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50M"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50M"
		}},
		{"resource limit correct format: 50k", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50k"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50k"
		}},
		{"resource limit correct format: 50Gi", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50Gi"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50Gi"
		}},
		{"resource limit correct format: 50Mi", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50Mi"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50Mi"
		}},
		{"resource limit correct format: 50Ki", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50Ki"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50Ki"
		}},
		{"common resource memory correct format: 50", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50"
		}},
		{"common resource limit correct format: 50T", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50T"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50T"
		}},
		{"common resource limit correct format: 50G", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50G"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50G"
		}},
		{"common resource limit correct format: 50M", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50M"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50M"
		}},
		{"common resource limit correct format: 50k", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50k"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50k"
		}},
		{"common resource limit correct format: 50Gi", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50Gi"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50Gi"
		}},
		{"common resource limit correct format: 50Mi", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50Mi"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50Mi"
		}},
		{"common resource limit correct format: 50Ki", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50Ki"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50Ki"
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

func Test_InvalidRAComponentLimitRequest_Error(t *testing.T) {
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
		{"common resource limit incorrect format: 50MB", func(ra *v1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50MB"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50MB"
		}},
		{"common resource limit incorrect format: 50K", func(ra *v1.RadixApplication) {
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

func Test_InvalidRAJobLimitRequest_Error(t *testing.T) {
	var testScenarios = []struct {
		name     string
		updateRA updateRAFunc
	}{
		{"resource limit incorrect format: 50MB", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50MB"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50MB"
		}},
		{"resource limit incorrect format: 50K", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50K"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50K"
		}},
		{"common resource limit incorrect format: 50MB", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50MB"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50MB"
		}},
		{"common resource limit incorrect format: 50K", func(ra *v1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50K"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50K"
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
		{
			name: "privileged port used in radixConfig",
			updateRA: func(ra *v1.RadixApplication) {
				newPorts := []v1.ComponentPort{
					{
						Name: "http",
						Port: 1000,
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

func Test_Variables(t *testing.T) {
	var testScenarios = []struct {
		name       string
		updateRA   updateRAFunc
		isValid    bool
		isErrorNil bool
	}{
		{
			name: "check that user defined variable with legal prefix succeeds",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Components[1].Variables["RADIX"] = "any value"
			},
			isValid:    true,
			isErrorNil: true,
		},
		{
			name: "check that user defined variable with legal prefix succeeds",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Components[1].Variables["RADIXX_SOMETHING"] = "any value"
			},
			isValid:    true,
			isErrorNil: true,
		},
		{
			name: "check that user defined variable with legal prefix succeeds",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Components[1].Variables["SOMETHING_RADIX_SOMETHING"] = "any value"
			},
			isValid:    true,
			isErrorNil: true,
		},
		{
			name: "check that user defined variable with legal prefix succeeds",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Components[1].Variables["S_RADIXOPERATOR"] = "any value"
			},
			isValid:    true,
			isErrorNil: true,
		},
		{
			name: "check that user defined variable with illegal prefix fails",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Components[1].Variables["RADIX_SOMETHING"] = "any value"
			},
			isValid:    false,
			isErrorNil: false,
		},
		{
			name: "check that user defined variable with illegal prefix fails",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Components[1].Variables["RADIXOPERATOR_SOMETHING"] = "any value"
			},
			isValid:    false,
			isErrorNil: false,
		},
		{
			name: "check that user defined variable with illegal prefix fails",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Components[1].Variables["RADIXOPERATOR_"] = "any value"
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

func Test_ValidationOfVolumeMounts_Errors(t *testing.T) {
	type volumeMountsFunc func() []v1.RadixVolumeMount
	type setVolumeMountsFunc func(*v1.RadixApplication, []v1.RadixVolumeMount)

	setRaComponentVolumeMounts := func(ra *v1.RadixApplication, volumeMounts []v1.RadixVolumeMount) {
		ra.Spec.Components[0].EnvironmentConfig[0].VolumeMounts = volumeMounts
	}

	setRaJobsVolumeMounts := func(ra *v1.RadixApplication, volumeMounts []v1.RadixVolumeMount) {
		ra.Spec.Jobs[0].EnvironmentConfig[0].VolumeMounts = volumeMounts
	}

	setComponentAndJobsVolumeMounts := []setVolumeMountsFunc{setRaComponentVolumeMounts, setRaJobsVolumeMounts}

	var testScenarios = []struct {
		name                 string
		volumeMounts         volumeMountsFunc
		updateRA             []setVolumeMountsFunc
		isValid              bool
		isErrorNil           bool
		testContainedByError string
	}{
		{
			"incorrect mount type",
			func() []v1.RadixVolumeMount {
				volumeMounts := []v1.RadixVolumeMount{
					{
						Type:    "new-type",
						Name:    "some_name",
						Storage: "some_container_name",
						Path:    "some_path",
					},
				}

				return volumeMounts
			},
			setComponentAndJobsVolumeMounts,
			false,
			false,
			"not recognized volume mount type",
		},
		{
			"blob mount type with different name, containers and path",
			func() []v1.RadixVolumeMount {
				volumeMounts := []v1.RadixVolumeMount{
					{
						Type:      v1.MountTypeBlob,
						Name:      "some_name_1",
						Container: "some_container_name_1",
						Path:      "some_path_1",
					},
					{
						Type:      v1.MountTypeBlob,
						Name:      "some_name_2",
						Container: "some_container_name_2",
						Path:      "some_path_2",
					},
				}

				return volumeMounts
			},
			setComponentAndJobsVolumeMounts,
			true,
			true,
			"",
		},
		{
			"blob mount type with duplicate names",
			func() []v1.RadixVolumeMount {
				volumeMounts := []v1.RadixVolumeMount{
					{
						Type:      v1.MountTypeBlob,
						Name:      "some_name",
						Container: "some_container_name_1",
						Path:      "some_path_1",
					},
					{
						Type:      v1.MountTypeBlob,
						Name:      "some_name",
						Container: "some_container_name_2",
						Path:      "some_path_2",
					},
				}

				return volumeMounts
			},
			setComponentAndJobsVolumeMounts,
			false,
			false,
			"duplicate names",
		},
		{
			"blob mount type with duplicate path",
			func() []v1.RadixVolumeMount {
				volumeMounts := []v1.RadixVolumeMount{
					{
						Type:      v1.MountTypeBlob,
						Name:      "some_name_1",
						Container: "some_container_name_1",
						Path:      "some_path",
					},
					{
						Type:      v1.MountTypeBlob,
						Name:      "some_name_2",
						Container: "some_container_name_2",
						Path:      "some_path",
					},
				}

				return volumeMounts
			},
			setComponentAndJobsVolumeMounts,
			false,
			false,
			"duplicate path",
		},
		{
			"mount volume type is not set",
			func() []v1.RadixVolumeMount {
				volumeMounts := []v1.RadixVolumeMount{
					{
						Name:      "some_name",
						Container: "some_container_name",
						Path:      "some_path",
					},
				}

				return volumeMounts
			},
			setComponentAndJobsVolumeMounts,
			false,
			false,
			"non of volume mount type, blobfuse2 or azureFile options are defined in the volumeMount for component",
		},
		{
			"mount volume name is not set",
			func() []v1.RadixVolumeMount {
				volumeMounts := []v1.RadixVolumeMount{
					{
						Type:      v1.MountTypeBlob,
						Container: "some_container_name",
						Path:      "some_path",
					},
				}

				return volumeMounts
			},
			setComponentAndJobsVolumeMounts,
			false,
			false,
			"missing volume mount name and path of volumeMount for component",
		},
		{
			"mount volume containers is not set",
			func() []v1.RadixVolumeMount {
				volumeMounts := []v1.RadixVolumeMount{
					{
						Type: v1.MountTypeBlob,
						Name: "some_name",
						Path: "some_path",
					},
				}

				return volumeMounts
			},
			setComponentAndJobsVolumeMounts,
			false,
			false,
			"missing volume mount storage of volumeMount for component",
		},
		{
			"mount volume path is not set",
			func() []v1.RadixVolumeMount {
				volumeMounts := []v1.RadixVolumeMount{
					{
						Type:      v1.MountTypeBlob,
						Name:      "some_name",
						Container: "some_container_name",
					},
				}

				return volumeMounts
			},
			setComponentAndJobsVolumeMounts,
			false,
			false,
			"missing volume mount name and path of volumeMount for component",
		},
		{
			"mount volume is blobfuse2 fuse2",
			func() []v1.RadixVolumeMount {
				volumeMounts := []v1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &v1.RadixBlobFuse2VolumeMount{
							Protocol:  v1.BlobFuse2ProtocolFuse2,
							Container: "some-container",
						},
					},
				}
				return volumeMounts
			},
			setComponentAndJobsVolumeMounts, true, true, "",
		},
		{
			"mount volume is blobfuse2 nfs",
			func() []v1.RadixVolumeMount {
				volumeMounts := []v1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &v1.RadixBlobFuse2VolumeMount{
							Protocol:  v1.BlobFuse2ProtocolNfs,
							Container: "some-container",
						},
					},
				}
				return volumeMounts
			},
			setComponentAndJobsVolumeMounts, true, true, "",
		},
		{
			"missing container name in mount volume blobfuse2 fuse2",
			func() []v1.RadixVolumeMount {
				volumeMounts := []v1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &v1.RadixBlobFuse2VolumeMount{
							Protocol: v1.BlobFuse2ProtocolFuse2,
						},
					},
				}
				return volumeMounts
			},
			setComponentAndJobsVolumeMounts, false, false, "missing BlobFuse2 volume mount container of volumeMount for component",
		},
		{
			"missing container name in mount volume blobfuse2 nfs",
			func() []v1.RadixVolumeMount {
				volumeMounts := []v1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &v1.RadixBlobFuse2VolumeMount{
							Protocol: v1.BlobFuse2ProtocolNfs,
						},
					},
				}
				return volumeMounts
			},
			setComponentAndJobsVolumeMounts, false, false, "missing BlobFuse2 volume mount container of volumeMount for component",
		},
		{
			"default empty protocol is fuse2 in mount volume blobfuse2 fuse2",
			func() []v1.RadixVolumeMount {
				volumeMounts := []v1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &v1.RadixBlobFuse2VolumeMount{
							Container: "some-container",
						},
					},
				}
				return volumeMounts
			},
			setComponentAndJobsVolumeMounts, true, true, "",
		},
		{
			"failed unsupported protocol in mount volume blobfuse2 fuse2",
			func() []v1.RadixVolumeMount {
				volumeMounts := []v1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &v1.RadixBlobFuse2VolumeMount{
							Container: "some-container",
							Protocol:  "some-text",
						},
					},
				}
				return volumeMounts
			},
			setComponentAndJobsVolumeMounts, false, false, "unsupported BlobFuse2 volume mount protocol of volumeMount for component",
		},
	}

	_, client := validRASetup()
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			if testcase.updateRA == nil || len(testcase.updateRA) == 0 {
				assert.FailNow(t, "missing updateRA functions for %s", testcase.name)
				return
			}

			for _, ra := range testcase.updateRA {
				validRA := createValidRA()
				volumes := testcase.volumeMounts()
				ra(validRA, volumes)
				isValid, err := radixvalidators.CanRadixApplicationBeInserted(client, validRA)
				isErrorNil := false
				if err == nil {
					isErrorNil = true
				}

				assert.Equal(t, testcase.isValid, isValid)
				assert.Equal(t, testcase.isErrorNil, isErrorNil)
				if !isErrorNil {
					assert.Contains(t, err.Error(), testcase.testContainedByError)
					continue
				}
			}
		})
	}
}

func Test_ValidHPA_NoError(t *testing.T) {
	var testScenarios = []struct {
		name       string
		updateRA   updateRAFunc
		isValid    bool
		isErrorNil bool
	}{
		{
			"horizontalScaling is not set",
			func(ra *v1.RadixApplication) {},
			true,
			true,
		},
		{
			"minReplicas and maxReplicas are not set",
			func(ra *v1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &v1.RadixHorizontalScaling{}
			},
			false,
			false,
		},
		{
			"maxReplicas is not set and minReplicas is set",
			func(ra *v1.RadixApplication) {
				minReplica := int32(3)
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &v1.RadixHorizontalScaling{}
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling.MinReplicas = &minReplica
			},
			false,
			false,
		},
		{
			"minReplicas is not set and maxReplicas is set",
			func(ra *v1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &v1.RadixHorizontalScaling{}
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling.MaxReplicas = 2
			},
			true,
			true,
		},
		{
			"minReplicas is greater than maxReplicas",
			func(ra *v1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &v1.RadixHorizontalScaling{}
				minReplica := int32(3)
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling.MinReplicas = &minReplica
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling.MaxReplicas = 2
			},
			false,
			false,
		},
		{
			"maxReplicas is greater than minReplicas",
			func(ra *v1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &v1.RadixHorizontalScaling{}
				minReplica := int32(3)
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling.MinReplicas = &minReplica
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling.MaxReplicas = 4
			},
			true,
			true,
		},
		{
			"custom resource scaling for HPA is set, but no resource thresholds are defined",
			func(ra *v1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &v1.RadixHorizontalScaling{}
				minReplica := int32(2)
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling.MinReplicas = &minReplica
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling.MaxReplicas = 4
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling.RadixHorizontalScalingResources = &v1.RadixHorizontalScalingResources{}
			},
			false,
			false,
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

func Test_EgressConfig(t *testing.T) {
	var testScenarios = []struct {
		name     string
		updateRA updateRAFunc
		isValid  bool
	}{
		{
			name: "egress rule must have valid destination masks",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []v1.EgressRule{{
					Destinations: []v1.EgressDestination{"notanIPmask"},
					Ports:        nil,
				}}
			},
			isValid: false,
		},
		{
			name: "egress rule must use IPv4 in destination CIDR, zero ports",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []v1.EgressRule{{
					Destinations: []v1.EgressDestination{"2001:0DB8:0000:000b::/64"},
					Ports:        nil,
				}}
			},
			isValid: false,
		},
		{
			name: "egress rule must use IPv4 in destination CIDR",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []v1.EgressRule{{
					Destinations: []v1.EgressDestination{"2001:0DB8:0000:000b::/64"},
					Ports: []v1.EgressPort{{
						Port:     10,
						Protocol: "TCP",
					}},
				}}
			},
			isValid: false,
		},
		{
			name: "egress rule must have postfix in IPv4 CIDR",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []v1.EgressRule{{
					Destinations: []v1.EgressDestination{"10.0.0.1"},
					Ports:        nil,
				}}
			},
			isValid: false,
		},
		{
			name: "egress rule must have valid ports",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []v1.EgressRule{{
					Destinations: []v1.EgressDestination{"10.0.0.1"},
					Ports: []v1.EgressPort{{
						Port:     0,
						Protocol: "TCP",
					}},
				}}
			},
			isValid: false,
		},
		{
			name: "egress rule must have valid ports",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []v1.EgressRule{{
					Destinations: []v1.EgressDestination{"10.0.0.1"},
					Ports: []v1.EgressPort{{
						Port:     66000,
						Protocol: "TCP",
					}},
				}}
			},
			isValid: false,
		},
		{
			name: "egress rule must contain destination",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []v1.EgressRule{{
					Destinations: nil,
					Ports: []v1.EgressPort{{
						Port:     24,
						Protocol: "TCP",
					}},
				}}
			},
			isValid: false,
		},
		{
			name: "egress rule must have valid port protocol",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []v1.EgressRule{{
					Destinations: []v1.EgressDestination{"10.0.0.1/32"},
					Ports: []v1.EgressPort{{
						Port:     2000,
						Protocol: "SCTP",
					}},
				}}
			},
			isValid: false,
		},
		{
			name: "egress rule must have valid port protocol",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []v1.EgressRule{{
					Destinations: []v1.EgressDestination{"10.0.0.1/32"},
					Ports: []v1.EgressPort{{
						Port:     2000,
						Protocol: "erwef",
					}},
				}}
			},
			isValid: false,
		},
		{
			name: "can not exceed max nr of egress rules",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []v1.EgressRule{}
				for i := 0; i <= 1000; i++ {
					ra.Spec.Environments[0].Egress.Rules = append(ra.Spec.Environments[0].Egress.Rules, v1.EgressRule{
						Destinations: []v1.EgressDestination{"10.0.0.0/8"},
						Ports:        nil,
					})
				}
			},
			isValid: false,
		},
		{
			name: "sample egress rule with valid destination, zero ports",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []v1.EgressRule{{
					Destinations: []v1.EgressDestination{"10.0.0.0/8"},
					Ports:        nil,
				}}
			},
			isValid: true,
		},
		{
			name: "sample egress rule with valid destinations",
			updateRA: func(ra *v1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []v1.EgressRule{{
					Destinations: []v1.EgressDestination{"10.0.0.0/8", "192.10.10.10/32"},
					Ports: []v1.EgressPort{
						{
							Port:     53,
							Protocol: "udp",
						},
						{
							Port:     53,
							Protocol: "TCP",
						},
					},
				}}
			},
			isValid: true,
		},
	}

	_, client := validRASetup()
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := createValidRA()
			testcase.updateRA(validRA)
			isValid, _ := radixvalidators.CanRadixApplicationBeInserted(client, validRA)
			assert.Equal(t, testcase.isValid, isValid)
		})
	}
}

func Test_validateNotificationsRA(t *testing.T) {
	invalidUrl := string([]byte{1, 2, 3, 0x7f, 0})
	var testScenarios = []struct {
		name          string
		expectedError error
		updateRa      updateRAFunc
	}{
		{name: "No notification", expectedError: nil,
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = nil
			},
		},
		{name: "valid webhook with http", expectedError: nil,
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &v1.Notifications{Webhook: pointers.Ptr("http://api:8090/abc")}
			},
		},
		{name: "valid webhook with https", expectedError: nil,
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &v1.Notifications{Webhook: pointers.Ptr("https://api:8090/abc")}
			},
		},
		{name: "valid webhook in environment", expectedError: nil,
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &v1.Notifications{Webhook: pointers.Ptr("http://api:8090/abc")}
			},
		},
		{name: "valid webhook to job component", expectedError: nil,
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &v1.Notifications{Webhook: pointers.Ptr("http://job3:8099/abc")}
			},
		},
		{name: "valid webhook to job component in environment", expectedError: nil,
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &v1.Notifications{Webhook: pointers.Ptr("http://job3:8099/abc")}
			},
		},
		{name: "Invalid webhook URL", expectedError: radixvalidators.InvalidWebhookUrl("job", ""),
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &v1.Notifications{Webhook: &invalidUrl}
			},
		},
		{name: "Invalid webhook URL in environment", expectedError: radixvalidators.InvalidWebhookUrl("job", "dev"),
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &v1.Notifications{Webhook: &invalidUrl}
			},
		},
		{name: "Not allowed scheme ftp", expectedError: radixvalidators.NotAllowedSchemeInWebhookUrl("ftp", "job", ""),
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &v1.Notifications{Webhook: pointers.Ptr("ftp://api:8090")}
			},
		},
		{name: "Not allowed scheme ftp in environment", expectedError: radixvalidators.NotAllowedSchemeInWebhookUrl("ftp", "job", "dev"),
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &v1.Notifications{Webhook: pointers.Ptr("ftp://api:8090")}
			},
		},
		{name: "missing port in the webhook", expectedError: radixvalidators.MissingPortInWebhookUrl("job", ""),
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &v1.Notifications{Webhook: pointers.Ptr("http://api/abc")}
			},
		},
		{name: "missing port in the webhook in environment", expectedError: radixvalidators.MissingPortInWebhookUrl("job", "dev"),
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &v1.Notifications{Webhook: pointers.Ptr("http://api/abc")}
			},
		},
		{name: "webhook can only reference to an application component or job", expectedError: radixvalidators.OnlyAppComponentAllowedInWebhookUrl("job", ""),
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &v1.Notifications{Webhook: pointers.Ptr("http://notexistingcomponent:8090/abc")}
			},
		},
		{name: "webhook can only reference to an application component or job in environment", expectedError: radixvalidators.OnlyAppComponentAllowedInWebhookUrl("job", "dev"),
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &v1.Notifications{Webhook: pointers.Ptr("http://notexistingcomponent:8090/abc")}
			},
		},
		{name: "webhook port does not exist in an application component", expectedError: radixvalidators.InvalidPortInWebhookUrl("8077", "api", "job", ""),
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &v1.Notifications{Webhook: pointers.Ptr("http://api:8077")}
			},
		},
		{name: "webhook port does not exist in an application component in environment", expectedError: radixvalidators.InvalidPortInWebhookUrl("8077", "api", "job", "dev"),
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &v1.Notifications{Webhook: pointers.Ptr("http://api:8077")}
			},
		},
		{name: "webhook port does not exist in an application job component", expectedError: radixvalidators.InvalidPortInWebhookUrl("8077", "job3", "job", ""),
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &v1.Notifications{Webhook: pointers.Ptr("http://job3:8077")}
			},
		},
		{name: "webhook port does not exist in an application job component in environment", expectedError: radixvalidators.InvalidPortInWebhookUrl("8077", "job3", "job", "dev"),
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &v1.Notifications{Webhook: pointers.Ptr("http://job3:8077")}
			},
		},
		{name: "not allowed to use in the webhook a public port of an application component", expectedError: radixvalidators.InvalidUseOfPublicPortInWebhookUrl("8080", "app", "job", ""),
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &v1.Notifications{Webhook: pointers.Ptr("http://app:8080")}
			},
		},
		{name: "not allowed to use in the webhook a public port of an application component in environment", expectedError: radixvalidators.InvalidUseOfPublicPortInWebhookUrl("8080", "app", "job", "dev"),
			updateRa: func(ra *v1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &v1.Notifications{Webhook: pointers.Ptr("http://app:8080")}
			},
		},
	}

	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			ra := createValidRA()
			testcase.updateRa(ra)

			err := radixvalidators.ValidateNotificationsForRA(ra)

			if testcase.expectedError == nil && err != nil {
				assert.Fail(t, "Not expected error %v", err)
				return
			}
			if err != nil {
				assert.NotNil(t, err)
				assert.True(t, testcase.expectedError.Error() == err.Error())
				return
			}
			assert.Nil(t, testcase.expectedError)
			assert.Nil(t, err)
		})
	}
}

func createValidRA() *v1.RadixApplication {
	validRA, _ := utils.GetRadixApplicationFromFile("testdata/radixconfig.yaml")

	return validRA
}

func validRASetup() (kubernetes.Interface, radixclient.Interface) {
	validRR, _ := utils.GetRadixRegistrationFromFile("testdata/radixregistration.yaml")
	kubeclient := kubefake.NewSimpleClientset()
	client := radixfake.NewSimpleClientset(validRR)

	return kubeclient, client
}
