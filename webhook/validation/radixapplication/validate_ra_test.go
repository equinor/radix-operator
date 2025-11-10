//nolint:staticcheck
package radixapplication_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/webhook/validation/radixapplication"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(radixv1.AddToScheme(scheme))
}

type updateRAFunc func(rr *radixv1.RadixApplication)

func Test_ParseRadixApplication_LimitMemoryIsTakenFromRequestsMemory(t *testing.T) {
	radixClient := createClient("testdata/radixregistration.yaml")
	ra := load[*radixv1.RadixApplication]("./testdata/radixconfig.yaml")

	validator := radixapplication.CreateOnlineValidator(radixClient, []string{"grafana"}, map[string]string{"api": "radix-api"})
	wnrs, err := validator.Validate(context.Background(), ra)
	assert.NoError(t, err)
	assert.Empty(t, wnrs)
}

func Test_valid_ra_returns_true(t *testing.T) {
	client := createClient("testdata/radixregistration.yaml")
	validRA := load[*radixv1.RadixApplication]("./testdata/radixconfig.yaml")
	validator := radixapplication.CreateOnlineValidator(client, []string{"grafana"}, map[string]string{"api": "radix-api"})
	wnrs, err := validator.Validate(context.Background(), validRA)

	assert.NoError(t, err)
	assert.Empty(t, wnrs)
}

func Test_missing_rr(t *testing.T) {
	client := createClient()
	validRA := load[*radixv1.RadixApplication]("./testdata/radixconfig.yaml")

	validator := radixapplication.CreateOnlineValidator(client, []string{"grafana"}, map[string]string{"api": "radix-api"})
	wnrs, err := validator.Validate(context.Background(), validRA)

	assert.Error(t, err)
	assert.Empty(t, wnrs)
}

func Test_invalid_ra(t *testing.T) {
	wayTooLongName := "waytoooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooolongname"
	invalidBranchName := "/master"
	oauthAuxSuffixComponentName := fmt.Sprintf("app-%s", radixv1.OAuthProxyAuxiliaryComponentSuffix)
	oauthAuxSuffixJobName := fmt.Sprintf("job-%s", radixv1.OAuthProxyAuxiliaryComponentSuffix)
	invalidVariableName := "invalid:variable"
	noReleatedRRAppName := "no related rr"
	noExistingEnvironment := "nonexistingenv"
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
		{"no related rr", radixapplication.ErrNoRadixApplication, func(ra *radixv1.RadixApplication) {
			ra.Name = noReleatedRRAppName
		}},
		{"non existing env for component", radixapplication.ErrEnvironmentReferencedByComponentDoesNotExist, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig = []radixv1.RadixEnvironmentConfig{
				{
					Environment: noExistingEnvironment,
				},
			}
		}},
		{"component name with oauth auxiliary name suffix", radixapplication.ErrComponentNameReservedSuffix, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Name = oauthAuxSuffixComponentName
		}},
		{"invalid environment variable name", radixapplication.ErrVariableNameCannotContainIllegalCharacters, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[1].EnvironmentConfig[0].Variables[invalidVariableName] = "Any value"
		}},
		{"too long environment variable name", radixapplication.ErrVariableNameCannotExceedMaxLength, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[1].EnvironmentConfig[0].Variables[wayTooLongName] = "Any value"
		}},
		{"conflicting variable and secret name", radixapplication.ErrSecretNameConflictsWithVariable, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[1].EnvironmentConfig[0].Variables[conflictingVariableName] = "Any value"
			ra.Spec.Components[1].Secrets[0] = conflictingVariableName
		}},
		{"invalid common environment variable name", radixapplication.ErrVariableNameCannotContainIllegalCharacters, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[1].Variables[invalidVariableName] = "Any value"
		}},
		{"too long common environment variable name", radixapplication.ErrVariableNameCannotExceedMaxLength, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[1].Variables[wayTooLongName] = "Any value"
		}},
		{"conflicting common variable and secret name", radixapplication.ErrSecretNameConflictsWithVariable, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[1].Variables[conflictingVariableName] = "Any value"
			ra.Spec.Components[1].Secrets[0] = conflictingVariableName
		}},
		{"conflicting common variable and secret name when not environment config", radixapplication.ErrSecretNameConflictsWithVariable, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[1].Variables[conflictingVariableName] = "Any value"
			ra.Spec.Components[1].Secrets[0] = conflictingVariableName
			ra.Spec.Components[1].EnvironmentConfig = nil
		}},
		{"invalid number of replicas in an environment", radixapplication.ErrInvalidNumberOfReplicas, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Replicas = pointers.Ptr(radixapplication.MaxReplica + 1)
		}},
		{"invalid number of replicas in a component", radixapplication.ErrInvalidNumberOfReplicas, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Replicas = pointers.Ptr(radixapplication.MaxReplica + 1)
		}},
		{"invalid branch name", radixapplication.ErrInvalidBranchName, func(ra *radixv1.RadixApplication) {
			ra.Spec.Environments[0].Build.From = invalidBranchName
		}},
		{"too long branch name", radixapplication.ErrBranchFromTooLong, func(ra *radixv1.RadixApplication) {
			ra.Spec.Environments[0].Build.From = wayTooLongName
		}},
		{"dns app alias non existing component", radixapplication.ErrDNSAliasComponentNotDefinedOrDisabled, func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAppAlias.Component = nonExistingComponent
		}},
		{"dns app alias non existing env", radixapplication.ErrDNSAliasEnvironmentNotDefined, func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAppAlias.Environment = noExistingEnvironment
		}},
		{"dns alias non existing component", radixapplication.ErrDNSAliasComponentNotDefinedOrDisabled, func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAlias[0].Component = nonExistingComponent
		}},
		{"dns alias non existing env", radixapplication.ErrDNSAliasEnvironmentNotDefined, func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAlias[0].Environment = noExistingEnvironment
		}},
		{"dns alias with no public port", radixapplication.ErrDNSAliasComponentIsNotMarkedAsPublic, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[3].PublicPort = ""
			ra.Spec.Components[3].Public = false
			ra.Spec.DNSAlias[0] = radixv1.DNSAlias{
				Alias:       "my-alias",
				Component:   ra.Spec.Components[3].Name,
				Environment: ra.Spec.Environments[0].Name,
			}
		}},
		{"dns alias non existing component", radixapplication.ErrDNSAliasComponentNotDefinedOrDisabled, func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAppAlias.Component = nonExistingComponent
		}},
		{"dns alias non existing env", radixapplication.ErrDNSAliasEnvironmentNotDefined, func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSAppAlias.Environment = noExistingEnvironment
		}},
		{"dns external alias non existing component", radixapplication.ErrExternalAliasComponentNotDefined, func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSExternalAlias = []radixv1.ExternalAlias{
				{
					Alias:       "some.alias.com",
					Component:   nonExistingComponent,
					Environment: ra.Spec.Environments[0].Name,
				},
			}
		}},
		{"dns external alias non existing environment", radixapplication.ErrExternalAliasEnvironmentNotDefined, func(ra *radixv1.RadixApplication) {
			ra.Spec.DNSExternalAlias = []radixv1.ExternalAlias{
				{
					Alias:       "some.alias.com",
					Component:   ra.Spec.Components[0].Name,
					Environment: noExistingEnvironment,
				},
			}
		}},
		{"dns external alias with no public port", radixapplication.ErrExternalAliasComponentNotMarkedAsPublic, func(ra *radixv1.RadixApplication) {
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
		{"resource limit unsupported resource", radixapplication.ErrInvalidResourceType, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits[unsupportedResource] = "250m"
		}},
		{"memory resource limit wrong format", radixapplication.ErrMemoryResourceRequirementFormat, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = invalidResourceValue
		}},
		{"memory resource request wrong format", radixapplication.ErrInvalidResourceFormat, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = invalidResourceValue
		}},
		{"memory resource request larger than limit", radixapplication.ErrMemoryResourceRequirementFormat, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "250Ki"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "249Mi"
		}},
		{"cpu resource limit wrong format", radixapplication.ErrInvalidResourceFormat, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["cpu"] = invalidResourceValue
		}},
		{"cpu resource request wrong format", radixapplication.ErrInvalidResourceFormat, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["cpu"] = invalidResourceValue
		}},
		{"cpu resource request larger than limit", radixapplication.ErrRequestedResourceExceedsLimit, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["cpu"] = "250m"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["cpu"] = "251m"
		}},
		{"resource request unsupported resource", radixapplication.ErrInvalidResourceType, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests[unsupportedResource] = "250m"
		}},
		{"common resource limit unsupported resource", radixapplication.ErrInvalidResourceType, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits[unsupportedResource] = "250m"
		}},
		{"common resource request unsupported resource", radixapplication.ErrInvalidResourceType, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Requests[unsupportedResource] = "250m"
		}},
		{"common memory resource limit wrong format", radixapplication.ErrInvalidResourceFormat, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = invalidResourceValue
		}},
		{"common memory resource request wrong format", radixapplication.ErrMemoryResourceRequirementFormat, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Requests["memory"] = invalidResourceValue
		}},
		{"common cpu resource limit wrong format", radixapplication.ErrInvalidResourceFormat, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["cpu"] = invalidResourceValue
		}},
		{"common cpu resource request wrong format", radixapplication.ErrInvalidResourceFormat, func(ra *radixv1.RadixApplication) {
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
		{"invalid verificationType for component", radixapplication.ErrInvalidVerificationType, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Authentication = &radixv1.Authentication{
				ClientCertificate: &radixv1.ClientCertificate{
					Verification: &invalidCertificateVerification,
				},
			}
		}},
		{"invalid verificationType for environment", radixapplication.ErrInvalidVerificationType, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Authentication = &radixv1.Authentication{
				ClientCertificate: &radixv1.ClientCertificate{
					Verification: &invalidCertificateVerification,
				},
			}
		}},
		{"job name with oauth auxiliary name suffix", radixapplication.ErrComponentNameReservedSuffix, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Name = oauthAuxSuffixJobName
		}},
		{"invalid job common environment variable name", radixapplication.ErrVariableNameCannotContainIllegalCharacters, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Variables[invalidVariableName] = "Any value"
		}},
		{"too long job common environment variable name", radixapplication.ErrVariableNameCannotExceedMaxLength, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Variables[wayTooLongName] = "Any value"
		}},
		{"invalid job environment variable name", radixapplication.ErrVariableNameCannotContainIllegalCharacters, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Variables[invalidVariableName] = "Any value"
		}},
		{"too long job environment variable name", radixapplication.ErrVariableNameCannotExceedMaxLength, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Variables[wayTooLongName] = "Any value"
		}},
		{"conflicting job variable and secret name", radixapplication.ErrSecretNameConflictsWithVariable, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Variables[conflictingVariableName] = "Any value"
			ra.Spec.Jobs[0].Secrets[0] = conflictingVariableName
		}},
		{"non existing env for job", radixapplication.ErrEnvironmentReferencedByComponentDoesNotExist, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig = []radixv1.RadixJobComponentEnvironmentConfig{
				{
					Environment: noExistingEnvironment,
				},
			}
		}},
		{"job resource limit unsupported resource", radixapplication.ErrInvalidResourceType, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits[unsupportedResource] = "250m"
		}},
		{"job memory resource limit wrong format", radixapplication.ErrMemoryResourceRequirementFormat, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = invalidResourceValue
		}},
		{"job memory resource request wrong format", radixapplication.ErrMemoryResourceRequirementFormat, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = invalidResourceValue
		}},
		{"job memory resource request larger than limit", radixapplication.ErrRequestedResourceExceedsLimit, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "250Ki"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "249Mi"
		}},
		{"job cpu resource limit wrong format", radixapplication.ErrInvalidResourceFormat, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["cpu"] = invalidResourceValue
		}},
		{"job cpu resource request wrong format", radixapplication.ErrInvalidResourceFormat, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["cpu"] = invalidResourceValue
		}},
		{"job cpu resource request larger than limit", radixapplication.ErrRequestedResourceExceedsLimit, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["cpu"] = "250m"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["cpu"] = "251m"
		}},
		{"job resource request unsupported resource", radixapplication.ErrInvalidResourceType, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests[unsupportedResource] = "250m"
		}},
		{"job common resource limit unsupported resource", radixapplication.ErrInvalidResourceType, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits[unsupportedResource] = "250m"
		}},
		{"job common resource request unsupported resource", radixapplication.ErrInvalidResourceType, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Requests[unsupportedResource] = "250m"
		}},
		{"job common memory resource limit wrong format", radixapplication.ErrMemoryResourceRequirementFormat, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = invalidResourceValue
		}},
		{"job common memory resource request wrong format", radixapplication.ErrMemoryResourceRequirementFormat, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Requests["memory"] = invalidResourceValue
		}},
		{"job common cpu resource limit wrong format", radixapplication.ErrInvalidResourceFormat, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["cpu"] = invalidResourceValue
		}},
		{"job common cpu resource request wrong format", radixapplication.ErrInvalidResourceFormat, func(ra *radixv1.RadixApplication) {
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
		{"too long app name together with env name", radixapplication.ErrInvalidEnvironmentNameLength, func(ra *radixv1.RadixApplication) {
			ra.Name = name50charsLong
			ra.Spec.Environments = append(ra.Spec.Environments, radixv1.Environment{Name: "extra-14-chars"})
		}},
		{"missing OAuth clientId for dev env - common OAuth config", radixapplication.ErrOAuthClientIdEmpty, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].Authentication.OAuth2 = &radixv1.OAuth2{}
		}},
		{"missing OAuth clientId for prod env - environmentConfig OAuth config", radixapplication.ErrOAuthClientIdEmpty, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.ClientID = ""
		}},
		{"OAuth path prefix is root", radixapplication.ErrOAuthProxyPrefixIsRoot, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.ProxyPrefix = "/"
		}},
		{"invalid OAuth session store type", radixapplication.ErrOAuthSessionStoreTypeInvalid, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SessionStoreType = "invalid-store"
		}},
		{"missing OAuth redisStore property", radixapplication.ErrOAuthRedisStoreEmpty, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.RedisStore = nil
		}},
		{"missing OAuth redis connection URL", radixapplication.ErrOAuthRedisStoreConnectionURLEmpty, func(rr *radixv1.RadixApplication) {
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
		{"error when skipDiscovery=true and missing loginUrl", radixapplication.ErrOAuthLoginUrlEmpty, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.OIDC = &radixv1.OAuth2OIDC{
				SkipDiscovery: commonUtils.BoolPtr(true),
				JWKSURL:       "jwksurl",
			}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.RedeemURL = "redeemurl"
		}},
		{"error when skipDiscovery=true and missing redeemUrl", radixapplication.ErrOAuthRedeemUrlEmpty, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.OIDC = &radixv1.OAuth2OIDC{
				SkipDiscovery: commonUtils.BoolPtr(true),
				JWKSURL:       "jwksurl",
			}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.LoginURL = "loginurl"
		}},
		{"error when skipDiscovery=true and missing redeemUrl", radixapplication.ErrOAuthOidcJwksUrlEmpty, func(rr *radixv1.RadixApplication) {
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
		{"error when cookieStore.minimal=true and SetAuthorizationHeader=true", radixapplication.ErrOAuthCookieStoreMinimalIncorrectSetAuthorizationHeader, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SessionStoreType = radixv1.SessionStoreCookie
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.CookieStore = &radixv1.OAuth2CookieStore{Minimal: commonUtils.BoolPtr(true)}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetAuthorizationHeader = commonUtils.BoolPtr(true)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetXAuthRequestHeaders = commonUtils.BoolPtr(false)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie = &radixv1.OAuth2Cookie{
				Expire:  "1h",
				Refresh: "0s",
			}
		}},
		{"error when cookieStore.minimal=true and SetXAuthRequestHeaders=true", radixapplication.ErrOAuthCookieStoreMinimalIncorrectSetXAuthRequestHeaders, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SessionStoreType = radixv1.SessionStoreCookie
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.CookieStore = &radixv1.OAuth2CookieStore{Minimal: commonUtils.BoolPtr(true)}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetAuthorizationHeader = commonUtils.BoolPtr(false)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetXAuthRequestHeaders = commonUtils.BoolPtr(true)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie = &radixv1.OAuth2Cookie{
				Expire:  "1h",
				Refresh: "0s",
			}
		}},
		{"error when cookieStore.minimal=true and Cookie.Refresh>0", radixapplication.ErrOAuthCookieStoreMinimalIncorrectCookieRefreshInterval, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SessionStoreType = radixv1.SessionStoreCookie
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.CookieStore = &radixv1.OAuth2CookieStore{Minimal: commonUtils.BoolPtr(true)}
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetAuthorizationHeader = commonUtils.BoolPtr(false)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SetXAuthRequestHeaders = commonUtils.BoolPtr(false)
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie = &radixv1.OAuth2Cookie{
				Expire:  "1h",
				Refresh: "1s",
			}
		}},
		{"invalid OAuth cookie same site", radixapplication.ErrOAuthCookieSameSiteInvalid, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.SameSite = "invalid-samesite"
		}},
		{"invalid OAuth cookie expire timeframe", radixapplication.ErrOAuthCookieExpireInvalid, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Expire = "invalid-expire"
		}},
		{"negative OAuth cookie expire timeframe", radixapplication.ErrOAuthCookieExpireInvalid, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Expire = "-1s"
		}},
		{"invalid OAuth cookie refresh time frame", radixapplication.ErrOAuthCookieRefreshInvalid, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Refresh = "invalid-refresh"
		}},
		{"negative OAuth cookie refresh time frame", radixapplication.ErrOAuthCookieRefreshInvalid, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Refresh = "-1s"
		}},
		{"oauth cookie expire equals refresh", radixapplication.ErrOAuthCookieRefreshMustBeLessThanExpire, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Expire = "1h"
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Refresh = "1h"
		}},
		{"oauth cookie expire less than refresh", radixapplication.ErrOAuthCookieRefreshMustBeLessThanExpire, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Expire = "30m"
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.Cookie.Refresh = "1h"
		}},
		{"oauth SkipAuthRoutes are correct", nil, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SkipAuthRoutes = []string{"POST=^/api/public-entity/?$", "GET=^/skip/auth/routes/get", "!=^/api"}
		}},
		{"oauth SkipAuthRoutes are invalid", radixapplication.ErrOAuthSkipAuthRoutesInvalid, func(rr *radixv1.RadixApplication) {
			// Bad regexes do not compile
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SkipAuthRoutes = []string{
				"POST=/(foo",
				"OPTIONS=/foo/bar)",
				"GET=^]/foo/bar[$",
				"GET=^]/foo/bar[$",
			}
		}},
		{"oauth SkipAuthRoutes failed because has comma", radixapplication.ErrOAuthSkipAuthRoutesInvalid, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2.SkipAuthRoutes = []string{"POST=^/api/public,entity/?$", "GET=^/skip/auth/routes/get", "!=^/api"}
		}},
		{"invalid healthchecks are invalid", radixapplication.ErrInvalidHealthCheckProbe, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].HealthChecks = &radixv1.RadixHealthChecks{
				LivenessProbe: &radixv1.RadixProbe{
					RadixProbeHandler: radixv1.RadixProbeHandler{
						HTTPGet:   &radixv1.RadixProbeHTTPGetAction{Port: 5000, Path: "/healthz"},
						Exec:      &radixv1.RadixProbeExecAction{Command: []string{"/bin/sh", "-c", "/healthz"}},
						TCPSocket: &radixv1.RadixProbeTCPSocketAction{Port: 5000},
					},
				},
				ReadinessProbe: &radixv1.RadixProbe{
					RadixProbeHandler: radixv1.RadixProbeHandler{
						HTTPGet:   &radixv1.RadixProbeHTTPGetAction{Port: 5000, Path: "/healthz"},
						Exec:      &radixv1.RadixProbeExecAction{Command: []string{"/bin/sh", "-c", "/healthz"}},
						TCPSocket: &radixv1.RadixProbeTCPSocketAction{Port: 5000},
					},
				},
				StartupProbe: &radixv1.RadixProbe{
					RadixProbeHandler: radixv1.RadixProbeHandler{
						HTTPGet:   &radixv1.RadixProbeHTTPGetAction{Port: 5000, Path: "/healthz"},
						Exec:      &radixv1.RadixProbeExecAction{Command: []string{"/bin/sh", "-c", "/healthz"}},
						TCPSocket: &radixv1.RadixProbeTCPSocketAction{Port: 5000},
					},
				},
			}
		}},
		{"invalid healthchecks are invalid", radixapplication.ErrSuccessThresholdMustBeOne, func(rr *radixv1.RadixApplication) {
			rr.Spec.Components[0].HealthChecks = &radixv1.RadixHealthChecks{
				LivenessProbe: &radixv1.RadixProbe{
					RadixProbeHandler: radixv1.RadixProbeHandler{
						HTTPGet: &radixv1.RadixProbeHTTPGetAction{Port: 5000, Path: "/healthz"},
					},
					SuccessThreshold: 5,
				},
			}
		}},
		{"invalid exit code 0 for In operator in failure policy for job", radixapplication.ErrFailurePolicyRuleExitCodeZeroNotAllowedForInOperator, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].FailurePolicy = &radixv1.RadixJobComponentFailurePolicy{
				Rules: []radixv1.RadixJobComponentFailurePolicyRule{
					{
						Action:      radixv1.RadixJobComponentFailurePolicyActionFailJob,
						OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn, Values: []int32{0}},
					},
				}}
		}},
		{"invalid exit code 0 for In operator in failure policy for job environment config", radixapplication.ErrFailurePolicyRuleExitCodeZeroNotAllowedForInOperator, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].FailurePolicy = &radixv1.RadixJobComponentFailurePolicy{
				Rules: []radixv1.RadixJobComponentFailurePolicyRule{
					{
						Action:      radixv1.RadixJobComponentFailurePolicyActionFailJob,
						OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn, Values: []int32{0}},
					},
				}}
		}},
	}

	client := createClient("testdata/radixregistration.yaml")
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := load[*radixv1.RadixApplication]("./testdata/radixconfig.yaml")
			testcase.updateRA(validRA)
			validator := radixapplication.CreateOnlineValidator(client, []string{"grafana"}, map[string]string{"api": "radix-api"})
			wnrs, err := validator.Validate(context.Background(), validRA)
			assert.Empty(t, wnrs)

			if testcase.expectedError != nil {
				fmt.Println(err)
				assert.Error(t, err)
				assert.ErrorIs(t, err, testcase.expectedError, "Expected error is not contained in list of errors")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_RA_WithWarnings(t *testing.T) {

	var testScenarios = []struct {
		name            string
		expectedWarning string
		updateRA        updateRAFunc
	}{
		{"wrong public image config", radixapplication.WarnPublicImageComponentCannotHaveSourceOrDockerfileSetWithImage, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Image = "redis:5.0-alpine"
			ra.Spec.Components[0].SourceFolder = "./api"
			ra.Spec.Components[0].DockerfileName = ".Dockerfile"
		}},
		{"inconcistent dynamic tag config for environment", radixapplication.WarnComponentWithDynamicTagRequiresImageTag, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Image = "radixcanary.azurecr.io/my-private-image:some-tag"
			ra.Spec.Components[0].EnvironmentConfig[0].ImageTagName = "any-tag"
		}},

		{"job inconcistent dynamic tag config for environment", radixapplication.WarnPublicImageComponentCannotHaveSourceOrDockerfileSetWithImage, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Image = "radixcanary.azurecr.io/my-private-image:some-tag"
			ra.Spec.Jobs[0].EnvironmentConfig[0].ImageTagName = "any-tag"
		}},
		{"component memory limit below minimum", radixapplication.WarnMemoryResourceBelowRecommendedMinimum, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "10M"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "10M"
		}},
		{"component memory request below minimum", radixapplication.WarnMemoryResourceBelowRecommendedMinimum, func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "15M"
		}},
		{"job memory limit below minimum", radixapplication.WarnMemoryResourceBelowRecommendedMinimum, func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "10M"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "10M"
		}},
	}
	client := createClient("testdata/radixregistration.yaml")
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := load[*radixv1.RadixApplication]("./testdata/radixconfig.yaml")
			testcase.updateRA(validRA)
			validator := radixapplication.CreateOnlineValidator(client, []string{}, map[string]string{})
			wrns, err := validator.Validate(context.Background(), validRA)
			assert.NoError(t, err)

			if testcase.expectedWarning == "" {
				assert.Empty(t, wrns)
			} else {
				found := false
				for _, wrn := range wrns {
					if strings.Contains(wrn, testcase.expectedWarning) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning '%s' not found in warnings: %v", testcase.expectedWarning, wrns)
				}
			}
		})
	}
}

func Test_ValidRAComponentLimitRequest_NoError(t *testing.T) {
	var testScenarios = []struct {
		name     string
		updateRA updateRAFunc
	}{
		{"resource memory correct format: 50M", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50M"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50M"
		}},
		{"resource limit correct format: 50T", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50T"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50T"
		}},
		{"resource limit correct format: 50G", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50G"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50G"
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
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50Mi"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50Mi"
		}},
		{"common resource memory correct format: 50M", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50M"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50M"
		}},
		{"common resource limit correct format: 50T", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50T"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50T"
		}},
		{"common resource limit correct format: 50G", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50G"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50G"
		}},
		{"common resource limit correct format: 50Gi", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50Gi"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50Gi"
		}},
		{"common resource limit correct format: 50Mi", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].Resources.Limits["memory"] = "50Mi"
			ra.Spec.Components[0].Resources.Requests["memory"] = "50Mi"
		}},
		{"memory limit at minimum threshold 20M produces no warning", func(ra *radixv1.RadixApplication) {
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Limits["memory"] = "20M"
			ra.Spec.Components[0].EnvironmentConfig[0].Resources.Requests["memory"] = "20M"
		}},
	}

	client := createClient("./testdata/radixregistration.yaml")
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := load[*radixv1.RadixApplication]("./testdata/radixconfig.yaml")
			testcase.updateRA(validRA)
			validator := radixapplication.CreateOnlineValidator(client, []string{"grafana"}, map[string]string{"api": "radix-api"})
			wnrs, err := validator.Validate(context.Background(), validRA)

			assert.NoError(t, err)
			assert.Empty(t, wnrs)
		})
	}
}

func Test_ValidRAJobLimitRequest_NoError(t *testing.T) {
	var testScenarios = []struct {
		name     string
		updateRA updateRAFunc
	}{
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
		{"resource limit correct format: 50Gi", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50Gi"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50Gi"
		}},
		{"resource limit correct format: 50Mi", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Limits["memory"] = "50Mi"
			ra.Spec.Jobs[0].EnvironmentConfig[0].Resources.Requests["memory"] = "50Mi"
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
		{"common resource limit correct format: 50Gi", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50Gi"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50Gi"
		}},
		{"common resource limit correct format: 50Mi", func(ra *radixv1.RadixApplication) {
			ra.Spec.Jobs[0].Resources.Limits["memory"] = "50Mi"
			ra.Spec.Jobs[0].Resources.Requests["memory"] = "50Mi"
		}},
	}

	client := createClient("testdata/radixregistration.yaml")
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := load[*radixv1.RadixApplication]("./testdata/radixconfig.yaml")
			testcase.updateRA(validRA)
			validator := radixapplication.CreateOnlineValidator(client, []string{"grafana"}, map[string]string{"api": "radix-api"})
			wnrs, err := validator.Validate(context.Background(), validRA)

			assert.NoError(t, err)
			assert.Empty(t, wnrs)
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

	client := createClient("testdata/radixregistration.yaml")
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := load[*radixv1.RadixApplication]("./testdata/radixconfig.yaml")
			testcase.updateRA(validRA)
			validator := radixapplication.CreateOnlineValidator(client, []string{"grafana"}, map[string]string{"api": "radix-api"})
			wnrs, err := validator.Validate(context.Background(), validRA)

			assert.Empty(t, wnrs)
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

	client := createClient("testdata/radixregistration.yaml")
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := load[*radixv1.RadixApplication]("./testdata/radixconfig.yaml")
			testcase.updateRA(validRA)
			validator := radixapplication.CreateOnlineValidator(client, []string{"grafana"}, map[string]string{"api": "radix-api"})
			wnrs, err := validator.Validate(context.Background(), validRA)

			assert.Empty(t, wnrs)
			assert.Error(t, err)
		})
	}
}

func Test_PublicPort(t *testing.T) {
	var testScenarios = []struct {
		name      string
		updateRA  updateRAFunc
		isValid   bool
		isWarning bool
	}{
		{
			name: "matching port name for public component, old public does not exist",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].PublicPort = "http"
				ra.Spec.Components[0].Ports[0].Name = "http"
				ra.Spec.Components[0].Public = false
			},
			isValid:   true,
			isWarning: false,
		},
		{
			// For backwards compatibility
			name: "matching port name for public component, old public exists (ignored)",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].PublicPort = "http"
				ra.Spec.Components[0].Ports[0].Name = "http"
				ra.Spec.Components[0].Public = true
			},
			isValid:   true,
			isWarning: true,
		},
		{
			name: "port name is irrelevant for non-public component if old public does not exist",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].PublicPort = ""
				ra.Spec.Components[0].Ports[0].Name = "test"
				ra.Spec.Components[0].Public = false
				ra.Spec.Components[0].Authentication.OAuth2 = nil
				ra.Spec.Components[0].EnvironmentConfig[0].Authentication.OAuth2 = nil
				ra.Spec.DNSAlias = nil
				ra.Spec.DNSAppAlias = radixv1.AppAlias{}
			},
			isValid:   true,
			isWarning: false,
		},
		{
			// For backwards compatibility
			name: "old public is used if it exists and new publicPort does not exist",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].PublicPort = ""
				ra.Spec.Components[0].Ports[0].Name = "test"
				ra.Spec.Components[0].Public = true
			},
			isValid:   true,
			isWarning: true,
		},
		{
			name: "missing port name for public component, old public does not exist",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].PublicPort = "http"
				ra.Spec.Components[0].Ports[0].Name = "test"
				ra.Spec.Components[0].Public = false
			},
			isValid:   false,
			isWarning: false,
		},
		{
			// For backwards compatibility
			name: "missing port name for public component, old public exists (ignored)",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].PublicPort = "http"
				ra.Spec.Components[0].Ports[0].Name = "test"
				ra.Spec.Components[0].Public = true
			},
			isValid:   false,
			isWarning: true,
		},
		{
			name: "oauth2 require public port",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].Ports = []radixv1.ComponentPort{{Name: "http", Port: 1000}}
				ra.Spec.Components[0].PublicPort = ""
			},
			isValid:   false,
			isWarning: false,
		},
		{
			name: "oauth2 require ports",
			updateRA: func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].Ports = nil
				ra.Spec.Components[0].PublicPort = ""
			},
			isValid:   false,
			isWarning: false,
		},
	}

	client := createClient("testdata/radixregistration.yaml")
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := load[*radixv1.RadixApplication]("./testdata/radixconfig.yaml")
			testcase.updateRA(validRA)
			validator := radixapplication.CreateOnlineValidator(client, []string{"grafana"}, map[string]string{"api": "radix-api"})
			wnrs, err := validator.Validate(context.Background(), validRA)

			if testcase.isValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

			if testcase.isWarning {
				assert.NotEmpty(t, wnrs)
			} else {
				assert.Empty(t, wnrs)
			}
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

	client := createClient("testdata/radixregistration.yaml")
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := load[*radixv1.RadixApplication]("./testdata/radixconfig.yaml")
			testcase.updateRA(validRA)
			validator := radixapplication.CreateOnlineValidator(client, []string{"grafana"}, map[string]string{"api": "radix-api"})
			wnrs, err := validator.Validate(context.Background(), validRA)

			if testcase.isValid {
				assert.NoError(t, err)
				assert.Empty(t, wnrs)
			} else {
				assert.Error(t, err)
			}

			assert.Equal(t, testcase.isErrorNil, err == nil)
		})
	}
}

func Test_ValidationOfVolumeMounts_Errors(t *testing.T) {
	const (
		storageAccountName1 = "anystorageaccount"
		resourceGroup1      = "any-resource-group"
		subscriptionId1     = "any-subscription-id"
		tenantId1           = "any-tenant-id"
	)
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
		volumeMounts    volumeMountsFunc
		updateRA        []setVolumeMountsFunc
		expectedError   error
		expectedWarning string
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
			expectedError: radixapplication.ErrVolumeMountMissingName,
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
			expectedError: radixapplication.ErrVolumeMountMissingPath,
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
			expectedError: radixapplication.ErrVolumeMountDuplicateName,
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
			expectedError: radixapplication.ErrVolumeMountDuplicatePath,
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
			expectedError: radixapplication.ErrVolumeMountMissingType,
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
			updateRA:        setComponentAndJobsVolumeMounts,
			expectedError:   nil,
			expectedWarning: radixapplication.WarnDeprecatedFieldVolumeMountTypeUsed,
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
			updateRA:        setComponentAndJobsVolumeMounts,
			expectedError:   radixapplication.ErrVolumeMountMissingStorage,
			expectedWarning: radixapplication.WarnDeprecatedFieldVolumeMountTypeUsed,
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
			updateRA:        setComponentAndJobsVolumeMounts,
			expectedError:   radixapplication.ErrVolumeMountInvalidType,
			expectedWarning: radixapplication.WarnDeprecatedFieldVolumeMountTypeUsed,
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
			expectedError: radixapplication.ErrVolumeMountInvalidProtocol,
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
			expectedError: radixapplication.ErrVolumeMountMissingContainer,
		},
		"blobfuse2: has optional storage account": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
							Container:      "any-container",
							StorageAccount: storageAccountName1,
						},
					},
				}
				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: nil,
		},
		"blobfuse2: has invalid short storage account": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
							Container:      "any-container",
							StorageAccount: "st",
						},
					},
				}
				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixapplication.ErrVolumeMountInvalidStorageAccount,
		},
		"blobfuse2: has invalid long storage account": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
							Container:      "any-container",
							StorageAccount: strings.Repeat("a", 25),
						},
					},
				}
				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixapplication.ErrVolumeMountInvalidStorageAccount,
		},
		"blobfuse2: has invalid chars in storage account": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
							Container:      "any-container",
							StorageAccount: "name._-123",
						},
					},
				}
				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixapplication.ErrVolumeMountInvalidStorageAccount,
		},
		"blobfuse2 with useAzureIdentity: missing subscription": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
							Container:        "any-container",
							UseAzureIdentity: pointers.Ptr(true),
							StorageAccount:   storageAccountName1,
							ResourceGroup:    resourceGroup1,
							TenantId:         tenantId1,
						},
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixapplication.ErrVolumeMountWithUseAzureIdentityMissingSubscriptionId,
		},
		"blobfuse2 with useAzureIdentity: missing storage account name": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
							Container:        "any-container",
							UseAzureIdentity: pointers.Ptr(true),
							SubscriptionId:   subscriptionId1,
							ResourceGroup:    resourceGroup1,
							TenantId:         tenantId1,
						},
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixapplication.ErrVolumeMountWithUseAzureIdentityMissingStorageAccount,
		},
		"blobfuse2 with useAzureIdentity: missing resource group": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
							Container:        "any-container",
							UseAzureIdentity: pointers.Ptr(true),
							StorageAccount:   storageAccountName1,
							SubscriptionId:   subscriptionId1,
							TenantId:         tenantId1,
						},
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixapplication.ErrVolumeMountWithUseAzureIdentityMissingResourceGroup,
		},
		"blobfuse2 with useAzureIdentity: Tenant id is optional": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
							Container:        "any-container",
							UseAzureIdentity: pointers.Ptr(true),
							StorageAccount:   storageAccountName1,
							SubscriptionId:   subscriptionId1,
							ResourceGroup:    resourceGroup1,
						},
					},
				}

				return volumeMounts
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: nil,
		},
		"blobfuse2 with useAzureIdentity: component or job identity azure clientId is required": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				volumeMounts := []radixv1.RadixVolumeMount{
					{
						Name: "some_name",
						Path: "some_path",
						BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
							Container:        "any-container",
							UseAzureIdentity: pointers.Ptr(true),
							StorageAccount:   storageAccountName1,
							SubscriptionId:   subscriptionId1,
							ResourceGroup:    resourceGroup1,
						},
					},
				}

				return volumeMounts
			},
			updateRA: []setVolumeMountsFunc{
				func(ra *radixv1.RadixApplication, volumeMounts []radixv1.RadixVolumeMount) {
					ra.Spec.Components[0].Identity.Azure = nil
					ra.Spec.Components[0].EnvironmentConfig[0].Identity.Azure = nil
					ra.Spec.Components[0].EnvironmentConfig[0].VolumeMounts = volumeMounts
				},
				func(ra *radixv1.RadixApplication, volumeMounts []radixv1.RadixVolumeMount) {
					ra.Spec.Jobs[0].Identity.Azure = nil
					ra.Spec.Jobs[0].EnvironmentConfig[0].Identity.Azure = nil
					ra.Spec.Jobs[0].EnvironmentConfig[0].VolumeMounts = volumeMounts
				},
			},
			expectedError: radixapplication.ErrVolumeMountMissingAzureIdentity,
		},
		"blobfuse2.blockCache prefetchCount 0 is valid": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				return []radixv1.RadixVolumeMount{{
					Name: "anyname",
					Path: "anypath",
					BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
						Container: "anycontainer",
						BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
							PrefetchCount: pointers.Ptr[uint32](0),
						},
					},
				}}
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: nil,
		},
		"blobfuse2.blockCache prefetchCount 11 is valid": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				return []radixv1.RadixVolumeMount{{
					Name: "anyname",
					Path: "anypath",
					BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
						Container: "anycontainer",
						BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
							PrefetchCount: pointers.Ptr[uint32](11),
						},
					},
				}}
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: nil,
		},
		"blobfuse2.blockCache prefetchCount 1 is invalid": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				return []radixv1.RadixVolumeMount{{
					Name: "anyname",
					Path: "anypath",
					BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
						Container: "anycontainer",
						BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
							PrefetchCount: pointers.Ptr[uint32](1),
						},
					},
				}}
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixapplication.ErrInvalidBlobFuse2BlockCachePrefetchCount,
		},
		"blobfuse2.blockCache prefetchCount 10 is invalid": {
			volumeMounts: func() []radixv1.RadixVolumeMount {
				return []radixv1.RadixVolumeMount{{
					Name: "anyname",
					Path: "anypath",
					BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
						Container: "anycontainer",
						BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
							PrefetchCount: pointers.Ptr[uint32](10),
						},
					},
				}}
			},
			updateRA:      setComponentAndJobsVolumeMounts,
			expectedError: radixapplication.ErrInvalidBlobFuse2BlockCachePrefetchCount,
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
			expectedError: radixapplication.ErrVolumeMountMissingSizeLimit,
		},
	}

	client := createClient("testdata/radixregistration.yaml")
	for name, test := range testScenarios {
		t.Run(name, func(t *testing.T) {
			if len(test.updateRA) == 0 {
				assert.FailNow(t, "missing updateRA functions for %s", name)
				return
			}

			for _, ra := range test.updateRA {
				validRA := load[*radixv1.RadixApplication]("./testdata/radixconfig.yaml")
				volumes := test.volumeMounts()
				ra(validRA, volumes)
				validator := radixapplication.CreateOnlineValidator(client, []string{"grafana"}, map[string]string{"api": "radix-api"})
				wnrs, err := validator.Validate(context.Background(), validRA)
				if test.expectedError == nil {
					assert.NoError(t, err)
				} else {
					assert.ErrorIs(t, err, test.expectedError)
				}

				if test.expectedWarning == "" {
					assert.Empty(t, wnrs)
				} else {
					found := false
					for _, wrn := range wnrs {
						if strings.Contains(wrn, test.expectedWarning) {
							found = true
							break
						}
					}
					if !found {
						assert.Failf(t, "expected warning '%s' not found in '%s'", test.expectedWarning, strings.Join(wnrs, ", "))
					}
				}
			}
		})
	}
}

func Test_HorizontalScaling_Validation(t *testing.T) {
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
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().Build()
			},
			[]error{radixapplication.ErrMaxReplicasForHPANotSetOrZero},
		},
		{
			"component HPA maxReplicas is not set and minReplicas is set",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMinReplicas(3).
					Build()
			},
			[]error{radixapplication.ErrMinReplicasGreaterThanMaxReplicas},
		},
		{
			"component HPA minReplicas is not set and maxReplicas is set",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMaxReplicas(2).
					Build()
			},
			nil,
		},
		{
			"component HPA minReplicas is set to 0 but only CPU and Memory resoruces are set",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMinReplicas(0).
					WithMaxReplicas(2).
					WithCPUTrigger(80).
					WithMemoryTrigger(80).
					Build()
			},
			[]error{radixapplication.ErrInvalidMinimumReplicasConfigurationWithMemoryAndCPUTriggers},
		},
		{
			"component HPA minReplicas is set to 1 and only CPU and Memory resoruces are set",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMinReplicas(1).
					WithMaxReplicas(2).
					WithCPUTrigger(80).
					WithMemoryTrigger(80).
					Build()
			},
			nil,
		},
		{
			"component HPA minReplicas is greater than maxReplicas",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMinReplicas(3).
					WithMaxReplicas(2).
					Build()
			},
			[]error{radixapplication.ErrMinReplicasGreaterThanMaxReplicas},
		},
		{
			"component HPA maxReplicas is greater than minReplicas",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMinReplicas(3).
					WithMaxReplicas(4).
					Build()
			},
			nil,
		},
		{
			"Valid CPU trigger should be successfull",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMaxReplicas(4).
					WithCPUTrigger(radixv1.DefaultTargetCPUUtilizationPercentage).
					Build()
			},
			nil,
		},
		{
			"Invalid CPU trigger should fail",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMinReplicas(2).
					WithMaxReplicas(4).
					WithTrigger(radixv1.RadixHorizontalScalingTrigger{Cpu: &radixv1.RadixHorizontalScalingCPUTrigger{}}).
					Build()
			},
			[]error{radixapplication.ErrInvalidTriggerDefinition},
		},
		{
			"Valid Memory trigger should be successfull",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMaxReplicas(4).
					WithMemoryTrigger(80).
					Build()
			},
			nil,
		},
		{
			"Invalid Memory trigger should fail",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMinReplicas(2).
					WithMaxReplicas(4).
					WithTrigger(radixv1.RadixHorizontalScalingTrigger{Memory: &radixv1.RadixHorizontalScalingMemoryTrigger{}}).
					Build()
			},
			[]error{radixapplication.ErrInvalidTriggerDefinition},
		},
		{
			"Valid CRON trigger should be successfull",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMaxReplicas(4).
					WithCRONTrigger("* * * * *", "* * * * *", "Europe/Oslo", 10).
					Build()
			},
			nil,
		},
		{
			"Invalid CRON schedule must fail",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMaxReplicas(4).
					WithCRONTrigger("* * * *", "@hourly", "Europe/Oslo", 10).
					Build()
			},
			[]error{radixapplication.ErrInvalidTriggerDefinition},
		},
		{
			"Invalid CRON trigger should fail",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMaxReplicas(4).
					WithTrigger(radixv1.RadixHorizontalScalingTrigger{Name: "cron", Cron: &radixv1.RadixHorizontalScalingCronTrigger{}}).
					Build()
			},
			[]error{radixapplication.ErrInvalidTriggerDefinition},
		},
		{
			"Valid AzureServiceBus trigger should be successful with clientId",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMaxReplicas(4).
					WithAzureServiceBusTrigger("anamespace", "abcd", "queue-name", "", "", "", nil, nil).
					Build()
			},
			nil,
		},
		{
			"Valid AzureServiceBus trigger should be fail without connection string and clientId",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMaxReplicas(4).
					WithAzureServiceBusTrigger("anamespace", "", "queue-name", "", "", "", nil, nil).
					Build()
			},
			[]error{radixapplication.ErrInvalidTriggerDefinition},
		},
		{
			"Valid AzureServiceBus trigger should be successful with connection string",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMaxReplicas(4).
					WithAzureServiceBusTrigger("anamespace", "", "queue-name", "", "", "CONNECTION_STRING", nil, nil).
					Build()
			},
			nil,
		},
		{
			"Invalid AzureServiceBus trigger should fail",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMaxReplicas(4).
					WithAzureServiceBusTrigger("", "", "", "", "", "", nil, nil).
					Build()
			},
			[]error{radixapplication.ErrInvalidTriggerDefinition},
		},
		{
			"Valid AzureEventHub trigger should be successful with clientId",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMaxReplicas(4).
					WithAzureEventHubTrigger("abcd", &radixv1.RadixHorizontalScalingAzureEventHubTrigger{
						EventHubNamespace: "some-event-hub-namespace",
						EventHubName:      "some-event-hub-name",
						StorageAccount:    "some-storage-account",
						Container:         "some-container",
					}).
					Build()
			},
			nil,
		},
		{
			"Valid AzureEventHub trigger should be successful with clientId and attrs from env-vars",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMaxReplicas(4).
					WithAzureEventHubTrigger("abcd", &radixv1.RadixHorizontalScalingAzureEventHubTrigger{
						EventHubNamespaceFromEnv: "EVENT_HUB_NAMESPACE",
						EventHubNameFromEnv:      "EVENT_HUB_NAME",
						StorageAccount:           "some-storage-account",
						Container:                "some-container",
					}).
					Build()
			},
			nil,
		},
		{
			"Valid AzureEventHub trigger should be successful with no clientId",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMaxReplicas(4).
					WithAzureEventHubTrigger("", &radixv1.RadixHorizontalScalingAzureEventHubTrigger{
						EventHubConnectionFromEnv: "EVENT_HUB_CONNECTION",
						StorageConnectionFromEnv:  "STORAGE_CONNECTION",
						Container:                 "some-container",
					}).
					Build()
			},
			nil,
		},
		{
			"Invalid AzureEventHub trigger with no properties",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().WithMaxReplicas(4).
					WithAzureEventHubTrigger("", &radixv1.RadixHorizontalScalingAzureEventHubTrigger{}).
					Build()
			},
			[]error{radixapplication.ErrInvalidTriggerDefinition},
		},
		{
			"Invalid AzureEventHub trigger with no clientId and no event hub connection string",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().WithMaxReplicas(4).
					WithAzureEventHubTrigger("", &radixv1.RadixHorizontalScalingAzureEventHubTrigger{
						StorageConnectionFromEnv: "STORAGE_CONNECTION",
						Container:                "some-container",
					}).
					Build()
			},
			[]error{radixapplication.ErrInvalidTriggerDefinition},
		},
		{
			"Invalid AzureEventHub trigger with no clientId and no event storage connection string",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().WithMaxReplicas(4).
					WithAzureEventHubTrigger("", &radixv1.RadixHorizontalScalingAzureEventHubTrigger{
						EventHubConnectionFromEnv: "EVENT_HUB_CONNECTION",
						Container:                 "some-container",
					}).
					Build()
			},
			[]error{radixapplication.ErrInvalidTriggerDefinition},
		},
		{
			"Invalid AzureEventHub trigger with no clientId and no container name for blobMetadata checkpoint strategy",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().WithMaxReplicas(4).
					WithAzureEventHubTrigger("", &radixv1.RadixHorizontalScalingAzureEventHubTrigger{
						EventHubConnectionFromEnv: "EVENT_HUB_CONNECTION",
						StorageConnectionFromEnv:  "STORAGE_CONNECTION",
						CheckpointStrategy:        "blobMetadata",
					}).
					Build()
			},
			[]error{radixapplication.ErrInvalidTriggerDefinition},
		},
		{
			"Invalid AzureEventHub trigger with clientId and no container name for blobMetadata checkpoint strategy",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().WithMaxReplicas(4).
					WithAzureEventHubTrigger("", &radixv1.RadixHorizontalScalingAzureEventHubTrigger{
						EventHubNamespace:  "some-event-hub-namespace",
						EventHubName:       "some-event-hub-name",
						StorageAccount:     "some-storage-account",
						CheckpointStrategy: "blobMetadata",
					}).
					Build()
			},
			[]error{radixapplication.ErrInvalidTriggerDefinition},
		},
		{
			"Invalid AzureEventHub trigger with clientId and no storage account",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().WithMaxReplicas(4).
					WithAzureEventHubTrigger("", &radixv1.RadixHorizontalScalingAzureEventHubTrigger{
						EventHubNamespace: "some-event-hub-namespace",
						EventHubName:      "some-event-hub-name",
						Container:         "some-container",
					}).
					Build()
			},
			[]error{radixapplication.ErrInvalidTriggerDefinition},
		},
		{
			"Invalid AzureEventHub trigger with clientId and invalid strategy",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().WithMaxReplicas(4).
					WithAzureEventHubTrigger("", &radixv1.RadixHorizontalScalingAzureEventHubTrigger{
						EventHubNamespace:  "some-event-hub-namespace",
						EventHubName:       "some-event-hub-name",
						StorageAccount:     "some-storage-account",
						Container:          "some-container",
						CheckpointStrategy: "invalid-strategy",
					}).
					Build()
			},
			[]error{radixapplication.ErrInvalidTriggerDefinition},
		},
		{
			"Invalid AzureEventHub trigger with clientId and no event hub name",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().WithMaxReplicas(4).
					WithAzureEventHubTrigger("", &radixv1.RadixHorizontalScalingAzureEventHubTrigger{
						EventHubNamespace: "some-event-hub-namespace",
						StorageAccount:    "some-storage-account",
						Container:         "some-container",
					}).
					Build()
			},
			[]error{radixapplication.ErrInvalidTriggerDefinition},
		},
		{
			"Invalid AzureEventHub trigger with clientId and no event hub namespace",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().WithMaxReplicas(4).
					WithAzureEventHubTrigger("", &radixv1.RadixHorizontalScalingAzureEventHubTrigger{
						EventHubName:   "some-event-hub-name",
						StorageAccount: "some-storage-account",
						Container:      "some-container",
					}).
					Build()
			},
			[]error{radixapplication.ErrInvalidTriggerDefinition},
		},
		{
			"Invalid AzureEventHub trigger with clientId and invalid strategy",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().WithMaxReplicas(4).
					WithAzureEventHubTrigger("", &radixv1.RadixHorizontalScalingAzureEventHubTrigger{
						EventHubNamespace:  "some-event-hub-namespace",
						EventHubName:       "some-event-hub-name",
						StorageAccount:     "some-storage-account",
						Container:          "some-container",
						CheckpointStrategy: "invalid-strategy",
					}).
					Build()
			},
			[]error{radixapplication.ErrInvalidTriggerDefinition},
		},
		{
			"component HPA custom resource scaling for HPA is valid",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMinReplicas(2).
					WithMaxReplicas(4).
					WithCooldownPeriod(10).
					WithPollingInterval(10).
					WithMemoryTrigger(80).
					WithCPUTrigger(80).
					WithCRONTrigger("* * * * *", "* * * * *", "Europe/Oslo", 10).
					WithAzureServiceBusTrigger("anamespace", "abcd", "queue-name", "", "", "", nil, nil).
					Build()
			},
			nil,
		},
		{
			"environment HPA minReplicas is not set and maxReplicas is set",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMaxReplicas(4).
					Build()
			},
			nil,
		},
		{
			"invalid environment HPA is not valid",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMaxReplicas(0).
					Build()
			},
			[]error{radixapplication.ErrMinReplicasGreaterThanMaxReplicas},
		},
		{
			"Component with Trigger config and Resource config should fail",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = &radixv1.RadixHorizontalScaling{
					MinReplicas: pointers.Ptr(int32(2)),
					MaxReplicas: 4,
					RadixHorizontalScalingResources: &radixv1.RadixHorizontalScalingResources{
						Cpu:    &radixv1.RadixHorizontalScalingResource{AverageUtilization: pointers.Ptr(int32(80))},
						Memory: &radixv1.RadixHorizontalScalingResource{AverageUtilization: pointers.Ptr(int32(80))},
					},
					Triggers: []radixv1.RadixHorizontalScalingTrigger{
						{Name: "cpu", Cpu: &radixv1.RadixHorizontalScalingCPUTrigger{Value: 99}},
						{Name: "memory", Memory: &radixv1.RadixHorizontalScalingMemoryTrigger{Value: 99}},
					},
				}
			},
			[]error{radixapplication.ErrCombiningTriggersWithResourcesIsIllegal},
		},
		{
			"Component with 0 replicas is correct when combined with atleast 1 non-resource trigger",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMinReplicas(0).
					WithMaxReplicas(4).
					WithCPUTrigger(99).
					WithMemoryTrigger(99).
					WithCRONTrigger("0 5 * * *", "0 20 * * *", "Europe/Oslo", 1).
					Build()

			},
			nil,
		},
		{
			"Component with 0 replicas is invalid with only resource triggers",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMinReplicas(0).
					WithMaxReplicas(4).
					WithCPUTrigger(99).
					WithMemoryTrigger(99).
					Build()
			},
			[]error{radixapplication.ErrInvalidMinimumReplicasConfigurationWithMemoryAndCPUTriggers},
		},
		{
			"Component with multiple definitions in same trigger must fail",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMinReplicas(1).
					WithMaxReplicas(4).
					WithTrigger(radixv1.RadixHorizontalScalingTrigger{Name: "cpu", Cpu: &radixv1.RadixHorizontalScalingCPUTrigger{Value: 99}, Memory: &radixv1.RadixHorizontalScalingMemoryTrigger{Value: 99}}).
					Build()
			},
			[]error{radixapplication.ErrMoreThanOneDefinitionInTrigger},
		},
		{
			"Component with multiple definitions of same type is allowed",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMinReplicas(1).
					WithMaxReplicas(4).
					WithTrigger(radixv1.RadixHorizontalScalingTrigger{Name: "cpu", Cpu: &radixv1.RadixHorizontalScalingCPUTrigger{Value: 99}}).
					WithTrigger(radixv1.RadixHorizontalScalingTrigger{Name: "cpu2", Cpu: &radixv1.RadixHorizontalScalingCPUTrigger{Value: 99}}).
					Build()
			},
			nil,
		},
		{
			"Component must have unique name",
			func(ra *radixv1.RadixApplication) {
				ra.Spec.Components[0].EnvironmentConfig[0].HorizontalScaling = utils.NewHorizontalScalingBuilder().
					WithMinReplicas(1).
					WithMaxReplicas(4).
					WithTrigger(radixv1.RadixHorizontalScalingTrigger{Name: "test", Cpu: &radixv1.RadixHorizontalScalingCPUTrigger{Value: 99}}).
					WithTrigger(radixv1.RadixHorizontalScalingTrigger{Name: "test", Memory: &radixv1.RadixHorizontalScalingMemoryTrigger{Value: 99}}).
					Build()
			},
			[]error{radixapplication.ErrDuplicateTriggerName},
		},
	}

	client := createClient("testdata/radixregistration.yaml")
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := load[*radixv1.RadixApplication]("./testdata/radixconfig.yaml")
			testcase.updateRA(validRA)
			validator := radixapplication.CreateOnlineValidator(client, []string{"grafana"}, map[string]string{"api": "radix-api"})
			wnrs, err := validator.Validate(context.Background(), validRA)

			if testcase.isErrors == nil {
				assert.NoError(t, err)
				assert.Empty(t, wnrs)
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

	client := createClient("testdata/radixregistration.yaml")
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRA := load[*radixv1.RadixApplication]("./testdata/radixconfig.yaml")
			testcase.updateRA(validRA)
			validator := radixapplication.CreateOnlineValidator(client, []string{"grafana"}, map[string]string{"api": "radix-api"})
			wnrs, err := validator.Validate(context.Background(), validRA)

			if testcase.isValid {
				assert.NoError(t, err)
				assert.Empty(t, wnrs)
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
		{name: "Invalid webhook URL", expectedError: radixapplication.ErrInvalidWebhookUrl,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &radixv1.Notifications{Webhook: &invalidUrl}
			},
		},
		{name: "Invalid webhook URL in environment", expectedError: radixapplication.ErrInvalidWebhookUrl,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &radixv1.Notifications{Webhook: &invalidUrl}
			},
		},
		{name: "Not allowed scheme ftp", expectedError: radixapplication.ErrNotAllowedSchemeInWebhookUrl,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("ftp://api:8090")}
			},
		},
		{name: "Not allowed scheme ftp in environment", expectedError: radixapplication.ErrNotAllowedSchemeInWebhookUrl,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("ftp://api:8090")}
			},
		},
		{name: "missing port in the webhook", expectedError: radixapplication.ErrMissingPortInWebhookUrl,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://api/abc")}
			},
		},
		{name: "missing port in the webhook in environment", expectedError: radixapplication.ErrMissingPortInWebhookUrl,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://api/abc")}
			},
		},
		{name: "webhook can only reference to an application component or job", expectedError: radixapplication.ErrOnlyAppComponentAllowedInWebhookUrl,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://notexistingcomponent:8090/abc")}
			},
		},
		{name: "webhook can only reference to an application component or job in environment", expectedError: radixapplication.ErrOnlyAppComponentAllowedInWebhookUrl,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://notexistingcomponent:8090/abc")}
			},
		},
		{name: "webhook port does not exist in an application component", expectedError: radixapplication.ErrInvalidPortInWebhookUrl,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://api:8077")}
			},
		},
		{name: "webhook port does not exist in an application component in environment", expectedError: radixapplication.ErrInvalidPortInWebhookUrl,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://api:8077")}
			},
		},
		{name: "webhook port does not exist in an application job component", expectedError: radixapplication.ErrInvalidPortInWebhookUrl,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://job3:8077")}
			},
		},
		{name: "webhook port does not exist in an application job component in environment", expectedError: radixapplication.ErrInvalidPortInWebhookUrl,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://job3:8077")}
			},
		},
		{name: "not allowed to use in the webhook a public port of an application component", expectedError: radixapplication.ErrInvalidUseOfPublicPortInWebhookUrl,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://app:8080")}
			},
		},
		{name: "not allowed to use in the webhook a public port of an application component in environment", expectedError: radixapplication.ErrInvalidUseOfPublicPortInWebhookUrl,
			updateRa: func(ra *radixv1.RadixApplication) {
				ra.Spec.Jobs[0].EnvironmentConfig[0].Notifications = &radixv1.Notifications{Webhook: pointers.Ptr("http://app:8080")}
			},
		},
	}

	client := createClient("testdata/radixregistration.yaml")
	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			ra := load[*radixv1.RadixApplication]("./testdata/radixconfig.yaml")
			testcase.updateRa(ra)

			validator := radixapplication.CreateOnlineValidator(client, []string{"grafana"}, map[string]string{"api": "radix-api"})
			_, err := validator.Validate(context.Background(), ra)

			if testcase.expectedError == nil && err != nil {
				assert.Fail(t, "Not expected error %v", err)
				return
			}
			if err != nil {
				assert.NotNil(t, err)
				assert.True(t, errors.Is(err, testcase.expectedError), "Expected error is not contained in list of errors: %v", err)
				return
			}
			assert.Nil(t, testcase.expectedError)
			assert.Nil(t, err)
		})
	}
}

func Test_ValidateApplicationCanBeAppliedWithDNSAliases(t *testing.T) {
	const (
		raAppName         = "testapp"
		otherAppName      = "anyapp2"
		raEnv             = "test"
		raComponentName   = "app"
		someEnv           = "dev"
		someComponentName = "component-abc"
		alias1            = "alias1"
		alias2            = "alias2"
		dnsZone           = "dev.radix.equinor.com"
	)
	var testScenarios = []struct {
		name                    string
		applicationBuilder      utils.ApplicationBuilder
		expectedValidationError error
	}{
		{
			name:                    "No dns aliases",
			applicationBuilder:      utils.ARadixApplication().WithAppName("testapp"),
			expectedValidationError: nil,
		},
		{
			name:                    "Added dns aliases",
			applicationBuilder:      utils.ARadixApplication().WithAppName("testapp").WithDNSAlias(radixv1.DNSAlias{Alias: alias1, Environment: raEnv, Component: raComponentName}),
			expectedValidationError: nil,
		},
		{
			name:                    "Existing dns aliases for the app",
			applicationBuilder:      utils.ARadixApplication().WithAppName("testapp").WithDNSAlias(radixv1.DNSAlias{Alias: alias1, Environment: raEnv, Component: raComponentName}),
			expectedValidationError: nil,
		},
		{
			name:                    "Existing dns aliases for the app and another app",
			applicationBuilder:      utils.ARadixApplication().WithAppName("testapp").WithDNSAlias(radixv1.DNSAlias{Alias: alias1, Environment: raEnv, Component: raComponentName}),
			expectedValidationError: nil,
		},
		{
			name:                    "Same alias exists in dns alias for another app",
			applicationBuilder:      utils.ARadixApplication().WithAppName("testapp").WithDNSAlias(radixv1.DNSAlias{Alias: alias2, Environment: raEnv, Component: raComponentName}),
			expectedValidationError: radixapplication.ErrDNSAliasAlreadyUsedByAnotherApplication,
		},
		{
			name:                    "Reserved alias api for another app",
			applicationBuilder:      utils.ARadixApplication().WithAppName("testapp").WithDNSAlias(radixv1.DNSAlias{Alias: "api", Environment: raEnv, Component: raComponentName}),
			expectedValidationError: radixapplication.ErrDNSAliasReservedForRadixPlatformApplication,
		},
		{
			name:                    "Reserved alias api for another service",
			applicationBuilder:      utils.ARadixApplication().WithAppName("testapp").WithDNSAlias(radixv1.DNSAlias{Alias: "grafana", Environment: raEnv, Component: raComponentName}),
			expectedValidationError: radixapplication.ErrDNSAliasReservedForRadixPlatformService,
		},
		{
			name:                    "Reserved alias api for this app",
			applicationBuilder:      utils.ARadixApplication().WithAppName("radix-api").WithDNSAlias(radixv1.DNSAlias{Alias: "api", Environment: raEnv, Component: raComponentName}),
			expectedValidationError: nil,
		},
	}

	for _, ts := range testScenarios {
		t.Run(ts.name, func(t *testing.T) {
			client := createClient(
				"testdata/radixregistration.yaml",
				"testdata/radixregistration.radix-api.yaml",
				"testdata/radixregistration.anyapp2.yaml",
				"testdata/dnsalias.anyapp1.yaml",
				"testdata/dnsalias.anyapp2.yaml",
			)

			ra := ts.applicationBuilder.BuildRA()

			validator := radixapplication.CreateOnlineValidator(client, []string{"grafana"}, map[string]string{"api": "radix-api"})
			wnrs, actualValidationErr := validator.Validate(context.Background(), ra)

			if ts.expectedValidationError == nil {
				assert.NoError(t, actualValidationErr)
				assert.Empty(t, wnrs)
			} else {
				assert.ErrorIs(t, actualValidationErr, ts.expectedValidationError)
			}
		})
	}
}

func createClient(initObjsFilenames ...string) client.Client {
	objs := []client.Object{}
	for _, filename := range initObjsFilenames {
		obj := load[client.Object](filename)
		objs = append(objs, obj)
	}

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func load[T client.Object](filename string) T {
	raw := struct {
		metav1.TypeMeta   `json:",inline"`
		metav1.ObjectMeta `json:"metadata,omitempty"`
	}{}

	configFileContent, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	// Important: Must use sigs.k8s.io/yaml decoder to correctly unmarshal Kubernetes objects.
	// This package supports encoding and decoding of yaml for CRD struct types using the json tag.
	// The gopkg.in/yaml.v3 package requires the yaml tag.
	err = yaml.Unmarshal(configFileContent, &raw)
	if err != nil {
		panic(err)
	}

	gvk := raw.GetObjectKind().GroupVersionKind()
	t, ok := scheme.AllKnownTypes()[gvk]
	if !ok {
		panic(fmt.Sprintf("scheme does not know GroupVersionKind %s", gvk.String()))
	}

	obj := reflect.New(t)
	objP := obj.Interface()
	err = yaml.Unmarshal(configFileContent, objP)
	if err != nil {
		panic(err)
	}

	return objP.(T)
}
