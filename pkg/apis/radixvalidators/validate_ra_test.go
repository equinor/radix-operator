package radixvalidators_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	dnsaliasconfig "github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

type updateRAFunc func(rr *radixv1.RadixApplication)

func Test_valid_ra_returns_true(t *testing.T) {
	_, client := validRASetup()
	validRA := createValidRA()
	err := radixvalidators.CanRadixApplicationBeInserted(context.Background(), client, validRA, getDNSAliasConfig())

	assert.NoError(t, err)
}

func Test_missing_rr(t *testing.T) {
	client := radixfake.NewSimpleClientset()
	validRA := createValidRA()

	err := radixvalidators.CanRadixApplicationBeInserted(context.Background(), client, validRA, getDNSAliasConfig())

	assert.Error(t, err)
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
		{"Mixed case name", radixvalidators.ApplicationNameNotLowercaseErrorWithMessage(mixedCaseName), func(ra *radixv1.RadixApplication) { ra.Name = mixedCaseName }},
		{"Lower case name", radixvalidators.ApplicationNameNotLowercaseErrorWithMessage(lowerCaseName), func(ra *radixv1.RadixApplication) { ra.Name = lowerCaseName }},
		{"Upper case name", radixvalidators.ApplicationNameNotLowercaseErrorWithMessage(upperCaseName), func(ra *radixv1.RadixApplication) { ra.Name = upperCaseName }},
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
	invalidCertificateVerification := radixv1.VerificationType("obviously_an_invalid_value")
	name50charsLong := "a123456789a123456789a123456789a123456789a123456789"

	var testScenarios = []struct {
		name          string
		expectedError error
		updateRA      updateRAFunc
	}{
		{"no error", nil, func(ra *radixv1.RadixApplication) {}},
		{"too long app name", radixvalidators.InvalidAppNameLengthErrorWithMessage(wayTooLongName), func(ra *radixv1.RadixApplication) {
			ra.Name = wayTooLongName
		}},
		{"invalid app name", radixvalidators.InvalidLowerCaseAlphaNumericDashResourceNameErrorWithMessage("app name", invalidResourceName), func(ra *radixv1.RadixApplication) {
			ra.Name = invalidResourceName
		}},
		{"empty name", radixvalidators.ResourceNameCannotBeEmptyErrorWithMessage("app name"), func(ra *radixv1.RadixApplication) {
			ra.Name = ""
		}},
		{"no related rr", radixvalidators.NoRegistrationExistsForApplicationErrorWithMessage(noReleatedRRAppName), func(ra *radixv1.RadixApplication) {
			ra.Name = noReleatedRRAppName
		}},
		{"non existing env for component", radixvalidators.EnvironmentReferencedByComponentDoesNotExistErrorWithMessage(noExistingEnvironment, validRAFirstComponentName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig = []radixv1.RadixEnvironmentConfig{
				{
					Environment: noExistingEnvironment,
				},
			}
		}},
		{"invalid component name", radixvalidators.InvalidLowerCaseAlphaNumericDashResourceNameErrorWithMessage("component name", invalidResourceName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Name = invalidResourceName
		}},
		{"uppercase component name", radixvalidators.InvalidLowerCaseAlphaNumericDashResourceNameErrorWithMessage("component name", invalidUpperCaseResourceName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Name = invalidUpperCaseResourceName
		}},
		{"duplicate component name", radixvalidators.DuplicateComponentOrJobNameErrorWithMessage([]string{validRAFirstComponentName}), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components = append(ra.Spec.Components, *ra.Spec.Components[0].DeepCopy())
		}},
		{"component name with oauth auxiliary name suffix", radixvalidators.ComponentNameReservedSuffixErrorWithMessage(oauthAuxSuffixComponentName, "component", defaults.OAuthProxyAuxiliaryComponentSuffix), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Name = oauthAuxSuffixComponentName
		}},
		{"invalid port name", radixvalidators.InvalidLowerCaseAlphaNumericDashResourceNameErrorWithMessage("port name", invalidResourceName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Ports[0].Name = invalidResourceName
		}},
		{"too long port name", radixvalidators.InvalidPortNameLengthErrorWithMessage(tooLongPortName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].PublicPort = tooLongPortName
			ra.Spec.Components[0].Ports[0].Name = tooLongPortName
		}},
		{"invalid build secret name", radixvalidators.InvalidResourceNameErrorWithMessage("build secret name", invalidVariableName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Build = &radixv1.BuildSpec{
				Secrets: []string{invalidVariableName},
			}
		}},
		{"too long build secret name", radixvalidators.InvalidStringValueMaxLengthErrorWithMessage("build secret name", wayTooLongName, 253), func(ra *radixv1.RadixApplication) {
			ra.Spec.Build = &radixv1.BuildSpec{
				Secrets: []string{wayTooLongName},
			}
		}},
		{"invalid secret name", radixvalidators.InvalidResourceNameErrorWithMessage("secret name", invalidVariableName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[1].Secrets[0] = invalidVariableName
		}},
		{"too long secret name", radixvalidators.InvalidStringValueMaxLengthErrorWithMessage("secret name", wayTooLongName, 253), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[1].Secrets[0] = wayTooLongName
		}},
		{"invalid environment variable name", radixvalidators.InvalidResourceNameErrorWithMessage("environment variable name", invalidVariableName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[1].EnvironmentConfig[0].Variables[invalidVariableName] = "Any value"
		}},
		{"too long environment variable name", radixvalidators.InvalidStringValueMaxLengthErrorWithMessage("environment variable name", wayTooLongName, 253), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[1].EnvironmentConfig[0].Variables[wayTooLongName] = "Any value"
		}},
		{"conflicting variable and secret name", radixvalidators.SecretNameConflictsWithEnvironmentVariableWithMessage(validRASecondComponentName, conflictingVariableName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[1].EnvironmentConfig[0].Variables[conflictingVariableName] = "Any value"
			ra.Spec.Components[1].Secrets[0] = conflictingVariableName
		}},
		{"invalid common environment variable name", radixvalidators.InvalidResourceNameErrorWithMessage("environment variable name", invalidVariableName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[1].Variables[invalidVariableName] = "Any value"
		}},
		{"too long common environment variable name", radixvalidators.InvalidStringValueMaxLengthErrorWithMessage("environment variable name", wayTooLongName, 253), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[1].Variables[wayTooLongName] = "Any value"
		}},
		{"conflicting common variable and secret name", radixvalidators.SecretNameConflictsWithEnvironmentVariableWithMessage(validRASecondComponentName, conflictingVariableName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[1].Variables[conflictingVariableName] = "Any value"
			ra.Spec.Components[1].Secrets[0] = conflictingVariableName
		}},
		{"conflicting common variable and secret name when not environment config", radixvalidators.SecretNameConflictsWithEnvironmentVariableWithMessage(validRASecondComponentName, conflictingVariableName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[1].Variables[conflictingVariableName] = "Any value"
			ra.Spec.Components[1].Secrets[0] = conflictingVariableName
			ra.Spec.Components[1].EnvironmentConfig = nil
		}},
		{"invalid number of replicas", radixvalidators.InvalidNumberOfReplicaError(radixvalidators.MaxReplica + 1), func(ra *radixv1.RadixApplication) {
			*ra.Spec.Components[0].EnvironmentConfig[0].Replicas = radixvalidators.MaxReplica + 1
		}},
		{"invalid env name", radixvalidators.InvalidLowerCaseAlphaNumericDashResourceNameErrorWithMessage("env name", invalidResourceName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Environments[0].Name = invalidResourceName
		}},
		{"invalid branch name", radixvalidators.InvalidBranchNameErrorWithMessage(invalidBranchName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Environments[0].Build.From = invalidBranchName
		}},
		{"too long branch name", radixvalidators.InvalidStringValueMaxLengthErrorWithMessage("branch from", wayTooLongName, 253), func(ra *radixv1.RadixApplication) {
			ra.Spec.Environments[0].Build.From = wayTooLongName
		}},
		{"dns app alias non existing component", radixvalidators.ComponentForDNSAppAliasNotDefinedError(nonExistingComponent), func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAppAlias.Component = nonExistingComponent
		}},
		{"dns app alias non existing env", radixvalidators.EnvForDNSAppAliasNotDefinedErrorWithMessage(noExistingEnvironment), func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAppAlias.Environment = noExistingEnvironment
		}},
		{"dns alias is empty", radixvalidators.ResourceNameCannotBeEmptyErrorWithMessage("dnsAlias component"), func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAlias[0].Component = ""
		}},
		{"dns alias is empty", radixvalidators.ResourceNameCannotBeEmptyErrorWithMessage("dnsAlias environment"), func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAlias[0].Environment = ""
		}},
		{"dns alias is invalid", radixvalidators.InvalidLowerCaseAlphaNumericDashResourceNameErrorWithMessage("dnsAlias component", "component.abc"), func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAlias[0].Component = "component.abc"
		}},
		{"dns alias is invalid", radixvalidators.InvalidLowerCaseAlphaNumericDashResourceNameErrorWithMessage("dnsAlias environment", "environment.abc"), func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAlias[0].Environment = "environment.abc"
		}},
		{"dns alias non existing component", radixvalidators.ComponentForDNSAliasNotDefinedError(nonExistingComponent), func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAlias[0].Component = nonExistingComponent
		}},
		{"dns alias non existing env", radixvalidators.EnvForDNSAliasNotDefinedError(noExistingEnvironment), func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAlias[0].Environment = noExistingEnvironment
		}},
		{"dns alias alias is empty", radixvalidators.ResourceNameCannotBeEmptyErrorWithMessage("dnsAlias alias"), func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAlias[0].Alias = ""
		}},
		{"dns alias alias is invalid", radixvalidators.InvalidLowerCaseAlphaNumericDashResourceNameErrorWithMessage("dnsAlias alias", "my.alias"), func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAlias[0].Alias = "my.alias"
		}},
		{"dns alias alias is invalid", radixvalidators.DuplicateAliasForDNSAliasError("my-alias"), func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAlias = append(ra.Spec.DNSAlias, ra.Spec.DNSAlias[0])
		}},
		{"dns alias with no public port", radixvalidators.ComponentForDNSAliasIsNotMarkedAsPublicError(validRAComponentNameApp2), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[3].PublicPort = ""
			ra.Spec.Components[3].Public = false
			ra.Spec.DNSAlias[0] = radixv1.DNSAlias{
				Alias:       "my-alias",
				Component:   ra.Spec.Components[3].Name,
				Environment: ra.Spec.Environments[0].Name,
			}
		}},
		{"dns alias non existing component", radixvalidators.ComponentForDNSAppAliasNotDefinedError(nonExistingComponent), func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAppAlias.Component = nonExistingComponent
		}},
		{"dns alias non existing env", radixvalidators.EnvForDNSAppAliasNotDefinedErrorWithMessage(noExistingEnvironment), func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAppAlias.Environment = noExistingEnvironment
		}},
		{"dns external alias non existing component", radixvalidators.ComponentForDNSExternalAliasNotDefinedErrorWithMessage(nonExistingComponent), func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSExternalAlias = []radixv1.ExternalAlias{
				{
					Alias:       "some.alias.com",
					Component:   nonExistingComponent,
					Environment: ra.Spec.Environments[0].Name,
				},
			}
		}},
		{"dns external alias non existing environment", radixvalidators.EnvForDNSExternalAliasNotDefinedErrorWithMessage(noExistingEnvironment), func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSExternalAlias = []radixv1.ExternalAlias{
				{
					Alias:       "some.alias.com",
					Component:   ra.Spec.Components[0].Name,
					Environment: noExistingEnvironment,
				},
			}
		}},
		{"dns external alias non existing alias", radixvalidators.ErrExternalAliasCannotBeEmpty, func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSExternalAlias = []radixv1.ExternalAlias{
				{
					Component:   ra.Spec.Components[0].Name,
					Environment: ra.Spec.Environments[0].Name,
				},
			}
		}},
		{"dns external alias with no public port", radixvalidators.ComponentForDNSExternalAliasIsNotMarkedAsPublicErrorWithMessage(validRAFirstComponentName), func(ra *radixv1.RadixApplication) {
			// Backward compatible setting
			ra.Spec.Components[0].Public = false
			ra.Spec.Components[0].PublicPort = ""
			ra.Spec.DNSExternalAlias = []radixv1.ExternalAlias{
				{
					Alias:       "some.alias.com",
					Component:   ra.Spec.Components[0].Name,
					Environment: ra.Spec.Environments[0].Name,
				},
			}
		}},
		{"duplicate dns external alias", radixvalidators.DuplicateExternalAliasErrorWithMessage(), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Public = true
			ra.Spec.DNSExternalAlias = []radixv1.ExternalAlias{
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
		{"resource limit unsupported resource", radixvalidators.InvalidResourceErrorWithMessage(unsupportedResource), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits[unsupportedResource] = "250m"
		}},
		{"memory resource limit wrong format", radixvalidators.MemoryResourceRequirementFormatErrorWithMessage(invalidResourceValue), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = invalidResourceValue
		}},
		{"memory resource request wrong format", radixvalidators.MemoryResourceRequirementFormatErrorWithMessage(invalidResourceValue), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = invalidResourceValue
		}},
		{"memory resource request larger than limit", radixvalidators.ResourceRequestOverLimitErrorWithMessage("memory", "249Mi", "250Ki"), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "250Ki"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "249Mi"
		}},
		{"cpu resource limit wrong format", radixvalidators.CPUResourceRequirementFormatErrorWithMessage(invalidResourceValue), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["cpu"] = invalidResourceValue
		}},
		{"cpu resource request wrong format", radixvalidators.CPUResourceRequirementFormatErrorWithMessage(invalidResourceValue), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["cpu"] = invalidResourceValue
		}},
		{"cpu resource request larger than limit", radixvalidators.ResourceRequestOverLimitErrorWithMessage("cpu", "251m", "250m"), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["cpu"] = "250m"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["cpu"] = "251m"
		}},
		{"resource request unsupported resource", radixvalidators.InvalidResourceErrorWithMessage(unsupportedResource), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests[unsupportedResource] = "250m"
		}},
		{"common resource limit unsupported resource", radixvalidators.InvalidResourceErrorWithMessage(unsupportedResource), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits[unsupportedResource] = "250m"
		}},
		{"common resource request unsupported resource", radixvalidators.InvalidResourceErrorWithMessage(unsupportedResource), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Requests[unsupportedResource] = "250m"
		}},
		{"common memory resource limit wrong format", radixvalidators.MemoryResourceRequirementFormatErrorWithMessage(invalidResourceValue), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = invalidResourceValue
		}},
		{"common memory resource request wrong format", radixvalidators.MemoryResourceRequirementFormatErrorWithMessage(invalidResourceValue), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Requests["memory"] = invalidResourceValue
		}},
		{"common cpu resource limit wrong format", radixvalidators.CPUResourceRequirementFormatErrorWithMessage(invalidResourceValue), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["cpu"] = invalidResourceValue
		}},
		{"common cpu resource request wrong format", radixvalidators.CPUResourceRequirementFormatErrorWithMessage(invalidResourceValue), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Requests["cpu"] = invalidResourceValue
		}},
		{"cpu resource limit is empty", nil, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["cpu"] = ""
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["cpu"] = "251m"
		}},
		{"cpu resource limit not set", nil, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["cpu"] = "251m"
		}},
		{"memory resource limit is empty", nil, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = ""
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "249Mi"
		}},
		{"memory resource limit not set", nil, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "249Mi"
		}},
		{"wrong public image config", radixvalidators.PublicImageComponentCannotHaveSourceOrDockerfileSetWithMessage(validRAFirstComponentName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Image = "redis:5.0-alpine"
			ra.Spec.Components[0].SourceFolder = "./api"
			ra.Spec.Components[0].DockerfileName = ".Dockerfile"
		}},
		{"inconcistent dynamic tag config for environment", radixvalidators.ComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTagWithMessage(validRAFirstComponentName, "prod"), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Image = "radixcanary.azurecr.io/my-private-image:some-tag"
			ra.Spec.Components[0].EnvironmentConfig[0].ImageTagName = "any-tag"
		}},
		{"invalid verificationType for component", radixvalidators.InvalidVerificationTypeWithMessage(string(invalidCertificateVerification)), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Authentication = &radixv1.Authentication{
				ClientCertificate: &radixv1.ClientCertificate{
					Verification: &invalidCertificateVerification,
				},
			}
		}},
		{"invalid verificationType for environment", radixvalidators.InvalidVerificationTypeWithMessage(string(invalidCertificateVerification)), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Authentication = &radixv1.Authentication{
				ClientCertificate: &radixv1.ClientCertificate{
					Verification: &invalidCertificateVerification,
				},
			}
		}},
		{"duplicate job name", radixvalidators.DuplicateComponentOrJobNameErrorWithMessage([]string{validRAFirstJobName}), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs = append(ra.Spec.Jobs, *ra.Spec.Jobs[0].DeepCopy())
		}},
		{"job name with oauth auxiliary name suffix", radixvalidators.ComponentNameReservedSuffixErrorWithMessage(oauthAuxSuffixJobName, "job", defaults.OAuthProxyAuxiliaryComponentSuffix), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Name = oauthAuxSuffixJobName
		}},
		{"invalid job secret name", radixvalidators.InvalidResourceNameErrorWithMessage("secret name", invalidVariableName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Secrets[0] = invalidVariableName
		}},
		{"too long job secret name", radixvalidators.InvalidStringValueMaxLengthErrorWithMessage("secret name", wayTooLongName, 253), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Secrets[0] = wayTooLongName
		}},
		{"invalid job common environment variable name", radixvalidators.InvalidResourceNameErrorWithMessage("environment variable name", invalidVariableName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Variables[invalidVariableName] = "Any value"
		}},
		{"too long job common environment variable name", radixvalidators.InvalidStringValueMaxLengthErrorWithMessage("environment variable name", wayTooLongName, 253), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Variables[wayTooLongName] = "Any value"
		}},
		{"invalid job environment variable name", radixvalidators.InvalidResourceNameErrorWithMessage("environment variable name", invalidVariableName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Variables[invalidVariableName] = "Any value"
		}},
		{"too long job environment variable name", radixvalidators.InvalidStringValueMaxLengthErrorWithMessage("environment variable name", wayTooLongName, 253), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Variables[wayTooLongName] = "Any value"
		}},
		{"conflicting job variable and secret name", radixvalidators.SecretNameConflictsWithEnvironmentVariableWithMessage("job", conflictingVariableName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Variables[conflictingVariableName] = "Any value"
			ra.Spec.Jobs[0].Secrets[0] = conflictingVariableName
		}},
		{"non existing env for job", radixvalidators.EnvironmentReferencedByComponentDoesNotExistErrorWithMessage(noExistingEnvironment, validRAFirstJobName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig = []radixv1.RadixJobComponentEnvironmentConfig{
				{
					Environment: noExistingEnvironment,
				},
			}
		}},
		{"scheduler port is not set", radixvalidators.SchedulerPortCannotBeEmptyForJobErrorWithMessage(validRAFirstJobName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].SchedulerPort = nil
		}},
		{"payload is empty struct", radixvalidators.PayloadPathCannotBeEmptyForJobErrorWithMessage(validRAFirstJobName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Payload = &radixv1.RadixJobComponentPayload{}
		}},
		{"payload path is empty string", radixvalidators.PayloadPathCannotBeEmptyForJobErrorWithMessage(validRAFirstJobName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Payload = &radixv1.RadixJobComponentPayload{Path: ""}
		}},

		{"job resource limit unsupported resource", radixvalidators.InvalidResourceErrorWithMessage(unsupportedResource), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits[unsupportedResource] = "250m"
		}},
		{"job memory resource limit wrong format", radixvalidators.MemoryResourceRequirementFormatErrorWithMessage(invalidResourceValue), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = invalidResourceValue
		}},
		{"job memory resource request wrong format", radixvalidators.MemoryResourceRequirementFormatErrorWithMessage(invalidResourceValue), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = invalidResourceValue
		}},
		{"job memory resource request larger than limit", radixvalidators.ResourceRequestOverLimitErrorWithMessage("memory", "249Mi", "250Ki"), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "250Ki"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "249Mi"
		}},
		{"job cpu resource limit wrong format", radixvalidators.CPUResourceRequirementFormatErrorWithMessage(invalidResourceValue), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["cpu"] = invalidResourceValue
		}},
		{"job cpu resource request wrong format", radixvalidators.CPUResourceRequirementFormatErrorWithMessage(invalidResourceValue), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["cpu"] = invalidResourceValue
		}},
		{"job cpu resource request larger than limit", radixvalidators.ResourceRequestOverLimitErrorWithMessage("cpu", "251m", "250m"), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["cpu"] = "250m"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["cpu"] = "251m"
		}},
		{"job resource request unsupported resource", radixvalidators.InvalidResourceErrorWithMessage(unsupportedResource), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests[unsupportedResource] = "250m"
		}},
		{"job common resource limit unsupported resource", radixvalidators.InvalidResourceErrorWithMessage(unsupportedResource), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits[unsupportedResource] = "250m"
		}},
		{"job common resource request unsupported resource", radixvalidators.InvalidResourceErrorWithMessage(unsupportedResource), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Requests[unsupportedResource] = "250m"
		}},
		{"job common memory resource limit wrong format", radixvalidators.MemoryResourceRequirementFormatErrorWithMessage(invalidResourceValue), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = invalidResourceValue
		}},
		{"job common memory resource request wrong format", radixvalidators.MemoryResourceRequirementFormatErrorWithMessage(invalidResourceValue), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Requests["memory"] = invalidResourceValue
		}},
		{"job common cpu resource limit wrong format", radixvalidators.CPUResourceRequirementFormatErrorWithMessage(invalidResourceValue), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["cpu"] = invalidResourceValue
		}},
		{"job common cpu resource request wrong format", radixvalidators.CPUResourceRequirementFormatErrorWithMessage(invalidResourceValue), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Requests["cpu"] = invalidResourceValue
		}},
		{"job cpu resource limit is empty", nil, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["cpu"] = ""
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["cpu"] = "251m"
		}},
		{"job cpu resource limit not set", nil, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["cpu"] = "251m"
		}},
		{"job memory resource limit is empty", nil, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = ""
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "249Mi"
		}},
		{"job memory resource limit not set", nil, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "249Mi"
		}},
		{"job wrong public image config", radixvalidators.PublicImageComponentCannotHaveSourceOrDockerfileSetWithMessage(validRAFirstJobName), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Image = "redis:5.0-alpine"
			ra.Spec.Jobs[0].SourceFolder = "./api"
			ra.Spec.Jobs[0].DockerfileName = ".Dockerfile"
		}},
		{"job inconcistent dynamic tag config for environment", radixvalidators.ComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTagWithMessage(validRAFirstJobName, "dev"), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Image = "radixcanary.azurecr.io/my-private-image:some-tag"
			ra.Spec.Jobs[0].EnvironmentConfig[0].ImageTagName = "any-tag"
		}},
		{"too long app name together with env name", fmt.Errorf("summary length of app name and environment together should not exceed 62 characters"), func(ra *radixv1.RadixApplication) {
			ra.Name = name50charsLong
			ra.Spec.Environments = append(ra.Spec.Environments, radixv1.Environment{Name: "extra-14-chars"})
		}},
		{"missing OAuth clientId for dev env - common OAuth config", radixvalidators.OAuthClientIdEmptyErrorWithMessage(validRAFirstComponentName, "dev"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].Authentication.OAuth2 = &radixv1.OAuth2{}
		}},
		{"missing OAuth clientId for prod env - environmentConfig OAuth config", radixvalidators.OAuthClientIdEmptyErrorWithMessage(validRAFirstComponentName, "prod"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.ClientID = ""
		}},
		{"OAuth path prefix is root", radixvalidators.OAuthProxyPrefixIsRootErrorWithMessage(validRAFirstComponentName, "prod"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.ProxyPrefix = "/"
		}},
		{"invalid OAuth session store type", radixvalidators.OAuthSessionStoreTypeInvalidErrorWithMessage(validRAFirstComponentName, "prod", "invalid-store"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SessionStoreType = "invalid-store"
		}},
		{"missing OAuth redisStore property", radixvalidators.OAuthRedisStoreEmptyErrorWithMessage(validRAFirstComponentName, "prod"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.RedisStore = nil
		}},
		{"missing OAuth redis connection URL", radixvalidators.OAuthRedisStoreConnectionURLEmptyErrorWithMessage(validRAFirstComponentName, "prod"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.RedisStore.ConnectionURL = ""
		}},
		{"no error when skipDiscovery=true and login, redeem and jwks urls set", nil, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.OIDC = &radixv1.OAuth2OIDC{
				SkipDiscovery: commonUtils.BoolPtr(true),
				JWKSURL:       "jwksurl",
			}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.LoginURL = "loginurl"
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.RedeemURL = "redeemurl"
		}},
		{"error when skipDiscovery=true and missing loginUrl", radixvalidators.OAuthLoginUrlEmptyErrorWithMessage(validRAFirstComponentName, "prod"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.OIDC = &radixv1.OAuth2OIDC{
				SkipDiscovery: commonUtils.BoolPtr(true),
				JWKSURL:       "jwksurl",
			}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.RedeemURL = "redeemurl"
		}},
		{"error when skipDiscovery=true and missing redeemUrl", radixvalidators.OAuthRedeemUrlEmptyErrorWithMessage(validRAFirstComponentName, "prod"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.OIDC = &radixv1.OAuth2OIDC{
				SkipDiscovery: commonUtils.BoolPtr(true),
				JWKSURL:       "jwksurl",
			}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.LoginURL = "loginurl"
		}},
		{"error when skipDiscovery=true and missing redeemUrl", radixvalidators.OAuthOidcJwksUrlEmptyErrorWithMessage(validRAFirstComponentName, "prod"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.OIDC = &radixv1.OAuth2OIDC{
				SkipDiscovery: commonUtils.BoolPtr(true),
			}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.LoginURL = "loginurl"
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.RedeemURL = "redeemurl"
		}},
		{"valid OAuth configuration for session store cookie and cookieStore.minimal=true", nil, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SessionStoreType = radixv1.SessionStoreCookie
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.CookieStore = &radixv1.OAuth2CookieStore{Minimal: commonUtils.BoolPtr(true)}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetAuthorizationHeader = commonUtils.BoolPtr(false)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetXAuthRequestHeaders = commonUtils.BoolPtr(false)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie = &radixv1.OAuth2Cookie{
				Expire:  "1h",
				Refresh: "0s",
			}
		}},
		{"error when cookieStore.minimal=true and SetAuthorizationHeader=true", radixvalidators.OAuthCookieStoreMinimalIncorrectSetAuthorizationHeaderErrorWithMessage(validRAFirstComponentName, "prod"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SessionStoreType = radixv1.SessionStoreCookie
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.CookieStore = &radixv1.OAuth2CookieStore{Minimal: commonUtils.BoolPtr(true)}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetAuthorizationHeader = commonUtils.BoolPtr(true)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetXAuthRequestHeaders = commonUtils.BoolPtr(false)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie = &radixv1.OAuth2Cookie{
				Expire:  "1h",
				Refresh: "0s",
			}
		}},
		{"error when cookieStore.minimal=true and SetXAuthRequestHeaders=true", radixvalidators.OAuthCookieStoreMinimalIncorrectSetXAuthRequestHeadersErrorWithMessage(validRAFirstComponentName, "prod"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SessionStoreType = radixv1.SessionStoreCookie
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.CookieStore = &radixv1.OAuth2CookieStore{Minimal: commonUtils.BoolPtr(true)}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetAuthorizationHeader = commonUtils.BoolPtr(false)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetXAuthRequestHeaders = commonUtils.BoolPtr(true)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie = &radixv1.OAuth2Cookie{
				Expire:  "1h",
				Refresh: "0s",
			}
		}},
		{"error when cookieStore.minimal=true and Cookie.Refresh>0", radixvalidators.OAuthCookieStoreMinimalIncorrectCookieRefreshIntervalErrorWithMessage(validRAFirstComponentName, "prod"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SessionStoreType = radixv1.SessionStoreCookie
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.CookieStore = &radixv1.OAuth2CookieStore{Minimal: commonUtils.BoolPtr(true)}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetAuthorizationHeader = commonUtils.BoolPtr(false)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetXAuthRequestHeaders = commonUtils.BoolPtr(false)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie = &radixv1.OAuth2Cookie{
				Expire:  "1h",
				Refresh: "1s",
			}
		}},
		{"invalid OAuth cookie same site", radixvalidators.OAuthCookieSameSiteInvalidErrorWithMessage(validRAFirstComponentName, "prod", "invalid-samesite"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.SameSite = "invalid-samesite"
		}},
		{"invalid OAuth cookie expire timeframe", radixvalidators.OAuthCookieExpireInvalidErrorWithMessage(validRAFirstComponentName, "prod", "invalid-expire"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Expire = "invalid-expire"
		}},
		{"negative OAuth cookie expire timeframe", radixvalidators.OAuthCookieExpireInvalidErrorWithMessage(validRAFirstComponentName, "prod", "-1s"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Expire = "-1s"
		}},
		{"invalid OAuth cookie refresh time frame", radixvalidators.OAuthCookieRefreshInvalidErrorWithMessage(validRAFirstComponentName, "prod", "invalid-refresh"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Refresh = "invalid-refresh"
		}},
		{"negative OAuth cookie refresh time frame", radixvalidators.OAuthCookieRefreshInvalidErrorWithMessage(validRAFirstComponentName, "prod", "-1s"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Refresh = "-1s"
		}},
		{"oauth cookie expire equals refresh", radixvalidators.OAuthCookieRefreshMustBeLessThanExpireErrorWithMessage(validRAFirstComponentName, "prod"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Expire = "1h"
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Refresh = "1h"
		}},
		{"oauth cookie expire less than refresh", radixvalidators.OAuthCookieRefreshMustBeLessThanExpireErrorWithMessage(validRAFirstComponentName, "prod"), func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Expire = "30m"
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Refresh = "1h"
		}},
		{"duplicate name in job/component boundary", radixvalidators.DuplicateComponentOrJobNameErrorWithMessage([]string{validRAFirstComponentName}), func(ra *radixv1.RadixApplication) {
			job := *ra.Spec.Jobs[0].DeepCopy()
			job.Name = validRAFirstComponentName
			ra.Spec.Jobs = append(ra.Spec.Jobs, job)
		}},
		{"no mask size postfix in egress rule destination", radixvalidators.DuplicateComponentOrJobNameErrorWithMessage([]string{validRAFirstComponentName}), func(ra *radixv1.RadixApplication) {
			job := *ra.Spec.Jobs[0].DeepCopy()
			job.Name = validRAFirstComponentName
			ra.Spec.Jobs = append(ra.Spec.Jobs, job)
		}},
		{"identity.azure.clientId cannot be empty for component", radixvalidators.ResourceNameCannotBeEmptyErrorWithMessage("identity.azure.clientId"), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Identity.Azure.ClientId = " "
		}},
		{"identity.azure.clientId cannot be empty for component environment config", radixvalidators.ResourceNameCannotBeEmptyErrorWithMessage("identity.azure.clientId"), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Identity.Azure.ClientId = " "
		}},
		{"identity.azure.clientId cannot be empty for job", radixvalidators.ResourceNameCannotBeEmptyErrorWithMessage("identity.azure.clientId"), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Identity.Azure.ClientId = " "
		}},
		{"identity.azure.clientId cannot be empty for job environment config", radixvalidators.ResourceNameCannotBeEmptyErrorWithMessage("identity.azure.clientId"), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Identity.Azure.ClientId = " "
		}},

		{"invalid identity.azure.clientId for component", radixvalidators.InvalidUUIDErrorWithMessage("identity.azure.clientId", "1111-22-33-44"), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Identity.Azure.ClientId = "1111-22-33-44"
		}},
		{"invalid identity.azure.clientId for component environment config", radixvalidators.InvalidUUIDErrorWithMessage("identity.azure.clientId", "1111-22-33-44"), func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Identity.Azure.ClientId = "1111-22-33-44"
		}},
		{"invalid identity.azure.clientId for job", radixvalidators.InvalidUUIDErrorWithMessage("identity.azure.clientId", "1111-22-33-44"), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Identity.Azure.ClientId = "1111-22-33-44"
		}},
		{"invalid identity.azure.clientId for job environment config", radixvalidators.InvalidUUIDErrorWithMessage("identity.azure.clientId", "1111-22-33-44"), func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Identity.Azure.ClientId = "1111-22-33-44"
		}},
	}

	_, client := validRASetup()
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := createValidRA()
			testcase.updateRA(validRA)
			err := radixvalidators.CanRadixApplicationBeInserted(context.Background(), client, validRA, getDNSAliasConfig())

			if testcase.expectedError != nil {
				assert.Error(t, err)
				assertErrorCauseIs(t, err, testcase.expectedError, "Expected error is not contained in list of errors")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func assertErrorCauseIs(t *testing.T, err error, expectedError error, msgAndArgs ...interface{}) {
	assert.Contains(t, err.Error(), expectedError.Error(), msgAndArgs...)

	// If expecedError exposes a Cause method, lets check if the return error has the same cause error
	if x, ok := expectedError.(interface{ Cause() error }); ok {
		assert.ErrorIs(t, err, x.Cause(), msgAndArgs...)
	}
}

func Test_ValidRAComponentLimitRequest_NoError(t *testing.T) {
	var testScenarios = []struct {
		name     string
		updateRA updateRAFunc
	}{
		{"resource memory correct format: 50", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50"
		}},
		{"resource limit correct format: 50T", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50T"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50T"
		}},
		{"resource limit correct format: 50G", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50G"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50G"
		}},
		{"resource limit correct format: 50M", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50M"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50M"
		}},
		{"resource limit correct format: 50k", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50k"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50k"
		}},
		{"resource limit correct format: 50Gi", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50Gi"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50Gi"
		}},
		{"resource limit correct format: 50Mi", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50Mi"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50Mi"
		}},
		{"resource limit correct format: 50Ki", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50Ki"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50Ki"
		}},
		{"common resource memory correct format: 50", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50"
		}},
		{"common resource limit correct format: 50T", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50T"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50T"
		}},
		{"common resource limit correct format: 50G", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50G"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50G"
		}},
		{"common resource limit correct format: 50M", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50M"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50M"
		}},
		{"common resource limit correct format: 50k", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50k"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50k"
		}},
		{"common resource limit correct format: 50Gi", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50Gi"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50Gi"
		}},
		{"common resource limit correct format: 50Mi", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50Mi"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50Mi"
		}},
		{"common resource limit correct format: 50Ki", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50Ki"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50Ki"
		}},
	}

	_, client := validRASetup()
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := createValidRA()
			testcase.updateRA(validRA)
			err := radixvalidators.CanRadixApplicationBeInserted(context.Background(), client, validRA, getDNSAliasConfig())

			assert.NoError(t, err)
		})
	}
}

func Test_ValidRAJobLimitRequest_NoError(t *testing.T) {
	var testScenarios = []struct {
		name     string
		updateRA updateRAFunc
	}{
		{"resource memory correct format: 50", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50"
		}},
		{"resource limit correct format: 50T", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50T"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50T"
		}},
		{"resource limit correct format: 50G", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50G"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50G"
		}},
		{"resource limit correct format: 50M", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50M"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50M"
		}},
		{"resource limit correct format: 50k", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50k"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50k"
		}},
		{"resource limit correct format: 50Gi", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50Gi"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50Gi"
		}},
		{"resource limit correct format: 50Mi", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50Mi"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50Mi"
		}},
		{"resource limit correct format: 50Ki", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50Ki"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50Ki"
		}},
		{"common resource memory correct format: 50", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50"
		}},
		{"common resource limit correct format: 50T", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50T"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50T"
		}},
		{"common resource limit correct format: 50G", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50G"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50G"
		}},
		{"common resource limit correct format: 50M", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50M"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50M"
		}},
		{"common resource limit correct format: 50k", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50k"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50k"
		}},
		{"common resource limit correct format: 50Gi", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50Gi"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50Gi"
		}},
		{"common resource limit correct format: 50Mi", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50Mi"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50Mi"
		}},
		{"common resource limit correct format: 50Ki", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50Ki"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50Ki"
		}},
	}

	_, client := validRASetup()
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := createValidRA()
			testcase.updateRA(validRA)
			err := radixvalidators.CanRadixApplicationBeInserted(context.Background(), client, validRA, getDNSAliasConfig())

			assert.NoError(t, err)
		})
	}
}

func Test_InvalidRAComponentLimitRequest_Error(t *testing.T) {
	var testScenarios = []struct {
		name     string
		updateRA updateRAFunc
	}{
		{"resource limit incorrect format: 50MB", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50MB"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50MB"
		}},
		{"resource limit incorrect format: 50K", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50K"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50K"
		}},
		{"common resource limit incorrect format: 50MB", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50MB"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50MB"
		}},
		{"common resource limit incorrect format: 50K", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50K"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50K"
		}},
	}

	_, client := validRASetup()
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := createValidRA()
			testcase.updateRA(validRA)
			err := radixvalidators.CanRadixApplicationBeInserted(context.Background(), client, validRA, getDNSAliasConfig())

			assert.Error(t, err)
		})
	}
}

func Test_InvalidRAJobLimitRequest_Error(t *testing.T) {
	var testScenarios = []struct {
		name     string
		updateRA updateRAFunc
	}{
		{"resource limit incorrect format: 50MB", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50MB"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50MB"
		}},
		{"resource limit incorrect format: 50K", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50K"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50K"
		}},
		{"common resource limit incorrect format: 50MB", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50MB"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50MB"
		}},
		{"common resource limit incorrect format: 50K", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50K"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50K"
		}},
	}

	_, client := validRASetup()
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := createValidRA()
			testcase.updateRA(validRA)
			err := radixvalidators.CanRadixApplicationBeInserted(context.Background(), client, validRA, getDNSAliasConfig())

			assert.Error(t, err)
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
			updateRA: func(ra *radixv1.RadixApplication) {
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
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].PublicPort = "http"
				ra.Spec.Components[0].Ports[0].Name = "http"
				ra.Spec.Components[0].Public = true
			},
			isValid:    true,
			isErrorNil: true,
		},
		{
			name: "port name is irrelevant for non-public component if old public does not exist",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].PublicPort = ""
				ra.Spec.Components[0].Ports[0].Name = "test"
				ra.Spec.Components[0].Public = false
				ra.Spec.Components[0].Authentication.OAuth2 = nil
				ra.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2 = nil
			},
			isValid:    true,
			isErrorNil: true,
		},
		{
			// For backwards compatibility
			name: "old public is used if it exists and new publicPort does not exist",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].PublicPort = ""
				ra.Spec.Components[0].Ports[0].Name = "test"
				ra.Spec.Components[0].Public = true
			},
			isValid:    true,
			isErrorNil: true,
		},
		{
			name: "missing port name for public component, old public does not exist",
			updateRA: func(ra *radixv1.RadixApplication) {
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
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].PublicPort = "http"
				ra.Spec.Components[0].Ports[0].Name = "test"
				ra.Spec.Components[0].Public = true
			},
			isValid:    false,
			isErrorNil: false,
		},
		{
			name: "duplicate port name for public component, old public does not exist",
			updateRA: func(ra *radixv1.RadixApplication) {
				newPorts := []radixv1.ComponentPort{
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
			updateRA: func(ra *radixv1.RadixApplication) {
				newPorts := []radixv1.ComponentPort{
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
			updateRA: func(ra *radixv1.RadixApplication) {
				newPorts := []radixv1.ComponentPort{
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
		{
			name: "oauth2 require public port",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].Ports = []radixv1.ComponentPort{{Name: "http", Port: 1000}}
				ra.Spec.Components[0].PublicPort = ""
			},
			isValid:    false,
			isErrorNil: false,
		},
		{
			name: "oauth2 require ports",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].Ports = nil
				ra.Spec.Components[0].PublicPort = ""
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
			err := radixvalidators.CanRadixApplicationBeInserted(context.Background(), client, validRA, getDNSAliasConfig())

			if testcase.isValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

			assert.Equal(t, testcase.isErrorNil, err == nil)
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
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[1].Variables["RADIX"] = "any value"
			},
			isValid:    true,
			isErrorNil: true,
		},
		{
			name: "check that user defined variable with legal prefix succeeds",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[1].Variables["RADIXX_SOMETHING"] = "any value"
			},
			isValid:    true,
			isErrorNil: true,
		},
		{
			name: "check that user defined variable with legal prefix succeeds",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[1].Variables["SOMETHING_RADIX_SOMETHING"] = "any value"
			},
			isValid:    true,
			isErrorNil: true,
		},
		{
			name: "check that user defined variable with legal prefix succeeds",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[1].Variables["S_RADIXOPERATOR"] = "any value"
			},
			isValid:    true,
			isErrorNil: true,
		},
		{
			name: "check that user defined variable with illegal prefix fails",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[1].Variables["RADIX_SOMETHING"] = "any value"
			},
			isValid:    false,
			isErrorNil: false,
		},
		{
			name: "check that user defined variable with illegal prefix fails",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[1].Variables["RADIXOPERATOR_SOMETHING"] = "any value"
			},
			isValid:    false,
			isErrorNil: false,
		},
		{
			name: "check that user defined variable with illegal prefix fails",
			updateRA: func(ra *radixv1.RadixApplication) {
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
			err := radixvalidators.CanRadixApplicationBeInserted(context.Background(), client, validRA, getDNSAliasConfig())

			if testcase.isValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

			assert.Equal(t, testcase.isErrorNil, err == nil)
		})
	}
}

func Test_ValidationOfVolumeMounts_Errors(t *testing.T) {
	type volumeMountsFunc func() []radixv1.RadixVolumeMount
	type setVolumeMountsFunc func(*radixv1.RadixApplication, []radixv1.RadixVolumeMount)

	setRaComponentVolumeMounts := func(ra *radixv1.RadixApplication, volumeMounts []radixv1.RadixVolumeMount) {
		ra.Spec.Components[0].EnvironmentConfig[0].VolumeMounts = volumeMounts
	}

	setRaJobsVolumeMounts := func(ra *radixv1.RadixApplication, volumeMounts []radixv1.RadixVolumeMount) {
		ra.Spec.Jobs[0].EnvironmentConfig[0].VolumeMounts = volumeMounts
	}

	setComponentAndJobsVolumeMounts := []setVolumeMountsFunc{setRaComponentVolumeMounts, setRaJobsVolumeMounts}

	var testScenarios = map[string]struct {
		volumeMounts  volumeMountsFunc
		updateRA      []setVolumeMountsFunc
		expectedError error
	}{
		"name not set": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "",
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixvalidators.ErrVolumeMountMissingName,
		},
		"path not set": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "anyname",
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixvalidators.ErrVolumeMountMissingPath,
		},
		"duplicate name": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "anyname",
						Path: "/path1",
					},
					{
						Name: "anyname",
						Path: "/path2",
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixvalidators.ErrVolumeMountDuplicateName,
		},
		"duplicate path": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "anyname1",
						Path: "/path1",
					},
					{
						Name: "anyname2",
						Path: "/path1",
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixvalidators.ErrVolumeMountDuplicatePath,
		},
		"missing type": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "anyname",
						Path: "/path",
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixvalidators.ErrVolumeMountMissingType,
		},
		"multiple types: deprecated source and blobfuse2": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name:      "anyname",
						Path:      "/path",
						Type:      radixv1.MountTypeBlob,
						BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{},
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixvalidators.ErrVolumeMountMultipleTypes,
		},
		"multiple types: blobfuse2 and emptyDir": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name:      "anyname",
						Path:      "/path",
						Type:      radixv1.MountTypeBlob,
						BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{},
						EmptyDir:  &radixv1.RadixEmptyDirVolumeMount{},
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixvalidators.ErrVolumeMountMultipleTypes,
		},
		"deprecated blob: valid": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Type:      "blob",
						Name:      "some_name",
						Path:      "some_path",
						Container: "any-container",
						// RequestsStorage: "50M",
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: nil,
		},
		"deprecated blob: missing container": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Type: "blob",
						Name: "some_name",
						Path: "some_path",
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixvalidators.ErrVolumeMountMissingContainer,
		},
		"deprecated azure-blob: valid": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Type:    "azure-blob",
						Name:    "some_name",
						Path:    "some_path",
						Storage: "any-storage",
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: nil,
		},
		"deprecated azure-blob: missing container": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Type: "azure-blob",
						Name: "some_name",
						Path: "some_path",
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixvalidators.ErrVolumeMountMissingStorage,
		},
		"deprecated azure-file: valid": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Type:    "azure-file",
						Name:    "some_name",
						Path:    "some_path",
						Storage: "any-storage",
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: nil,
		},
		"deprecated common: invalid type": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Type: "invalid",
						Name: "some_name",
						Path: "some_path",
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixvalidators.ErrVolumeMountInvalidType,
		},
		"deprecated common: invalid requestsStorage": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Type:            "blob",
						Name:            "some_name",
						Path:            "some_path",
						RequestsStorage: "50x",
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixvalidators.ErrVolumeMountInvalidRequestsStorage,
		},
		"blobfuse2: valid": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
							Container: "any-container",
						},
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: nil,
		},
		"blobfuse2: valid protocol fuse2": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
							Protocol:  radixv1.BlobFuse2ProtocolFuse2,
							Container: "any-container",
						},
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: nil,
		},
		"blobfuse2: valid protocol nfs": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
							Protocol:  radixv1.BlobFuse2ProtocolNfs,
							Container: "any-container",
						},
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: nil,
		},
		"blobfuse2: valid requestsStorage": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
							Container:       "any-container",
							RequestsStorage: "50Mi",
						},
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: nil,
		},
		"blobfuse2: invalid protocol": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
							Protocol:  "invalid",
							Container: "any-container",
						},
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixvalidators.ErrVolumeMountInvalidProtocol,
		},
		"blobfuse2: missing container": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name:      "some_name",
						Path:      "some_path",
						BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{},
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixvalidators.ErrVolumeMountMissingContainer,
		},
		"blobfuse2: invalid requestsStorage": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
							Container:       "any-container",
							RequestsStorage: "100x",
						},
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixvalidators.ErrVolumeMountInvalidRequestsStorage,
		},
		"azureFile: not implemented": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name:      "some_name",
						Path:      "some_path",
						AzureFile: &radixv1.RadixAzureFileVolumeMount{},
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixvalidators.ErrVolumeMountTypeNotImplemented,
		},
		"emptyDir: valid": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						EmptyDir: &radixv1.RadixEmptyDirVolumeMount{
							SizeLimit: resource.MustParse("50M"),
						},
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: nil,
		},
		"emptyDir: missing sizeLimit": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name:     "some_name",
						Path:     "some_path",
						EmptyDir: &radixv1.RadixEmptyDirVolumeMount{},
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixvalidators.ErrVolumeMountMissingSizeLimit,
		},
	}

	_, client := validRASetup()
	for name, test := range testScenarios {
		t.Run(name, func(t *testing.T) {
			if test.updateRA == nil || len(test.updateRA) == 0 {
				assert.FailNow(t, "missing updateRA functions for %s", name)
				return
			}

			for _, ra := range test.updateRA {
				validRA := createValidRA()
				volumes := test.volumeMounts()
				ra(validRA, volumes)
				err := radixvalidators.CanRadixApplicationBeInserted(context.Background(), client, validRA, getDNSAliasConfig())
				if test.expectedError == nil {
					assert.NoError(t, err)
				} else {
					assert.ErrorIs(t, err, test.expectedError)
				}
			}
		})
	}
}

func Test_HPA_Validation(t *testing.T) {
	var testScenarios = []struct {
		name     string
		updateRA updateRAFunc
		isErrors []error
	}{
		{
			"horizontalScaling is not set",
			func(ra *radixv1.RadixApplication) {},
			nil,
		},
		{
			"component HPA minReplicas and maxReplicas are not set",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{}
			},
			[]error{radixvalidators.ErrMaxReplicasForHPANotSetOrZero},
		},
		{
			"component HPA maxReplicas is not set and minReplicas is set",
			func(ra *radixv1.RadixApplication) {
				minReplica := int32(3)
				ra.Spec.Components[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{}
				ra.Spec.Components[0].HorizontalScaling.MinReplicas = &minReplica
			},
			[]error{radixvalidators.ErrMinReplicasGreaterThanMaxReplicas},
		},
		{
			"component HPA minReplicas is not set and maxReplicas is set",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{}
				ra.Spec.Components[0].HorizontalScaling.MaxReplicas = 2
			},
			nil,
		},
		{
			"component HPA minReplicas is set to 0 but only CPU and Memory resoruces are set",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{}
				ra.Spec.Components[0].HorizontalScaling.MaxReplicas = 2
				ra.Spec.Components[0].HorizontalScaling.MinReplicas = pointers.Ptr[int32](0)
				ra.Spec.Components[0].HorizontalScaling.RadixHorizontalScalingResources = &radixv1.RadixHorizontalScalingResources{
					Cpu:    &radixv1.RadixHorizontalScalingResource{AverageUtilization: pointers.Ptr[int32](80)},
					Memory: &radixv1.RadixHorizontalScalingResource{AverageUtilization: pointers.Ptr[int32](80)},
				}
			},
			[]error{radixvalidators.ErrInvalidMinimumReplicasConfigurationWithMemoryAndCPUTriggers},
		},
		{
			"component HPA minReplicas is set to 1 and only CPU and Memory resoruces are set",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{}
				ra.Spec.Components[0].HorizontalScaling.MaxReplicas = 2
				ra.Spec.Components[0].HorizontalScaling.MinReplicas = pointers.Ptr[int32](1)
				ra.Spec.Components[0].HorizontalScaling.RadixHorizontalScalingResources = &radixv1.RadixHorizontalScalingResources{
					Cpu:    &radixv1.RadixHorizontalScalingResource{AverageUtilization: pointers.Ptr[int32](80)},
					Memory: &radixv1.RadixHorizontalScalingResource{AverageUtilization: pointers.Ptr[int32](80)},
				}
			},
			nil,
		},
		{
			"component HPA minReplicas is greater than maxReplicas",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{}
				minReplica := int32(3)
				ra.Spec.Components[0].HorizontalScaling.MinReplicas = &minReplica
				ra.Spec.Components[0].HorizontalScaling.MaxReplicas = 2
			},
			[]error{radixvalidators.ErrMinReplicasGreaterThanMaxReplicas},
		},
		{
			"component HPA maxReplicas is greater than minReplicas",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{}
				minReplica := int32(3)
				ra.Spec.Components[0].HorizontalScaling.MinReplicas = &minReplica
				ra.Spec.Components[0].HorizontalScaling.MaxReplicas = 4
			},
			nil,
		},
		{
			"component HPA custom resource scaling for HPA is set, but no resource thresholds are defined",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{MinReplicas: pointers.Ptr(int32(2)), MaxReplicas: 4,
					RadixHorizontalScalingResources: &radixv1.RadixHorizontalScalingResources{},
				}
			},
			[]error{radixvalidators.ErrNoScalingResourceSet},
		},
		{
			"component HPA custom resource scaling for HPA is set, but no resource thresholds for CPU AverageUtilization is not defined",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{MinReplicas: pointers.Ptr(int32(2)), MaxReplicas: 4,
					RadixHorizontalScalingResources: &radixv1.RadixHorizontalScalingResources{
						Cpu: &radixv1.RadixHorizontalScalingResource{},
					},
				}
			},
			[]error{radixvalidators.ErrNoScalingResourceSet},
		},
		{
			"component HPA custom resource scaling for HPA is set, but no resource thresholds for memory AverageUtilization is not defined",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{MinReplicas: pointers.Ptr(int32(2)), MaxReplicas: 4,
					RadixHorizontalScalingResources: &radixv1.RadixHorizontalScalingResources{
						Memory: &radixv1.RadixHorizontalScalingResource{},
					},
				}
			},
			[]error{radixvalidators.ErrNoScalingResourceSet},
		},
		{
			"component HPA custom resource scaling for HPA is set, but no resource thresholds for CPU and memory AverageUtilization are defined",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{MinReplicas: pointers.Ptr(int32(2)), MaxReplicas: 4,
					RadixHorizontalScalingResources: &radixv1.RadixHorizontalScalingResources{
						Cpu:    &radixv1.RadixHorizontalScalingResource{},
						Memory: &radixv1.RadixHorizontalScalingResource{},
					},
				}
			},
			[]error{radixvalidators.ErrNoScalingResourceSet},
		},
		{
			"component HPA custom resource scaling for HPA CPU AverageUtilization is set",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{MinReplicas: pointers.Ptr(int32(2)), MaxReplicas: 4,
					RadixHorizontalScalingResources: &radixv1.RadixHorizontalScalingResources{
						Cpu: &radixv1.RadixHorizontalScalingResource{AverageUtilization: pointers.Ptr(int32(80))},
					},
				}
			},
			nil,
		},
		{
			"component HPA custom resource scaling for HPA memory AverageUtilization is set",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{MinReplicas: pointers.Ptr(int32(2)), MaxReplicas: 4,
					RadixHorizontalScalingResources: &radixv1.RadixHorizontalScalingResources{
						Memory: &radixv1.RadixHorizontalScalingResource{AverageUtilization: pointers.Ptr(int32(80))},
					},
				}
			},
			nil,
		},
		{
			"component HPA custom resource scaling for HPA memory AverageUtilization is set",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{MinReplicas: pointers.Ptr(int32(2)), MaxReplicas: 4,
					RadixHorizontalScalingResources: &radixv1.RadixHorizontalScalingResources{
						Cpu:    &radixv1.RadixHorizontalScalingResource{AverageUtilization: pointers.Ptr(int32(80))},
						Memory: &radixv1.RadixHorizontalScalingResource{AverageUtilization: pointers.Ptr(int32(80))},
					},
				}
			},
			nil,
		},
		{
			"environment HPA minReplicas and maxReplicas are not set",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{}
			},
			// TODO: Maybe this validation should not return ErrMinReplicasGreaterThanMaxReplicas (it happens because minReplicas defaults to 1
			[]error{radixvalidators.ErrMinReplicasGreaterThanMaxReplicas},
		},
		{
			"environment HPA maxReplicas is not set and minReplicas is set",
			func(ra *radixv1.RadixApplication) {
				minReplica := int32(3)
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{}
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling.MinReplicas = &minReplica
			},
			[]error{radixvalidators.ErrMinReplicasGreaterThanMaxReplicas},
		},
		{
			"environment HPA minReplicas is not set and maxReplicas is set",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{}
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling.MaxReplicas = 2
			},
			nil,
		},
		{
			"environment HPA minReplicas is greater than maxReplicas",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{}
				minReplica := int32(3)
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling.MinReplicas = &minReplica
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling.MaxReplicas = 2
			},
			[]error{radixvalidators.ErrMinReplicasGreaterThanMaxReplicas},
		},
		{
			"environment HPA maxReplicas is greater than minReplicas",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{}
				minReplica := int32(3)
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling.MinReplicas = &minReplica
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling.MaxReplicas = 4
			},
			nil,
		},
		{
			"environment HPA custom resource scaling for HPA is set, but no resource thresholds are defined",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{}
				minReplica := int32(2)
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling.MinReplicas = &minReplica
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling.MaxReplicas = 4
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling.RadixHorizontalScalingResources = &radixv1.RadixHorizontalScalingResources{}
			},
			[]error{radixvalidators.ErrNoScalingResourceSet},
		},
		{
			"environment HPA custom resource scaling for HPA is set, but no resource thresholds are defined",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{MinReplicas: pointers.Ptr(int32(2)), MaxReplicas: 4,
					RadixHorizontalScalingResources: &radixv1.RadixHorizontalScalingResources{},
				}
			},
			[]error{radixvalidators.ErrNoScalingResourceSet},
		},
		{
			"environment HPA custom resource scaling for HPA is set, but no resource thresholds for CPU AverageUtilization is not defined",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{MinReplicas: pointers.Ptr(int32(2)), MaxReplicas: 4,
					RadixHorizontalScalingResources: &radixv1.RadixHorizontalScalingResources{
						Cpu: &radixv1.RadixHorizontalScalingResource{},
					},
				}
			},
			[]error{radixvalidators.ErrNoScalingResourceSet},
		},
		{
			"environment HPA custom resource scaling for HPA is set, but no resource thresholds for memory AverageUtilization is not defined",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{MinReplicas: pointers.Ptr(int32(2)), MaxReplicas: 4,
					RadixHorizontalScalingResources: &radixv1.RadixHorizontalScalingResources{
						Memory: &radixv1.RadixHorizontalScalingResource{},
					},
				}
			},
			[]error{radixvalidators.ErrNoScalingResourceSet},
		},
		{
			"environment HPA custom resource scaling for HPA is set, but no resource thresholds for CPU and memory AverageUtilization are defined",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{MinReplicas: pointers.Ptr(int32(2)), MaxReplicas: 4,
					RadixHorizontalScalingResources: &radixv1.RadixHorizontalScalingResources{
						Cpu:    &radixv1.RadixHorizontalScalingResource{},
						Memory: &radixv1.RadixHorizontalScalingResource{},
					},
				}
			},
			[]error{radixvalidators.ErrNoScalingResourceSet},
		},
		{
			"environment HPA custom resource scaling for HPA CPU AverageUtilization is set",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{MinReplicas: pointers.Ptr(int32(2)), MaxReplicas: 4,
					RadixHorizontalScalingResources: &radixv1.RadixHorizontalScalingResources{
						Cpu: &radixv1.RadixHorizontalScalingResource{AverageUtilization: pointers.Ptr(int32(80))},
					},
				}
			},
			nil,
		},
		{
			"environment HPA custom resource scaling for HPA memory AverageUtilization is set",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{MinReplicas: pointers.Ptr(int32(2)), MaxReplicas: 4,
					RadixHorizontalScalingResources: &radixv1.RadixHorizontalScalingResources{
						Memory: &radixv1.RadixHorizontalScalingResource{AverageUtilization: pointers.Ptr(int32(80))},
					},
				}
			},
			nil,
		},
		{
			"environment HPA custom resource scaling for HPA memory AverageUtilization is set",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{MinReplicas: pointers.Ptr(int32(2)), MaxReplicas: 4,
					RadixHorizontalScalingResources: &radixv1.RadixHorizontalScalingResources{
						Cpu:    &radixv1.RadixHorizontalScalingResource{AverageUtilization: pointers.Ptr(int32(80))},
						Memory: &radixv1.RadixHorizontalScalingResource{AverageUtilization: pointers.Ptr(int32(80))},
					},
				}
			},
			nil,
		},
	}

	_, client := validRASetup()
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := createValidRA()
			testcase.updateRA(validRA)
			err := radixvalidators.CanRadixApplicationBeInserted(context.Background(), client, validRA, getDNSAliasConfig())

			if testcase.isErrors == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.NotEqual(t, 0, len(testcase.isErrors), "Test is invalid, should have atleast 1 target error")
				for _, target := range testcase.isErrors {
					assert.ErrorIs(t, err, target)
				}
			}
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
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []radixv1.EgressRule{{
					Destinations: []radixv1.EgressDestination{"notanIPmask"},
					Ports:        nil,
				}}
			},
			isValid: false,
		},
		{
			name: "egress rule must use IPv4 in destination CIDR, zero ports",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []radixv1.EgressRule{{
					Destinations: []radixv1.EgressDestination{"2001:0DB8:0000:000b::/64"},
					Ports:        nil,
				}}
			},
			isValid: false,
		},
		{
			name: "egress rule must use IPv4 in destination CIDR",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []radixv1.EgressRule{{
					Destinations: []radixv1.EgressDestination{"2001:0DB8:0000:000b::/64"},
					Ports: []radixv1.EgressPort{{
						Port:     10,
						Protocol: "TCP",
					}},
				}}
			},
			isValid: false,
		},
		{
			name: "egress rule must have postfix in IPv4 CIDR",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []radixv1.EgressRule{{
					Destinations: []radixv1.EgressDestination{"10.0.0.1"},
					Ports:        nil,
				}}
			},
			isValid: false,
		},
		{
			name: "egress rule must have valid ports",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []radixv1.EgressRule{{
					Destinations: []radixv1.EgressDestination{"10.0.0.1"},
					Ports: []radixv1.EgressPort{{
						Port:     0,
						Protocol: "TCP",
					}},
				}}
			},
			isValid: false,
		},
		{
			name: "egress rule must have valid ports",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []radixv1.EgressRule{{
					Destinations: []radixv1.EgressDestination{"10.0.0.1"},
					Ports: []radixv1.EgressPort{{
						Port:     66000,
						Protocol: "TCP",
					}},
				}}
			},
			isValid: false,
		},
		{
			name: "egress rule must contain destination",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []radixv1.EgressRule{{
					Destinations: nil,
					Ports: []radixv1.EgressPort{{
						Port:     24,
						Protocol: "TCP",
					}},
				}}
			},
			isValid: false,
		},
		{
			name: "egress rule must have valid port protocol",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []radixv1.EgressRule{{
					Destinations: []radixv1.EgressDestination{"10.0.0.1/32"},
					Ports: []radixv1.EgressPort{{
						Port:     2000,
						Protocol: "SCTP",
					}},
				}}
			},
			isValid: false,
		},
		{
			name: "egress rule must have valid port protocol",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []radixv1.EgressRule{{
					Destinations: []radixv1.EgressDestination{"10.0.0.1/32"},
					Ports: []radixv1.EgressPort{{
						Port:     2000,
						Protocol: "erwef",
					}},
				}}
			},
			isValid: false,
		},
		{
			name: "can not exceed max nr of egress rules",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []radixv1.EgressRule{}
				for i := 0; i <= 1000; i++ {
					ra.Spec.Environments[0].Egress.Rules = append(ra.Spec.Environments[0].Egress.Rules, radixv1.EgressRule{
						Destinations: []radixv1.EgressDestination{"10.0.0.0/8"},
						Ports:        nil,
					})
				}
			},
			isValid: false,
		},
		{
			name: "sample egress rule with valid destination, zero ports",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []radixv1.EgressRule{{
					Destinations: []radixv1.EgressDestination{"10.0.0.0/8"},
					Ports:        nil,
				}}
			},
			isValid: true,
		},
		{
			name: "sample egress rule with valid destinations",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Environments[0].Egress.Rules = []radixv1.EgressRule{{
					Destinations: []radixv1.EgressDestination{"10.0.0.0/8", "192.10.10.10/32"},
					Ports: []radixv1.EgressPort{
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
			err := radixvalidators.CanRadixApplicationBeInserted(context.Background(), client, validRA, getDNSAliasConfig())

			if testcase.isValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

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
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = nil
			},
		},
		{name: "valid webhook with http", expectedError: nil,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://api:8090/abc")}
			},
		},
		{name: "valid webhook with https", expectedError: nil,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("https://api:8090/abc")}
			},
		},
		{name: "valid webhook in environment", expectedError: nil,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://api:8090/abc")}
			},
		},
		{name: "valid webhook to job component", expectedError: nil,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://job3:8099/abc")}
			},
		},
		{name: "valid webhook to job component in environment", expectedError: nil,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://job3:8099/abc")}
			},
		},
		{name: "Invalid webhook URL", expectedError: radixvalidators.InvalidWebhookUrlWithMessage("job", ""),
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &radixv1.Notifications{Webhook: &invalidUrl}
			},
		},
		{name: "Invalid webhook URL in environment", expectedError: radixvalidators.InvalidWebhookUrlWithMessage("job", "dev"),
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &radixv1.Notifications{Webhook: &invalidUrl}
			},
		},
		{name: "Not allowed scheme ftp", expectedError: radixvalidators.NotAllowedSchemeInWebhookUrlWithMessage("ftp", "job", ""),
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("ftp://api:8090")}
			},
		},
		{name: "Not allowed scheme ftp in environment", expectedError: radixvalidators.NotAllowedSchemeInWebhookUrlWithMessage("ftp", "job", "dev"),
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("ftp://api:8090")}
			},
		},
		{name: "missing port in the webhook", expectedError: radixvalidators.MissingPortInWebhookUrlWithMessage("job", ""),
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://api/abc")}
			},
		},
		{name: "missing port in the webhook in environment", expectedError: radixvalidators.MissingPortInWebhookUrlWithMessage("job", "dev"),
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://api/abc")}
			},
		},
		{name: "webhook can only reference to an application component or job", expectedError: radixvalidators.OnlyAppComponentAllowedInWebhookUrlWithMessage("job", ""),
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://notexistingcomponent:8090/abc")}
			},
		},
		{name: "webhook can only reference to an application component or job in environment", expectedError: radixvalidators.OnlyAppComponentAllowedInWebhookUrlWithMessage("job", "dev"),
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://notexistingcomponent:8090/abc")}
			},
		},
		{name: "webhook port does not exist in an application component", expectedError: radixvalidators.InvalidPortInWebhookUrlWithMessage("8077", "api", "job", ""),
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://api:8077")}
			},
		},
		{name: "webhook port does not exist in an application component in environment", expectedError: radixvalidators.InvalidPortInWebhookUrlWithMessage("8077", "api", "job", "dev"),
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://api:8077")}
			},
		},
		{name: "webhook port does not exist in an application job component", expectedError: radixvalidators.InvalidPortInWebhookUrlWithMessage("8077", "job3", "job", ""),
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://job3:8077")}
			},
		},
		{name: "webhook port does not exist in an application job component in environment", expectedError: radixvalidators.InvalidPortInWebhookUrlWithMessage("8077", "job3", "job", "dev"),
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://job3:8077")}
			},
		},
		{name: "not allowed to use in the webhook a public port of an application component", expectedError: radixvalidators.InvalidUseOfPublicPortInWebhookUrlWithMessage("8080", "app", "job", ""),
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://app:8080")}
			},
		},
		{name: "not allowed to use in the webhook a public port of an application component in environment", expectedError: radixvalidators.InvalidUseOfPublicPortInWebhookUrlWithMessage("8080", "app", "job", "dev"),
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://app:8080")}
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

func Test_ValidateApplicationCanBeAppliedWithDNSAliases(t *testing.T) {
	const (
		raAppName         = "anyapp"
		otherAppName      = "anyapp2"
		raEnv             = "test"
		raComponentName   = "app"
		raPublicPort      = 8080
		someEnv           = "dev"
		someComponentName = "component-abc"
		somePort          = 9090
		alias1            = "alias1"
		alias2            = "alias2"
	)
	dnsConfig := &dnsaliasconfig.DNSConfig{
		DNSZone:               "dev.radix.equinor.com",
		ReservedAppDNSAliases: dnsaliasconfig.AppReservedDNSAlias{"api": "radix-api"},
		ReservedDNSAliases:    []string{"grafana"},
	}
	var testScenarios = []struct {
		name                    string
		applicationBuilder      utils.ApplicationBuilder
		existingRadixDNSAliases map[string]commonTest.DNSAlias
		expectedValidationError error
	}{
		{
			name:                    "No dns aliases",
			applicationBuilder:      utils.ARadixApplication(),
			expectedValidationError: nil,
		},
		{
			name:                    "Added dns aliases",
			applicationBuilder:      utils.ARadixApplication().WithDNSAlias(radixv1.DNSAlias{Alias: alias1, Environment: raEnv, Component: raComponentName}),
			expectedValidationError: nil,
		},
		{
			name:               "Existing dns aliases for the app",
			applicationBuilder: utils.ARadixApplication().WithDNSAlias(radixv1.DNSAlias{Alias: alias1, Environment: raEnv, Component: raComponentName}),
			existingRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: raAppName, Environment: raEnv, Component: raComponentName},
			},
			expectedValidationError: nil,
		},
		{
			name:               "Existing dns aliases for the app and another app",
			applicationBuilder: utils.ARadixApplication().WithDNSAlias(radixv1.DNSAlias{Alias: alias1, Environment: raEnv, Component: raComponentName}),
			existingRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: raAppName, Environment: raEnv, Component: raComponentName},
				alias2: {AppName: otherAppName, Environment: someEnv, Component: someComponentName},
			},
			expectedValidationError: nil,
		},
		{
			name:               "Same alias exists in dns alias for another app",
			applicationBuilder: utils.ARadixApplication().WithDNSAlias(radixv1.DNSAlias{Alias: alias1, Environment: raEnv, Component: raComponentName}),
			existingRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: otherAppName, Environment: someEnv, Component: someComponentName},
			},
			expectedValidationError: radixvalidators.RadixDNSAliasAlreadyUsedByAnotherApplicationError(alias1),
		},
		{
			name:                    "Reserved alias api for another app",
			applicationBuilder:      utils.ARadixApplication().WithDNSAlias(radixv1.DNSAlias{Alias: "api", Environment: raEnv, Component: raComponentName}),
			expectedValidationError: radixvalidators.RadixDNSAliasIsReservedForRadixPlatformApplicationError("api"),
		},
		{
			name:                    "Reserved alias api for another service",
			applicationBuilder:      utils.ARadixApplication().WithDNSAlias(radixv1.DNSAlias{Alias: "grafana", Environment: raEnv, Component: raComponentName}),
			expectedValidationError: radixvalidators.RadixDNSAliasIsReservedForRadixPlatformServiceError("grafana"),
		},
		{
			name:                    "Reserved alias api for this app",
			applicationBuilder:      utils.ARadixApplication().WithAppName("radix-api").WithDNSAlias(radixv1.DNSAlias{Alias: "api", Environment: raEnv, Component: raComponentName}),
			expectedValidationError: nil,
		},
	}

	for _, ts := range testScenarios {
		t.Run(ts.name, func(t *testing.T) {
			_, radixClient := validRASetup()

			err := commonTest.RegisterRadixDNSAliases(context.Background(), radixClient, ts.existingRadixDNSAliases)
			require.NoError(t, err)
			rr := ts.applicationBuilder.GetRegistrationBuilder().BuildRR()
			_, err = radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
			require.NoError(t, err)
			ra := ts.applicationBuilder.BuildRA()

			actualValidationErr := radixvalidators.CanRadixApplicationBeInserted(context.Background(), radixClient, ra, dnsConfig)

			if ts.expectedValidationError == nil {
				require.NoError(t, actualValidationErr)
			} else {
				require.EqualError(t, actualValidationErr, ts.expectedValidationError.Error(), "missing or unexpected error")
			}
		})
	}
}

func createValidRA() *radixv1.RadixApplication {
	validRA, _ := utils.GetRadixApplicationFromFile("testdata/radixconfig.yaml")

	return validRA
}

func validRASetup() (kubernetes.Interface, radixclient.Interface) {
	validRR, _ := utils.GetRadixRegistrationFromFile("testdata/radixregistration.yaml")
	kubeclient := kubefake.NewSimpleClientset()
	client := radixfake.NewSimpleClientset(validRR)

	return kubeclient, client
}

func getDNSAliasConfig() *dnsaliasconfig.DNSConfig {
	return &dnsaliasconfig.DNSConfig{
		DNSZone:               "dev.radix.equinor.com",
		ReservedAppDNSAliases: dnsaliasconfig.AppReservedDNSAlias{"api": "radix-api"},
		ReservedDNSAliases:    []string{"grafana"},
	}
}
