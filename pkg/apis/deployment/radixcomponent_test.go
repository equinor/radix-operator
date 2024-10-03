package deployment

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
)

type scenarioDef struct {
	name     string
	comp     interface{}
	env      interface{}
	expected interface{}
}

func TestGetAuthenticationForComponent(t *testing.T) {
	scenarios := []scenarioDef{
		{name: "should return nil when component and environment is nil"},
		{name: "should return component when environment is nil", comp: &radixv1.Authentication{}, expected: &radixv1.Authentication{}},
		{name: "should return environment when component is nil", env: &radixv1.Authentication{}, expected: &radixv1.Authentication{}},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			comp, _ := scenario.comp.(*radixv1.Authentication)
			env, _ := scenario.env.(*radixv1.Authentication)
			expected, _ := scenario.expected.(*radixv1.Authentication)
			actual, _ := GetAuthenticationForComponent(comp, env)
			assert.Equal(t, expected, actual)
		})

	}
}

func TestGetClientCertificateAuthenticationForComponent(t *testing.T) {
	verificationOptional := radixv1.VerificationTypeOptional
	verificationOff := radixv1.VerificationTypeOff

	scenarios := []scenarioDef{
		{name: "should return nil when component and environment is nil"},
		{name: "should return component when environment is nil", comp: &radixv1.ClientCertificate{}, expected: &radixv1.ClientCertificate{}},
		{name: "should return environment when component is nil", env: &radixv1.ClientCertificate{}, expected: &radixv1.ClientCertificate{}},
		{
			name:     "should use PassCertificateToUpstream from environment",
			comp:     &radixv1.ClientCertificate{PassCertificateToUpstream: pointers.Ptr(true)},
			env:      &radixv1.ClientCertificate{PassCertificateToUpstream: pointers.Ptr(false)},
			expected: &radixv1.ClientCertificate{PassCertificateToUpstream: pointers.Ptr(false)},
		},
		{
			name:     "should use PassCertificateToUpstream from environment",
			comp:     &radixv1.ClientCertificate{PassCertificateToUpstream: pointers.Ptr(false)},
			env:      &radixv1.ClientCertificate{PassCertificateToUpstream: pointers.Ptr(true)},
			expected: &radixv1.ClientCertificate{PassCertificateToUpstream: pointers.Ptr(true)},
		},
		{
			name:     "should use PassCertificateToUpstream from environment",
			comp:     &radixv1.ClientCertificate{},
			env:      &radixv1.ClientCertificate{PassCertificateToUpstream: pointers.Ptr(false)},
			expected: &radixv1.ClientCertificate{PassCertificateToUpstream: pointers.Ptr(false)},
		},
		{
			name:     "should use PassCertificateToUpstream from environment",
			comp:     &radixv1.ClientCertificate{},
			env:      &radixv1.ClientCertificate{PassCertificateToUpstream: pointers.Ptr(true)},
			expected: &radixv1.ClientCertificate{PassCertificateToUpstream: pointers.Ptr(true)},
		},
		{
			name:     "should use PassCertificateToUpstream from component when not set in environment",
			comp:     &radixv1.ClientCertificate{PassCertificateToUpstream: pointers.Ptr(false)},
			env:      &radixv1.ClientCertificate{},
			expected: &radixv1.ClientCertificate{PassCertificateToUpstream: pointers.Ptr(false)},
		},
		{
			name:     "should use PassCertificateToUpstream from component when not set in environment",
			comp:     &radixv1.ClientCertificate{PassCertificateToUpstream: pointers.Ptr(true)},
			env:      &radixv1.ClientCertificate{},
			expected: &radixv1.ClientCertificate{PassCertificateToUpstream: pointers.Ptr(true)},
		},
		{
			name:     "should use Verification from environment",
			comp:     &radixv1.ClientCertificate{Verification: &verificationOff},
			env:      &radixv1.ClientCertificate{Verification: &verificationOptional},
			expected: &radixv1.ClientCertificate{Verification: &verificationOptional},
		},
		{
			name:     "should use Verification from environment",
			comp:     &radixv1.ClientCertificate{},
			env:      &radixv1.ClientCertificate{Verification: &verificationOptional},
			expected: &radixv1.ClientCertificate{Verification: &verificationOptional},
		},
		{
			name:     "should use Verification from component",
			comp:     &radixv1.ClientCertificate{Verification: &verificationOff},
			env:      &radixv1.ClientCertificate{},
			expected: &radixv1.ClientCertificate{Verification: &verificationOff},
		},
		{
			name:     "should use Verification from component and PassCertificateToUpstream from environment",
			comp:     &radixv1.ClientCertificate{Verification: &verificationOff, PassCertificateToUpstream: pointers.Ptr(true)},
			env:      &radixv1.ClientCertificate{PassCertificateToUpstream: pointers.Ptr(false)},
			expected: &radixv1.ClientCertificate{Verification: &verificationOff, PassCertificateToUpstream: pointers.Ptr(false)},
		},
		{
			name:     "should use Verification from environment and PassCertificateToUpstream from component",
			comp:     &radixv1.ClientCertificate{Verification: &verificationOff, PassCertificateToUpstream: pointers.Ptr(true)},
			env:      &radixv1.ClientCertificate{Verification: &verificationOptional},
			expected: &radixv1.ClientCertificate{Verification: &verificationOptional, PassCertificateToUpstream: pointers.Ptr(true)},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			comp, _ := scenario.comp.(*radixv1.ClientCertificate)
			env, _ := scenario.env.(*radixv1.ClientCertificate)
			expected, _ := scenario.expected.(*radixv1.ClientCertificate)
			actual, _ := GetAuthenticationForComponent(&radixv1.Authentication{ClientCertificate: comp}, &radixv1.Authentication{ClientCertificate: env})
			assert.Equal(t, expected, actual.ClientCertificate)
		})

	}
}

func TestGetOAuth2AuthenticationForComponent(t *testing.T) {
	scenarios := []scenarioDef{
		{name: "should return nil when component and environment is nil"},
		{name: "should return component when environment is nil", comp: &radixv1.OAuth2{}, expected: &radixv1.OAuth2{}},
		{name: "should return environment when component is nil", env: &radixv1.OAuth2{}, expected: &radixv1.OAuth2{}},
		{
			name:     "should override OAuth2 from environment",
			comp:     &radixv1.OAuth2{ClientID: "123", Scope: "openid", SetXAuthRequestHeaders: pointers.Ptr(true), SessionStoreType: "cookie"},
			env:      &radixv1.OAuth2{Scope: "email", SetXAuthRequestHeaders: pointers.Ptr(false), SetAuthorizationHeader: pointers.Ptr(true), SessionStoreType: "redis"},
			expected: &radixv1.OAuth2{ClientID: "123", Scope: "email", SetXAuthRequestHeaders: pointers.Ptr(false), SetAuthorizationHeader: pointers.Ptr(true), SessionStoreType: "redis"},
		},
		{
			name:     "should override OAuth2.RedisStore from environment",
			comp:     &radixv1.OAuth2{RedisStore: &radixv1.OAuth2RedisStore{ConnectionURL: "foo"}},
			env:      &radixv1.OAuth2{RedisStore: &radixv1.OAuth2RedisStore{ConnectionURL: "bar"}},
			expected: &radixv1.OAuth2{RedisStore: &radixv1.OAuth2RedisStore{ConnectionURL: "bar"}},
		},
		{
			name:     "should override OAuth2.CookieStore from environment",
			comp:     &radixv1.OAuth2{CookieStore: &radixv1.OAuth2CookieStore{Minimal: pointers.Ptr(true)}},
			env:      &radixv1.OAuth2{CookieStore: &radixv1.OAuth2CookieStore{Minimal: pointers.Ptr(false)}},
			expected: &radixv1.OAuth2{CookieStore: &radixv1.OAuth2CookieStore{Minimal: pointers.Ptr(false)}},
		},
		{
			name:     "should override OAuth2.Cookie from environment",
			comp:     &radixv1.OAuth2{Cookie: &radixv1.OAuth2Cookie{Name: "oauth", Expire: "1h"}},
			env:      &radixv1.OAuth2{Cookie: &radixv1.OAuth2Cookie{Name: "_oauth", Refresh: "2h"}},
			expected: &radixv1.OAuth2{Cookie: &radixv1.OAuth2Cookie{Name: "_oauth", Expire: "1h", Refresh: "2h"}},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			comp, _ := scenario.comp.(*radixv1.OAuth2)
			env, _ := scenario.env.(*radixv1.OAuth2)
			expected, _ := scenario.expected.(*radixv1.OAuth2)
			actual, _ := GetAuthenticationForComponent(&radixv1.Authentication{OAuth2: comp}, &radixv1.Authentication{OAuth2: env})
			assert.Equal(t, expected, actual.OAuth2)
		})
	}
}

//nolint:staticcheck
func TestGetRadixComponentsForEnv_PublicPort_OldPublic(t *testing.T) {
	// New publicPort does not exist, old public does not exist
	componentName := "comp"
	env := "dev"
	anyImagePath := "imagepath"
	ra := utils.ARadixApplication().
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName(componentName).
				WithPort("http", 80).
				WithPort("https", 443)).BuildRA()

	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	deployComponent, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, env, componentImages, envVarsMap, nil)
	assert.Equal(t, ra.Spec.Components[0].PublicPort, deployComponent[0].PublicPort)
	//lint:ignore SA1019 backward compatibility test
	assert.Equal(t, ra.Spec.Components[0].Public, deployComponent[0].Public)
	assert.Equal(t, "", deployComponent[0].PublicPort)
	//lint:ignore SA1019 backward compatibility test
	assert.Equal(t, false, deployComponent[0].Public)

	// New publicPort exists, old public does not exist
	ra = utils.ARadixApplication().
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName(componentName).
				WithPort("http", 80).
				WithPort("https", 443).
				WithPublicPort("http")).BuildRA()
	deployComponent, _ = GetRadixComponentsForEnv(context.Background(), ra, nil, env, componentImages, envVarsMap, nil)
	assert.Equal(t, ra.Spec.Components[0].PublicPort, deployComponent[0].PublicPort)
	//lint:ignore SA1019 backward compatibility test
	assert.Equal(t, ra.Spec.Components[0].Public, deployComponent[0].Public)
	assert.Equal(t, "http", deployComponent[0].PublicPort)
	//lint:ignore SA1019 backward compatibility test
	assert.Equal(t, false, deployComponent[0].Public)

	// New publicPort exists, old public exists (ignored)
	ra = utils.ARadixApplication().
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName(componentName).
				WithPort("http", 80).
				WithPort("https", 443).
				WithPublicPort("http").
				WithPublic(true)).BuildRA()
	deployComponent, _ = GetRadixComponentsForEnv(context.Background(), ra, nil, env, componentImages, envVarsMap, nil)
	assert.Equal(t, ra.Spec.Components[0].PublicPort, deployComponent[0].PublicPort)
	//lint:ignore SA1019 backward compatibility test
	assert.NotEqual(t, ra.Spec.Components[0].Public, deployComponent[0].Public)
	assert.Equal(t, "http", deployComponent[0].PublicPort)
	//lint:ignore SA1019 backward compatibility test
	assert.Equal(t, false, deployComponent[0].Public)

	// New publicPort does not exist, old public exists (used)
	ra = utils.ARadixApplication().
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName(componentName).
				WithPort("http", 80).
				WithPort("https", 443).
				WithPublic(true)).BuildRA()
	deployComponent, _ = GetRadixComponentsForEnv(context.Background(), ra, nil, env, componentImages, envVarsMap, nil)
	assert.Equal(t, ra.Spec.Components[0].Ports[0].Name, deployComponent[0].PublicPort)
	//lint:ignore SA1019 backward compatibility test
	assert.NotEqual(t, ra.Spec.Components[0].Public, deployComponent[0].Public)
	//lint:ignore SA1019 backward compatibility test
	assert.Equal(t, false, deployComponent[0].Public)
}

func TestGetRadixComponentsForEnv_ReadOnlyFileSystem(t *testing.T) {
	componentName := "comp"
	env := "dev"
	anyImagePath := "imagepath"
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	// Test cases with different values for ReadOnlyFileSystem
	testCases := []struct {
		description           string
		readOnlyFileSystem    *bool
		readOnlyFileSystemEnv *bool

		expectedReadOnlyFilesystem *bool
	}{
		{"No configuration set", nil, nil, nil},
		{"Env controls when readOnlyFileSystem is nil, set to true", nil, pointers.Ptr(true), pointers.Ptr(true)},
		{"Env controls when readOnlyFileSystem is nil, set to false", nil, pointers.Ptr(false), pointers.Ptr(false)},
		{"readOnlyFileSystem set to true, no env config", pointers.Ptr(true), nil, pointers.Ptr(true)},
		{"Both readOnlyFileSystem and monitoringEnv set to true", pointers.Ptr(true), pointers.Ptr(true), pointers.Ptr(true)},
		{"Env overrides to false when both is set", pointers.Ptr(true), pointers.Ptr(false), pointers.Ptr(false)},
		{"readOnlyFileSystem set to false, no env config", pointers.Ptr(false), nil, pointers.Ptr(false)},
		{"Env overrides to true when both is set", pointers.Ptr(false), pointers.Ptr(true), pointers.Ptr(true)},
		{"Both readOnlyFileSystem and monitoringEnv set to false", pointers.Ptr(false), pointers.Ptr(false), pointers.Ptr(false)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			ra := utils.ARadixApplication().
				WithComponents(
					utils.NewApplicationComponentBuilder().
						WithName(componentName).
						WithReadOnlyFileSystem(testCase.readOnlyFileSystem).
						WithEnvironmentConfigs(
							utils.AnEnvironmentConfig().
								WithEnvironment(env).
								WithReadOnlyFileSystem(testCase.readOnlyFileSystemEnv),
							utils.AnEnvironmentConfig().
								WithEnvironment("prod").WithReadOnlyFileSystem(pointers.Ptr(false)),
						)).BuildRA()

			deployComponent, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, env, componentImages, envVarsMap, nil)
			assert.Equal(t, testCase.expectedReadOnlyFilesystem, deployComponent[0].ReadOnlyFileSystem)
		})
	}
}

func TestGetRadixComponentsForEnv_ListOfExternalAliasesForComponent_GetListOfAliases(t *testing.T) {
	anyImagePath := "imagepath"
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	ra := utils.ARadixApplication().
		WithEnvironment("prod", "release").
		WithEnvironment("dev", "master").
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName("componentA"),
			utils.NewApplicationComponentBuilder().
				WithName("componentB")).
		WithDNSExternalAlias("some.alias.com", "prod", "componentA", true).
		WithDNSExternalAlias("another.alias.com", "prod", "componentA", false).
		WithDNSExternalAlias("athird.alias.com", "prod", "componentB", false).BuildRA()

	deployComponent, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, "prod", componentImages, envVarsMap, nil)
	assert.Equal(t, 2, len(deployComponent))
	assert.Len(t, deployComponent, 2)
	assert.ElementsMatch(t, []radixv1.RadixDeployExternalDNS{{FQDN: "some.alias.com", UseCertificateAutomation: true}, {FQDN: "another.alias.com", UseCertificateAutomation: false}}, deployComponent[0].ExternalDNS)
	assert.ElementsMatch(t, []radixv1.RadixDeployExternalDNS{{FQDN: "athird.alias.com", UseCertificateAutomation: false}}, deployComponent[1].ExternalDNS)

	deployComponent, _ = GetRadixComponentsForEnv(context.Background(), ra, nil, "dev", componentImages, envVarsMap, nil)
	assert.Equal(t, 2, len(deployComponent))
	assert.Len(t, deployComponent[0].ExternalDNS, 0)
}

func TestGetRadixComponentsForEnv_CommonEnvironmentVariables_No_Override(t *testing.T) {
	anyImagePath := "imagepath"
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	ra := utils.ARadixApplication().
		WithEnvironment("prod", "release").
		WithEnvironment("dev", "master").
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName("comp_1").
				WithCommonEnvironmentVariable("ENV_COMMON_1", "environment_common_1").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithEnvironmentVariable("ENV_1", "environment_1"),
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithEnvironmentVariable("ENV_2", "environment_2")),
			utils.NewApplicationComponentBuilder().
				WithName("comp_2").
				WithCommonEnvironmentVariable("ENV_COMMON_2", "environment_common_2").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithEnvironmentVariable("ENV_3", "environment_3"),
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithEnvironmentVariable("ENV_4", "environment_4"))).BuildRA()

	deployComponentProd, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, "prod", componentImages, envVarsMap, nil)
	assert.Equal(t, 2, len(deployComponentProd))

	assert.Equal(t, "comp_1", deployComponentProd[0].Name)
	assert.Equal(t, 4, len(deployComponentProd[0].EnvironmentVariables))
	assert.Equal(t, "environment_1", deployComponentProd[0].EnvironmentVariables["ENV_1"])
	assert.Equal(t, "environment_common_1", deployComponentProd[0].EnvironmentVariables["ENV_COMMON_1"])

	assert.Equal(t, "comp_2", deployComponentProd[1].Name)
	assert.Equal(t, 4, len(deployComponentProd[1].EnvironmentVariables))
	assert.Equal(t, "environment_3", deployComponentProd[1].EnvironmentVariables["ENV_3"])
	assert.Equal(t, "environment_common_2", deployComponentProd[1].EnvironmentVariables["ENV_COMMON_2"])

	deployComponentDev, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, "dev", componentImages, envVarsMap, nil)
	assert.Equal(t, 2, len(deployComponentDev))

	assert.Equal(t, "comp_1", deployComponentDev[0].Name)
	assert.Equal(t, 4, len(deployComponentDev[0].EnvironmentVariables))
	assert.Equal(t, "environment_2", deployComponentDev[0].EnvironmentVariables["ENV_2"])
	assert.Equal(t, "environment_common_1", deployComponentDev[0].EnvironmentVariables["ENV_COMMON_1"])

	assert.Equal(t, "comp_2", deployComponentDev[1].Name)
	assert.Equal(t, 4, len(deployComponentDev[1].EnvironmentVariables))
	assert.Equal(t, "environment_4", deployComponentDev[1].EnvironmentVariables["ENV_4"])
	assert.Equal(t, "environment_common_2", deployComponentDev[1].EnvironmentVariables["ENV_COMMON_2"])
}

func TestGetRadixComponentsForEnv_CommonEnvironmentVariables_With_Override(t *testing.T) {
	anyImagePath := "imagepath"
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	ra := utils.ARadixApplication().
		WithEnvironment("prod", "release").
		WithEnvironment("dev", "master").
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName("comp_1").
				WithCommonEnvironmentVariable("ENV_COMMON_1", "environment_common_1").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithEnvironmentVariable("ENV_1", "environment_1").
						WithEnvironmentVariable("ENV_COMMON_1", "environment_common_1_prod_override"),
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithEnvironmentVariable("ENV_2", "environment_2").
						WithEnvironmentVariable("ENV_COMMON_1", "environment_common_1_dev_override")),

			utils.NewApplicationComponentBuilder().
				WithName("comp_2").
				WithCommonEnvironmentVariable("ENV_COMMON_2", "environment_common_2").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithEnvironmentVariable("ENV_3", "environment_3").
						WithEnvironmentVariable("ENV_COMMON_2", "environment_common_2_prod_override"),
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithEnvironmentVariable("ENV_4", "environment_4").
						WithEnvironmentVariable("ENV_COMMON_2", "environment_common_2_dev_override"))).BuildRA()

	deployComponentProd, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, "prod", componentImages, envVarsMap, nil)
	assert.Equal(t, 2, len(deployComponentProd))

	assert.Equal(t, "comp_1", deployComponentProd[0].Name)
	assert.Equal(t, 4, len(deployComponentProd[0].EnvironmentVariables))
	assert.Equal(t, "environment_1", deployComponentProd[0].EnvironmentVariables["ENV_1"])
	assert.Equal(t, "environment_common_1_prod_override", deployComponentProd[0].EnvironmentVariables["ENV_COMMON_1"])

	assert.Equal(t, "comp_2", deployComponentProd[1].Name)
	assert.Equal(t, 4, len(deployComponentProd[1].EnvironmentVariables))
	assert.Equal(t, "environment_3", deployComponentProd[1].EnvironmentVariables["ENV_3"])
	assert.Equal(t, "environment_common_2_prod_override", deployComponentProd[1].EnvironmentVariables["ENV_COMMON_2"])

	deployComponentDev, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, "dev", componentImages, envVarsMap, nil)
	assert.Equal(t, 2, len(deployComponentDev))

	assert.Equal(t, "comp_1", deployComponentDev[0].Name)
	assert.Equal(t, 4, len(deployComponentDev[0].EnvironmentVariables))
	assert.Equal(t, "environment_2", deployComponentDev[0].EnvironmentVariables["ENV_2"])
	assert.Equal(t, "environment_common_1_dev_override", deployComponentDev[0].EnvironmentVariables["ENV_COMMON_1"])

	assert.Equal(t, "comp_2", deployComponentDev[1].Name)
	assert.Equal(t, 4, len(deployComponentDev[1].EnvironmentVariables))
	assert.Equal(t, "environment_4", deployComponentDev[1].EnvironmentVariables["ENV_4"])
	assert.Equal(t, "environment_common_2_dev_override", deployComponentDev[1].EnvironmentVariables["ENV_COMMON_2"])
}

func TestGetRadixComponentsForEnv_CommonEnvironmentVariables_NilVariablesMapInEnvConfig(t *testing.T) {
	anyImagePath := "imagepath"
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	ra := utils.ARadixApplication().
		WithEnvironment("prod", "release").
		WithEnvironment("dev", "master").
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName("comp_1").
				WithCommonEnvironmentVariable("ENV_COMMON_1", "environment_common_1").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod"),
					utils.AnEnvironmentConfig().
						WithEnvironment("dev")),
			utils.NewApplicationComponentBuilder().
				WithName("comp_2").
				WithCommonEnvironmentVariable("ENV_COMMON_2", "environment_common_2").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod"),
					utils.AnEnvironmentConfig().
						WithEnvironment("dev"))).BuildRA()

	deployComponentProd, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, "prod", componentImages, envVarsMap, nil)
	assert.Equal(t, 2, len(deployComponentProd))

	assert.Equal(t, "comp_1", deployComponentProd[0].Name)
	assert.Equal(t, 3, len(deployComponentProd[0].EnvironmentVariables))
	assert.Equal(t, "environment_common_1", deployComponentProd[0].EnvironmentVariables["ENV_COMMON_1"])

	assert.Equal(t, "comp_2", deployComponentProd[1].Name)
	assert.Equal(t, 3, len(deployComponentProd[1].EnvironmentVariables))
	assert.Equal(t, "environment_common_2", deployComponentProd[1].EnvironmentVariables["ENV_COMMON_2"])

	deployComponentDev, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, "dev", componentImages, envVarsMap, nil)
	assert.Equal(t, 2, len(deployComponentDev))

	assert.Equal(t, "comp_1", deployComponentDev[0].Name)
	assert.Equal(t, 3, len(deployComponentDev[0].EnvironmentVariables))
	assert.Equal(t, "environment_common_1", deployComponentDev[0].EnvironmentVariables["ENV_COMMON_1"])

	assert.Equal(t, "comp_2", deployComponentDev[1].Name)
	assert.Equal(t, 3, len(deployComponentDev[1].EnvironmentVariables))
	assert.Equal(t, "environment_common_2", deployComponentDev[1].EnvironmentVariables["ENV_COMMON_2"])
}

func TestGetRadixComponentsForEnv_Monitoring(t *testing.T) {
	envs := [2]string{"prod", "dev"}
	anyImagePath := "imagepath"
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	monitoringConfig := radixv1.MonitoringConfig{
		PortName: "monitor",
		Path:     "/api/monitor",
	}

	radApp := utils.ARadixApplication().
		WithEnvironment(envs[0], "release").
		WithEnvironment(envs[1], "master").
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName("comp_1").
				WithMonitoringConfig(monitoringConfig).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment(envs[0]).
						WithMonitoring(pointers.Ptr(true)),
					utils.AnEnvironmentConfig().
						WithEnvironment(envs[1]),
				),
			utils.NewApplicationComponentBuilder().
				WithName("comp_2").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment(envs[0]),
					utils.AnEnvironmentConfig().
						WithEnvironment(envs[1]).
						WithMonitoring(pointers.Ptr(true)),
				),
		).BuildRA()

	// check component(s) env
	comps, err := GetRadixComponentsForEnv(context.Background(), radApp, nil, envs[0], componentImages, envVarsMap, nil)
	assert.Nil(t, err)
	assert.True(t, comps[0].Monitoring)
	assert.Equal(t, monitoringConfig.PortName, comps[0].MonitoringConfig.PortName)
	assert.Equal(t, monitoringConfig.Path, comps[0].MonitoringConfig.Path)
	assert.False(t, comps[1].Monitoring)
	assert.Empty(t, comps[1].MonitoringConfig.PortName)
	assert.Empty(t, comps[1].MonitoringConfig.Path)

	// check other component(s) env
	comps, err = GetRadixComponentsForEnv(context.Background(), radApp, nil, envs[1], componentImages, envVarsMap, nil)
	assert.Nil(t, err)
	assert.False(t, comps[0].Monitoring)
	assert.Equal(t, monitoringConfig.PortName, comps[0].MonitoringConfig.PortName)
	assert.Equal(t, monitoringConfig.Path, comps[0].MonitoringConfig.Path)
	assert.True(t, comps[1].Monitoring)
	assert.Empty(t, comps[1].MonitoringConfig.PortName)
	assert.Empty(t, comps[1].MonitoringConfig.Path)
}

func TestGetRadixComponentsForEnv_CommonResources(t *testing.T) {
	anyImagePath := "imagepath"
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	ra := utils.ARadixApplication().
		WithEnvironment("prod", "release").
		WithEnvironment("dev", "master").
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName("comp_1").
				WithCommonResource(map[string]string{
					"memory": "64Mi",
					"cpu":    "250m",
				}, map[string]string{
					"memory": "128Mi",
					"cpu":    "500m",
				}).WithEnvironmentConfigs(
				utils.AnEnvironmentConfig().
					WithEnvironment("prod").
					WithResource(map[string]string{
						"memory": "128Mi",
						"cpu":    "500m",
					}, map[string]string{
						"memory": "256Mi",
						"cpu":    "750m",
					}))).BuildRA()

	deployComponentProd, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, "prod", componentImages, envVarsMap, nil)
	assert.Equal(t, 1, len(deployComponentProd))
	assert.Equal(t, "comp_1", deployComponentProd[0].Name)
	assert.Equal(t, "500m", deployComponentProd[0].Resources.Requests["cpu"])
	assert.Equal(t, "128Mi", deployComponentProd[0].Resources.Requests["memory"])
	assert.Equal(t, "750m", deployComponentProd[0].Resources.Limits["cpu"])
	assert.Equal(t, "256Mi", deployComponentProd[0].Resources.Limits["memory"])

	deployComponentDev, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, "dev", componentImages, envVarsMap, nil)
	assert.Equal(t, 1, len(deployComponentDev))
	assert.Equal(t, "comp_1", deployComponentDev[0].Name)
	assert.Equal(t, "250m", deployComponentDev[0].Resources.Requests["cpu"])
	assert.Equal(t, "64Mi", deployComponentDev[0].Resources.Requests["memory"])
	assert.Equal(t, "500m", deployComponentDev[0].Resources.Limits["cpu"])
	assert.Equal(t, "128Mi", deployComponentDev[0].Resources.Limits["memory"])
}

func Test_GetRadixComponents_NodeName(t *testing.T) {
	anyImagePath := "imagepath"
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"
	compGpu := "comp gpu"
	compGpuCount := "10"
	envGpu1 := "env1 gpu"
	envGpuCount1 := "20"
	envGpuCount2 := "30"
	envGpu3 := "env3 gpu"
	ra := utils.ARadixApplication().
		WithComponents(
			utils.AnApplicationComponent().
				WithName("comp").
				WithNode(radixv1.RadixNode{Gpu: compGpu, GpuCount: compGpuCount}).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("env1").
						WithNode(radixv1.RadixNode{Gpu: envGpu1, GpuCount: envGpuCount1}),
					utils.AnEnvironmentConfig().
						WithEnvironment("env2").
						WithNode(radixv1.RadixNode{GpuCount: envGpuCount2}),
					utils.AnEnvironmentConfig().
						WithEnvironment("env3").
						WithNode(radixv1.RadixNode{Gpu: envGpu3}),
					utils.AnEnvironmentConfig().
						WithEnvironment("env4"),
				),
		).BuildRA()

	t.Run("override job gpu and gpu-count with environment gpu and gpu-count", func(t *testing.T) {
		t.Parallel()
		deployComponent, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, "env1", componentImages, envVarsMap, nil)
		assert.Equal(t, envGpu1, deployComponent[0].Node.Gpu)
		assert.Equal(t, envGpuCount1, deployComponent[0].Node.GpuCount)
	})
	t.Run("override job gpu-count with environment gpu-count", func(t *testing.T) {
		t.Parallel()
		deployComponent, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, "env2", componentImages, envVarsMap, nil)
		assert.Equal(t, compGpu, deployComponent[0].Node.Gpu)
		assert.Equal(t, envGpuCount2, deployComponent[0].Node.GpuCount)
	})
	t.Run("override job gpu with environment gpu", func(t *testing.T) {
		t.Parallel()
		deployComponent, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, "env3", componentImages, envVarsMap, nil)
		assert.Equal(t, envGpu3, deployComponent[0].Node.Gpu)
		assert.Equal(t, compGpuCount, deployComponent[0].Node.GpuCount)
	})
	t.Run("do not override job gpu or gpu-count with environment gpu or gpu-count", func(t *testing.T) {
		t.Parallel()
		deployComponent, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, "env4", componentImages, envVarsMap, nil)
		assert.Equal(t, compGpu, deployComponent[0].Node.Gpu)
		assert.Equal(t, compGpuCount, deployComponent[0].Node.GpuCount)
	})
}

func TestGetRadixComponentsForEnv_ReturnsOnlyNotDisabledComponents(t *testing.T) {
	anyImagePath := "imagepath"
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	ra := utils.ARadixApplication().
		WithEnvironment("prod", "release").
		WithEnvironment("dev", "master").
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName("comp_1"),
			utils.NewApplicationComponentBuilder().
				WithName("comp_2").WithEnabled(true),
			utils.NewApplicationComponentBuilder().
				WithName("comp_3").WithEnabled(false),
			utils.NewApplicationComponentBuilder().
				WithName("comp_4").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod")),
			utils.NewApplicationComponentBuilder().
				WithName("comp_5").WithEnabled(true).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod")),
			utils.NewApplicationComponentBuilder().
				WithName("comp_6").WithEnabled(false).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod")),
			utils.NewApplicationComponentBuilder().
				WithName("comp_7").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").WithEnabled(true)),
			utils.NewApplicationComponentBuilder().
				WithName("comp_8").WithEnabled(true).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").WithEnabled(false)),
			utils.NewApplicationComponentBuilder().
				WithName("comp_9").WithEnabled(false).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").WithEnabled(true)),
			utils.NewApplicationComponentBuilder().
				WithName("comp_10").WithEnabled(false).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").WithEnabled(false))).
		BuildRA()

	deployComponentProd, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, "prod", componentImages, envVarsMap, nil)
	nameSet := convertRadixDeployComponentToNameSet(deployComponentProd)
	assert.NotEmpty(t, nameSet["comp_1"])
	assert.NotEmpty(t, nameSet["comp_2"])
	assert.Empty(t, nameSet["comp_3"])
	assert.NotEmpty(t, nameSet["comp_4"])
	assert.NotEmpty(t, nameSet["comp_5"])
	assert.Empty(t, nameSet["comp_6"])
	assert.NotEmpty(t, nameSet["comp_7"])
	assert.Empty(t, nameSet["comp_8"])
	assert.NotEmpty(t, nameSet["comp_9"])
	assert.Empty(t, nameSet["comp_10"])
}

func TestGetRadixComponentsForEnv_ReturnsOnlyNotDisabledJobComponents(t *testing.T) {
	anyImagePath := "imagepath"
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	ra := utils.ARadixApplication().
		WithEnvironment("prod", "release").
		WithEnvironment("dev", "master").
		WithJobComponents(
			utils.NewApplicationJobComponentBuilder().
				WithName("job_1"),
			utils.NewApplicationJobComponentBuilder().
				WithName("job_2").WithEnabled(true),
			utils.NewApplicationJobComponentBuilder().
				WithName("job_3").WithEnabled(false),
			utils.NewApplicationJobComponentBuilder().
				WithName("job_4").
				WithEnvironmentConfigs(
					utils.AJobComponentEnvironmentConfig().
						WithEnvironment("prod")),
			utils.NewApplicationJobComponentBuilder().
				WithName("job_5").WithEnabled(true).
				WithEnvironmentConfigs(
					utils.AJobComponentEnvironmentConfig().
						WithEnvironment("prod")),
			utils.NewApplicationJobComponentBuilder().
				WithName("job_6").WithEnabled(false).
				WithEnvironmentConfigs(
					utils.AJobComponentEnvironmentConfig().
						WithEnvironment("prod")),
			utils.NewApplicationJobComponentBuilder().
				WithName("job_7").
				WithEnvironmentConfigs(
					utils.AJobComponentEnvironmentConfig().
						WithEnvironment("prod").WithEnabled(true)),
			utils.NewApplicationJobComponentBuilder().
				WithName("job_8").WithEnabled(true).
				WithEnvironmentConfigs(
					utils.AJobComponentEnvironmentConfig().
						WithEnvironment("prod").WithEnabled(false)),
			utils.NewApplicationJobComponentBuilder().
				WithName("job_9").WithEnabled(false).
				WithEnvironmentConfigs(
					utils.AJobComponentEnvironmentConfig().
						WithEnvironment("prod").WithEnabled(true)),
			utils.NewApplicationJobComponentBuilder().
				WithName("job_10").WithEnabled(false).
				WithEnvironmentConfigs(
					utils.AJobComponentEnvironmentConfig().
						WithEnvironment("prod").WithEnabled(false))).
		BuildRA()

	builder := NewJobComponentsBuilder(ra, "prod", componentImages, envVarsMap, nil)
	deployComponentProd, err := builder.JobComponents(context.Background())
	require.NoError(t, err)
	nameSet := convertRadixDeployJobComponentsToNameSet(deployComponentProd)
	assert.NotEmpty(t, nameSet["job_1"])
	assert.NotEmpty(t, nameSet["job_2"])
	assert.Empty(t, nameSet["job_3"])
	assert.NotEmpty(t, nameSet["job_4"])
	assert.NotEmpty(t, nameSet["job_5"])
	assert.Empty(t, nameSet["job_6"])
	assert.NotEmpty(t, nameSet["job_7"])
	assert.Empty(t, nameSet["job_8"])
	assert.NotEmpty(t, nameSet["job_9"])
	assert.Empty(t, nameSet["job_10"])
}

func Test_GetRadixComponentsForEnv_Identity(t *testing.T) {
	type scenarioSpec struct {
		name                 string
		commonConfig         *radixv1.Identity
		configureEnvironment bool
		environmentConfig    *radixv1.Identity
		expected             *radixv1.Identity
	}

	scenarios := []scenarioSpec{
		{name: "nil when commonConfig and environmentConfig is empty", commonConfig: &radixv1.Identity{}, configureEnvironment: true, environmentConfig: &radixv1.Identity{}, expected: nil},
		{name: "nil when commonConfig is nil and environmentConfig is empty", commonConfig: nil, configureEnvironment: true, environmentConfig: &radixv1.Identity{}, expected: nil},
		{name: "nil when commonConfig is empty and environmentConfig is nil", commonConfig: &radixv1.Identity{}, configureEnvironment: true, environmentConfig: nil, expected: nil},
		{name: "nil when commonConfig is nil and environmentConfig is not set", commonConfig: nil, configureEnvironment: false, environmentConfig: nil, expected: nil},
		{name: "nil when commonConfig is empty and environmentConfig is not set", commonConfig: &radixv1.Identity{}, configureEnvironment: false, environmentConfig: nil, expected: nil},
		{name: "use commonConfig when environmentConfig is empty", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}, configureEnvironment: true, environmentConfig: &radixv1.Identity{}, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "use commonConfig when environmentConfig.Azure is empty", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}, configureEnvironment: true, environmentConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{}}, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "override non-empty commonConfig with environmentConfig.Azure", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}, configureEnvironment: true, environmentConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}},
		{name: "override empty commonConfig with environmentConfig", commonConfig: &radixv1.Identity{}, configureEnvironment: true, environmentConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}},
		{name: "override empty commonConfig.Azure with environmentConfig", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{}}, configureEnvironment: true, environmentConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}},
		{name: "transform clientId with curly to standard format", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "{11111111-2222-3333-4444-555555555555}"}}, configureEnvironment: false, environmentConfig: nil, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "transform clientId with urn:uuid to standard format", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "urn:uuid:11111111-2222-3333-4444-555555555555"}}, configureEnvironment: false, environmentConfig: nil, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "transform clientId without dashes to standard format", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111222233334444555555555555"}}, configureEnvironment: false, environmentConfig: nil, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			const envName = "anyenv"
			component := utils.AnApplicationComponent().WithName("anycomponent").WithIdentity(scenario.commonConfig)
			if scenario.configureEnvironment {
				component = component.WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().WithEnvironment(envName).WithIdentity(scenario.environmentConfig),
				)
			}
			ra := utils.ARadixApplication().WithComponents(component).BuildRA()
			components, err := GetRadixComponentsForEnv(context.Background(), ra, nil, envName, make(pipeline.DeployComponentImages), make(radixv1.EnvVarsMap), nil)
			require.NoError(t, err)
			assert.Equal(t, scenario.expected, components[0].Identity)
		})
	}
}

func TestGetRadixComponentsForEnv_ImageWithImageTagName(t *testing.T) {
	const (
		dynamicImageName1 = "custom-image-name1:{imageTagName}"
		dynamicImageName2 = "custom-image-name2:{imageTagName}"
		staticImageName1  = "custom-image-name1:latest"
		staticImageName2  = "custom-image-name2:latest"
		environment       = "dev"
	)
	type scenario struct {
		name                           string
		componentImages                map[string]string
		externalImageTagNames          map[string]string // map[component-name]image-tag
		componentImageTagNames         map[string]string // map[component-name]image-tag
		environmentConfigImageTagNames map[string]string // map[component-name]image-tag
		expectedComponentImage         map[string]string // map[component-name]image
		expectedError                  error
	}
	componentName1 := "componentA"
	componentName2 := "componentB"
	scenarios := []scenario{
		{
			name: "image has no tagName",
			componentImages: map[string]string{
				componentName1: staticImageName1,
				componentName2: staticImageName2,
			},
			expectedComponentImage: map[string]string{
				componentName1: staticImageName1,
				componentName2: staticImageName2,
			},
		},
		{
			name: "image has tagName, but no tags provided",
			componentImages: map[string]string{
				componentName1: dynamicImageName1,
				componentName2: staticImageName2,
			},
			expectedError: errorMissingExpectedDynamicImageTagName(componentName1),
		},
		{
			name: "with component image-tags",
			componentImages: map[string]string{
				componentName1: staticImageName1,
				componentName2: dynamicImageName2,
			},
			componentImageTagNames: map[string]string{
				componentName2: "tag-component-b",
			},
			environmentConfigImageTagNames: map[string]string{},
			expectedComponentImage: map[string]string{
				componentName1: staticImageName1,
				componentName2: "custom-image-name2:tag-component-b",
			},
		},
		{
			name: "with environment image-tags",
			componentImages: map[string]string{
				componentName1: staticImageName1,
				componentName2: dynamicImageName2,
			},
			environmentConfigImageTagNames: map[string]string{
				componentName2: "tag-component-env-b",
			},
			expectedComponentImage: map[string]string{
				componentName1: staticImageName1,
				componentName2: "custom-image-name2:tag-component-env-b",
			},
		},
		{
			name: "with environment overriding image-tags",
			componentImages: map[string]string{
				componentName1: staticImageName1,
				componentName2: dynamicImageName2,
			},
			componentImageTagNames: map[string]string{
				componentName2: "tag-component-b",
			},
			environmentConfigImageTagNames: map[string]string{
				componentName2: "tag-component-env-b",
			},
			expectedComponentImage: map[string]string{
				componentName1: staticImageName1,
				componentName2: "custom-image-name2:tag-component-env-b",
			},
		},
		{
			name: "external image-tags is used when missing component env imageTagName",
			componentImages: map[string]string{
				componentName1: staticImageName1,
				componentName2: dynamicImageName2,
			},
			componentImageTagNames: map[string]string{},
			externalImageTagNames: map[string]string{
				componentName2: "external-tag-component-b",
			},
			expectedComponentImage: map[string]string{
				componentName1: staticImageName1,
				componentName2: "custom-image-name2:external-tag-component-b",
			},
		},
	}

	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			componentImages := make(pipeline.DeployComponentImages)
			var componentBuilders []utils.RadixApplicationComponentBuilder
			for _, componentName := range []string{componentName1, componentName2} {
				componentImages[componentName] = pipeline.DeployComponentImage{ImagePath: ts.componentImages[componentName], ImageTagName: ts.externalImageTagNames[componentName]}
				componentBuilder := utils.NewApplicationComponentBuilder()
				componentBuilder.WithName(componentName).WithImage(ts.componentImages[componentName]).WithImageTagName(ts.componentImageTagNames[componentName]).
					WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment(environment).WithImageTagName(ts.environmentConfigImageTagNames[componentName]))
				componentBuilders = append(componentBuilders, componentBuilder)
			}

			ra := utils.ARadixApplication().WithEnvironment(environment, "master").WithComponents(componentBuilders...).BuildRA()

			deployComponents, err := GetRadixComponentsForEnv(context.Background(), ra, nil, environment, componentImages, make(radixv1.EnvVarsMap), nil)
			if err != nil && ts.expectedError == nil {
				assert.Fail(t, fmt.Sprintf("unexpected error %v", err))
				return
			}
			if err == nil && ts.expectedError != nil {
				assert.Fail(t, fmt.Sprintf("missing an expected error %s", ts.expectedError))
				return
			}
			if err != nil && err.Error() != ts.expectedError.Error() {
				assert.Fail(t, fmt.Sprintf("expected error '%s', but got '%s'", ts.expectedError, err.Error()))
				return
			}
			if ts.expectedError != nil {
				assert.Error(t, err)
				return
			}

			assert.Equal(t, 2, len(deployComponents))
			assert.Equal(t, ts.expectedComponentImage[deployComponents[0].Name], deployComponents[0].Image)
		})
	}
}

func Test_GetRadixComponents_Monitoring(t *testing.T) {
	componentName := "comp"
	env := "dev"
	anyImagePath := "imagepath"
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	testCases := []struct {
		description   string
		monitoring    *bool
		monitoringEnv *bool

		expectedMonitoring bool
	}{
		{"No configuration set", nil, nil, false},
		{"Env controls when monitoring is nil, set to true", nil, pointers.Ptr(true), true},
		{"Env controls when monitoring is nil, set to false", nil, pointers.Ptr(false), false},
		{"monitoring set to true, no env config", pointers.Ptr(true), nil, true},
		{"Both monitoring and monitoringEnv set to true", pointers.Ptr(true), pointers.Ptr(true), true},
		{"Env overrides to false when both is set", pointers.Ptr(true), pointers.Ptr(false), false},
		{"monitoring set to false, no env config", pointers.Ptr(false), nil, false},
		{"Env overrides to true when both is set", pointers.Ptr(false), pointers.Ptr(true), true},
		{"Both monitoring and monitoringEnv set to false", pointers.Ptr(false), pointers.Ptr(false), false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			ra := utils.ARadixApplication().
				WithComponents(
					utils.NewApplicationComponentBuilder().
						WithName(componentName).
						WithMonitoring(testCase.monitoring).
						WithEnvironmentConfigs(
							utils.AnEnvironmentConfig().
								WithEnvironment(env).
								WithMonitoring(testCase.monitoringEnv),
							utils.AnEnvironmentConfig().
								WithEnvironment("prod").
								WithMonitoring(pointers.Ptr(false)),
						)).BuildRA()

			deployComponent, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, env, componentImages, envVarsMap, nil)
			assert.Equal(t, testCase.expectedMonitoring, deployComponent[0].Monitoring)
		})
	}
}

func Test_GetRadixComponents_ReplicasOverride(t *testing.T) {
	componentName := "comp"
	env := "dev"
	anyImagePath := "imagepath"
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	testCases := map[string]struct {
		replicas                 *int
		expectedReplicas         *int
		replicasOverride         *int
		expectedReplicasOverride *int
	}{
		"nil":           {nil, nil, nil, nil},
		"regular":       {pointers.Ptr(1), pointers.Ptr(1), nil, nil},
		"override":      {pointers.Ptr(1), pointers.Ptr(1), pointers.Ptr(2), pointers.Ptr(2)},
		"only_override": {nil, nil, pointers.Ptr(3), pointers.Ptr(3)},
	}

	for description, testCase := range testCases {
		t.Run(description, func(t *testing.T) {
			raBuilder := utils.ARadixApplication().
				WithComponents(
					utils.NewApplicationComponentBuilder().
						WithName(componentName).
						WithEnvironmentConfigs(
							utils.AnEnvironmentConfig().
								WithEnvironment(env).
								WithReplicas(testCase.replicas),
						))
			ra := raBuilder.BuildRA()

			activeRd := utils.NewDeploymentBuilder().
				WithRadixApplication(raBuilder).
				WithEnvironment("dev").
				WithComponents(
					utils.NewDeployComponentBuilder().WithName("comp").WithReplicasOverride(testCase.replicasOverride),
				).
				BuildRD()

			deployComponents, err := GetRadixComponentsForEnv(context.Background(), ra, activeRd, env, componentImages, envVarsMap, nil)
			require.NoError(t, err)
			require.Len(t, deployComponents, 1)

			if testCase.expectedReplicas == nil {
				assert.Nil(t, deployComponents[0].Replicas)
			} else {
				require.NotNil(t, deployComponents[0].Replicas)
				assert.Equal(t, *testCase.expectedReplicas, *deployComponents[0].Replicas)
			}

			if testCase.expectedReplicasOverride == nil {
				assert.Nil(t, deployComponents[0].ReplicasOverride)
			} else {
				require.NotNil(t, deployComponents[0].ReplicasOverride)
				assert.Equal(t, *testCase.expectedReplicasOverride, *deployComponents[0].ReplicasOverride)
			}
		})
	}
}

func Test_GetRadixComponents_HorizontalScaling(t *testing.T) {
	componentName := "comp"
	env := "dev"
	anyImagePath := "imagepath"
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	testCases := []struct {
		description                  string
		componentHorizontalScaling   *utils.HorizontalScalingBuilderStruct
		environmentHorizontalScaling *utils.HorizontalScalingBuilderStruct

		expectedHorizontalScaling *utils.HorizontalScalingBuilderStruct
	}{
		{description: "No configuration set"},
		{
			description:                "Component sets HorizontalScaling",
			componentHorizontalScaling: utils.NewHorizontalScalingBuilder().WithMinReplicas(2).WithMaxReplicas(10).WithCPUTrigger(80).WithMemoryTrigger(70),
			expectedHorizontalScaling:  utils.NewHorizontalScalingBuilder().WithMinReplicas(2).WithMaxReplicas(10).WithCPUTrigger(80).WithMemoryTrigger(70),
		},
		{
			description:                  "Env sets HorizontalScaling",
			environmentHorizontalScaling: utils.NewHorizontalScalingBuilder().WithMinReplicas(1).WithMaxReplicas(8).WithCPUTrigger(85).WithMemoryTrigger(75),
			expectedHorizontalScaling:    utils.NewHorizontalScalingBuilder().WithMinReplicas(1).WithMaxReplicas(8).WithCPUTrigger(85).WithMemoryTrigger(75),
		},
		{
			description:                  "Env overrides all the component sets HorizontalScaling",
			componentHorizontalScaling:   utils.NewHorizontalScalingBuilder().WithMinReplicas(2).WithMaxReplicas(10).WithCPUTrigger(80).WithMemoryTrigger(70),
			environmentHorizontalScaling: utils.NewHorizontalScalingBuilder().WithMinReplicas(1).WithMaxReplicas(8).WithCPUTrigger(85).WithMemoryTrigger(75),
			expectedHorizontalScaling:    utils.NewHorizontalScalingBuilder().WithMinReplicas(1).WithMaxReplicas(8).WithCPUTrigger(85).WithMemoryTrigger(75),
		},
		{
			description:                  "Env overrides triggers from component",
			componentHorizontalScaling:   utils.NewHorizontalScalingBuilder().WithMinReplicas(2).WithMaxReplicas(10).WithMemoryTrigger(70),
			environmentHorizontalScaling: utils.NewHorizontalScalingBuilder().WithMaxReplicas(8).WithCPUTrigger(85),
			expectedHorizontalScaling:    utils.NewHorizontalScalingBuilder().WithMinReplicas(2).WithMaxReplicas(8).WithCPUTrigger(85),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			environmentConfigBuilder := utils.AnEnvironmentConfig().WithEnvironment(env)
			if testCase.environmentHorizontalScaling != nil {
				hs := testCase.environmentHorizontalScaling
				environmentConfigBuilder = environmentConfigBuilder.WithHorizontalScaling(hs.Build())
			}
			componentBuilder := utils.NewApplicationComponentBuilder().
				WithName(componentName).
				WithEnvironmentConfigs(environmentConfigBuilder)
			if testCase.componentHorizontalScaling != nil {
				hs := testCase.componentHorizontalScaling
				componentBuilder = componentBuilder.WithHorizontalScaling(hs.Build())
			}

			ra := utils.ARadixApplication().WithComponents(componentBuilder).BuildRA()

			deployComponents, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, env, componentImages, envVarsMap, nil)
			deployComponent, exists := slice.FindFirst(deployComponents, func(component radixv1.RadixDeployComponent) bool {
				return component.Name == componentName
			})
			require.True(t, exists)
			assert.Equal(t, testCase.expectedHorizontalScaling.Build(), deployComponent.HorizontalScaling)
		})
	}
}

func Test_GetRadixComponents_HorizontalScalingMultipleEnvs(t *testing.T) {
	componentName := "comp"
	anyImagePath := "imagepath"
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"
	const (
		env1 = "env1"
		env2 = "env2"
	)

	testCases := []struct {
		description                  string
		componentHorizontalScaling   *utils.HorizontalScalingBuilderStruct
		environmentHorizontalScaling map[string]*utils.HorizontalScalingBuilderStruct

		expectedHorizontalScaling map[string]*utils.HorizontalScalingBuilderStruct
	}{
		{
			description:                "Component sets HorizontalScaling",
			componentHorizontalScaling: utils.NewHorizontalScalingBuilder().WithMinReplicas(2).WithMaxReplicas(10).WithCPUTrigger(80).WithMemoryTrigger(70),
			environmentHorizontalScaling: map[string]*utils.HorizontalScalingBuilderStruct{
				env1: utils.NewHorizontalScalingBuilder().WithMinReplicas(1).WithMaxReplicas(8).WithCPUTrigger(85).WithMemoryTrigger(75),
			},
			expectedHorizontalScaling: map[string]*utils.HorizontalScalingBuilderStruct{
				env1: utils.NewHorizontalScalingBuilder().WithMinReplicas(1).WithMaxReplicas(8).WithCPUTrigger(85).WithMemoryTrigger(75),
				env2: utils.NewHorizontalScalingBuilder().WithMinReplicas(2).WithMaxReplicas(10).WithCPUTrigger(80).WithMemoryTrigger(70),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			componentBuilder := utils.NewApplicationComponentBuilder().WithName(componentName)
			for envName, hs := range testCase.environmentHorizontalScaling {
				componentBuilder = componentBuilder.WithEnvironmentConfig(utils.AnEnvironmentConfig().WithEnvironment(envName).WithHorizontalScaling(hs.Build()))
			}
			if testCase.componentHorizontalScaling != nil {
				hs := testCase.componentHorizontalScaling
				componentBuilder = componentBuilder.WithHorizontalScaling(hs.Build())
			}

			ra := utils.ARadixApplication().WithEnvironment(env1, "").WithEnvironment(env2, "").WithComponent(componentBuilder).BuildRA()
			for _, envName := range []string{env1, env2} {
				deployComponents, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, envName, componentImages, envVarsMap, nil)
				deployComponent, exists := slice.FindFirst(deployComponents, func(component radixv1.RadixDeployComponent) bool {
					return component.Name == componentName
				})
				require.True(t, exists)
				assert.Equal(t, testCase.expectedHorizontalScaling[envName].Build(), deployComponent.HorizontalScaling)
			}
		})
	}
}

func Test_GetRadixComponents_VolumeMounts(t *testing.T) {
	componentName := "comp"
	env := "dev"
	anyImagePath := "imagepath"
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	const (
		path1          = "/home/path1"
		path2          = "/home/path2"
		container1     = "container1"
		container2     = "container2"
		user1000       = "1000"
		user2000       = "2000"
		group1100      = "1100"
		group2200      = "2200"
		skuStandardLRS = "Standard_LRS"
		skuStandardGRS = "Standard_GRS"
	)
	var (
		accessModeReadWriteMany         = strings.ToLower(string(corev1.ReadWriteMany))
		accessModeReadOnlyMany          = strings.ToLower(string(corev1.ReadOnlyMany))
		bindingModeImmediate            = strings.ToLower(string(storagev1.VolumeBindingImmediate))
		bindingModeWaitForFirstConsumer = strings.ToLower(string(storagev1.VolumeBindingWaitForFirstConsumer))
	)
	testCases := []struct {
		description             string
		componentVolumeMounts   []radixv1.RadixVolumeMount
		environmentVolumeMounts []radixv1.RadixVolumeMount
		expectedVolumeMounts    []radixv1.RadixVolumeMount
	}{
		{description: "No configuration set"},
		{
			description: "Component sets VolumeMounts for azure-blob",
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Path: path1, Storage: container1, UID: user1000, GID: group1100, SkuName: skuStandardLRS, AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Path: path1, Storage: container1, UID: user1000, GID: group1100, SkuName: skuStandardLRS, AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate},
			},
		},
		{
			description: "Component sets VolumeMounts for blobFuse2",
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path1, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container1, GID: group1100, UID: user1000, SkuName: skuStandardLRS, RequestsStorage: "1M", AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate, UseAdls: pointers.Ptr(true),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(true), BlockSize: pointers.Ptr[uint64](1), MaxBuffers: pointers.Ptr[uint64](2), BufferSize: pointers.Ptr[uint64](3), StreamCache: pointers.Ptr[uint64](4), MaxBlocksPerFile: pointers.Ptr[uint64](5)},
				}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path1, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container1, GID: group1100, UID: user1000, SkuName: skuStandardLRS, RequestsStorage: "1M", AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate, UseAdls: pointers.Ptr(true),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(true), BlockSize: pointers.Ptr[uint64](1), MaxBuffers: pointers.Ptr[uint64](2), BufferSize: pointers.Ptr[uint64](3), StreamCache: pointers.Ptr[uint64](4), MaxBlocksPerFile: pointers.Ptr[uint64](5)},
				}},
			},
		},
		{
			description: "Env sets VolumeMounts for azure-blob",
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Path: path2, Storage: container2, UID: user2000, GID: group2200, SkuName: skuStandardGRS, AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Path: path2, Storage: container2, UID: user2000, GID: group2200, SkuName: skuStandardGRS, AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer},
			},
		},
		{
			description: "Env sets VolumeMounts for blobFuse2",
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container2, GID: group2200, UID: user2000, SkuName: skuStandardGRS, RequestsStorage: "2M", AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer, UseAdls: pointers.Ptr(false),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), BlockSize: pointers.Ptr[uint64](11), MaxBuffers: pointers.Ptr[uint64](22), BufferSize: pointers.Ptr[uint64](33), StreamCache: pointers.Ptr[uint64](44), MaxBlocksPerFile: pointers.Ptr[uint64](55)},
				}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container2, GID: group2200, UID: user2000, SkuName: skuStandardGRS, RequestsStorage: "2M", AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer, UseAdls: pointers.Ptr(false),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), BlockSize: pointers.Ptr[uint64](11), MaxBuffers: pointers.Ptr[uint64](22), BufferSize: pointers.Ptr[uint64](33), StreamCache: pointers.Ptr[uint64](44), MaxBlocksPerFile: pointers.Ptr[uint64](55)},
				}},
			},
		},
		{
			description: "Env overrides component VolumeMounts for azure-blob",
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Path: path1, Storage: container1, UID: user1000, GID: group1100, SkuName: skuStandardLRS, AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Path: path2, Storage: container2, UID: user2000, GID: group2200, SkuName: skuStandardGRS, AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Path: path2, Storage: container2, UID: user2000, GID: group2200, SkuName: skuStandardGRS, AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer},
			},
		},
		{
			description: "Env overrides component VolumeMounts for blobFuse2",
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path1, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container1, GID: group1100, UID: user1000, SkuName: skuStandardLRS, RequestsStorage: "1M", AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate, UseAdls: pointers.Ptr(true),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(true), BlockSize: pointers.Ptr[uint64](1), MaxBuffers: pointers.Ptr[uint64](2), BufferSize: pointers.Ptr[uint64](3), StreamCache: pointers.Ptr[uint64](4), MaxBlocksPerFile: pointers.Ptr[uint64](5)},
				}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container2, GID: group2200, UID: user2000, SkuName: skuStandardGRS, RequestsStorage: "2M", AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer, UseAdls: pointers.Ptr(false),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), BlockSize: pointers.Ptr[uint64](11), MaxBuffers: pointers.Ptr[uint64](22), BufferSize: pointers.Ptr[uint64](33), StreamCache: pointers.Ptr[uint64](44), MaxBlocksPerFile: pointers.Ptr[uint64](55)},
				}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container2, GID: group2200, UID: user2000, SkuName: skuStandardGRS, RequestsStorage: "2M", AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer, UseAdls: pointers.Ptr(false),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), BlockSize: pointers.Ptr[uint64](11), MaxBuffers: pointers.Ptr[uint64](22), BufferSize: pointers.Ptr[uint64](33), StreamCache: pointers.Ptr[uint64](44), MaxBlocksPerFile: pointers.Ptr[uint64](55)},
				}},
			},
		},
		{
			description: "Env overrides and adds component VolumeMounts for blobFuse2",
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path1, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container1, UID: user1000, SkuName: skuStandardLRS, AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate,
					Streaming: &radixv1.RadixVolumeMountStreaming{BlockSize: pointers.Ptr[uint64](1), MaxBuffers: pointers.Ptr[uint64](2), BufferSize: pointers.Ptr[uint64](3), MaxBlocksPerFile: pointers.Ptr[uint64](5)},
				}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container2, GID: group2200, SkuName: skuStandardGRS, RequestsStorage: "2M", AccessMode: accessModeReadOnlyMany, UseAdls: pointers.Ptr(false),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), MaxBuffers: pointers.Ptr[uint64](22), StreamCache: pointers.Ptr[uint64](44)},
				}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container2, GID: group2200, UID: user1000, SkuName: skuStandardGRS, RequestsStorage: "2M", AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeImmediate, UseAdls: pointers.Ptr(false),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), BlockSize: pointers.Ptr[uint64](1), MaxBuffers: pointers.Ptr[uint64](22), BufferSize: pointers.Ptr[uint64](3), StreamCache: pointers.Ptr[uint64](44), MaxBlocksPerFile: pointers.Ptr[uint64](5)},
				}},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			environmentConfigBuilder := utils.AnEnvironmentConfig().WithEnvironment(env).WithVolumeMounts(testCase.environmentVolumeMounts)
			componentBuilder := utils.NewApplicationComponentBuilder().WithName(componentName).
				WithEnvironmentConfigs(environmentConfigBuilder).WithVolumeMounts(testCase.componentVolumeMounts)

			ra := utils.ARadixApplication().WithComponents(componentBuilder).BuildRA()

			deployComponents, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, env, componentImages, envVarsMap, nil)
			deployComponent, exists := slice.FindFirst(deployComponents, func(component radixv1.RadixDeployComponent) bool {
				return component.Name == componentName
			})
			require.True(t, exists)
			assert.Equal(t, testCase.expectedVolumeMounts, deployComponent.VolumeMounts)
		})
	}
}

func Test_GetRadixComponents_VolumeMounts_MultipleEnvs(t *testing.T) {
	componentName := "comp"
	anyImagePath := "imagepath"
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	const (
		path1          = "/home/path1"
		path2          = "/home/path2"
		container1     = "container1"
		container2     = "container2"
		user1000       = "1000"
		user2000       = "2000"
		group1100      = "1100"
		group2200      = "2200"
		skuStandardLRS = "Standard_LRS"
		skuStandardGRS = "Standard_GRS"
		env1           = "env1"
		env2           = "env2"
	)
	var (
		accessModeReadWriteMany         = strings.ToLower(string(corev1.ReadWriteMany))
		accessModeReadOnlyMany          = strings.ToLower(string(corev1.ReadOnlyMany))
		bindingModeImmediate            = strings.ToLower(string(storagev1.VolumeBindingImmediate))
		bindingModeWaitForFirstConsumer = strings.ToLower(string(storagev1.VolumeBindingWaitForFirstConsumer))
	)
	testCases := []struct {
		description             string
		componentVolumeMounts   []radixv1.RadixVolumeMount
		environmentVolumeMounts map[string][]radixv1.RadixVolumeMount
		expectedVolumeMounts    map[string][]radixv1.RadixVolumeMount
	}{
		{
			description: "Env overrides component VolumeMounts for blobFuse2",
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path1, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container1, GID: group1100, UID: user1000, SkuName: skuStandardLRS, RequestsStorage: "1M", AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate, UseAdls: pointers.Ptr(true),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(true), BlockSize: pointers.Ptr[uint64](1), MaxBuffers: pointers.Ptr[uint64](2), BufferSize: pointers.Ptr[uint64](3), StreamCache: pointers.Ptr[uint64](4), MaxBlocksPerFile: pointers.Ptr[uint64](5)},
				}},
			},
			environmentVolumeMounts: map[string][]radixv1.RadixVolumeMount{
				env1: {
					{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
						Container: container2, GID: group2200, UID: user2000, SkuName: skuStandardGRS, RequestsStorage: "2M", AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer, UseAdls: pointers.Ptr(false),
						Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), BlockSize: pointers.Ptr[uint64](11), MaxBuffers: pointers.Ptr[uint64](22), BufferSize: pointers.Ptr[uint64](33), StreamCache: pointers.Ptr[uint64](44), MaxBlocksPerFile: pointers.Ptr[uint64](55)},
					}}},
			},
			expectedVolumeMounts: map[string][]radixv1.RadixVolumeMount{
				env1: {
					{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
						Container: container2, GID: group2200, UID: user2000, SkuName: skuStandardGRS, RequestsStorage: "2M", AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer, UseAdls: pointers.Ptr(false),
						Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), BlockSize: pointers.Ptr[uint64](11), MaxBuffers: pointers.Ptr[uint64](22), BufferSize: pointers.Ptr[uint64](33), StreamCache: pointers.Ptr[uint64](44), MaxBlocksPerFile: pointers.Ptr[uint64](55)},
					}}},
				env2: {
					{Name: "storage1", Path: path1, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
						Container: container1, GID: group1100, UID: user1000, SkuName: skuStandardLRS, RequestsStorage: "1M", AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate, UseAdls: pointers.Ptr(true),
						Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(true), BlockSize: pointers.Ptr[uint64](1), MaxBuffers: pointers.Ptr[uint64](2), BufferSize: pointers.Ptr[uint64](3), StreamCache: pointers.Ptr[uint64](4), MaxBlocksPerFile: pointers.Ptr[uint64](5)},
					}}},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			componentBuilder := utils.NewApplicationComponentBuilder().WithName(componentName).WithVolumeMounts(testCase.componentVolumeMounts)
			for envName, volumeMounts := range testCase.environmentVolumeMounts {
				componentBuilder = componentBuilder.WithEnvironmentConfig(utils.AnEnvironmentConfig().WithEnvironment(envName).WithVolumeMounts(volumeMounts))
			}

			ra := utils.ARadixApplication().WithEnvironment(env1, "").WithEnvironment(env2, "").
				WithComponents(componentBuilder).BuildRA()

			for _, envName := range []string{env1, env2} {
				deployComponents, _ := GetRadixComponentsForEnv(context.Background(), ra, nil, envName, componentImages, envVarsMap, nil)
				deployComponent, exists := slice.FindFirst(deployComponents, func(component radixv1.RadixDeployComponent) bool {
					return component.Name == componentName
				})
				require.True(t, exists)
				assert.Equal(t, testCase.expectedVolumeMounts[envName], deployComponent.VolumeMounts)
			}
		})
	}
}

func Test_GetRadixComponentsForEnv_Runtime_AlwaysUseFromDeployComponentImages(t *testing.T) {
	componentBuilder := utils.NewApplicationComponentBuilder().
		WithName("anycomp").
		WithRuntime(&radixv1.Runtime{Architecture: "commonarch"}).
		WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().
			WithEnvironment("dev").
			WithRuntime(&radixv1.Runtime{Architecture: "devarch"}))

	ra := utils.ARadixApplication().
		WithEnvironmentNoBranch("dev").
		WithEnvironmentNoBranch("prod").
		WithComponents(componentBuilder).BuildRA()

	tests := map[string]struct {
		env             string
		deployImages    pipeline.DeployComponentImages
		expectedRuntime *radixv1.Runtime
	}{
		"dev:nil when deployImages is nil": {
			env:             "dev",
			deployImages:    nil,
			expectedRuntime: nil,
		},
		"dev:nil when comp not defined in deployImages": {
			env:             "dev",
			deployImages:    pipeline.DeployComponentImages{"othercomp": {Runtime: &radixv1.Runtime{Architecture: "othercomparch"}}},
			expectedRuntime: nil,
		},
		"dev:runtime from deployImage comp when defined": {
			env:             "dev",
			deployImages:    pipeline.DeployComponentImages{"anycomp": {Runtime: &radixv1.Runtime{Architecture: "anycomparch"}}},
			expectedRuntime: &radixv1.Runtime{Architecture: "anycomparch"},
		},
		"prod:nil when deployImages is nil": {
			env:             "prod",
			deployImages:    nil,
			expectedRuntime: nil,
		},
		"prod:nil when comp not defined in deployImages": {
			env:             "prod",
			deployImages:    pipeline.DeployComponentImages{"othercomp": {Runtime: &radixv1.Runtime{Architecture: "othercomparch"}}},
			expectedRuntime: nil,
		},
		"prod:runtime from deployImage comp when defined": {
			env:             "prod",
			deployImages:    pipeline.DeployComponentImages{"anycomp": {Runtime: &radixv1.Runtime{Architecture: "anycomparch"}}},
			expectedRuntime: &radixv1.Runtime{Architecture: "anycomparch"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			deployComponents, err := GetRadixComponentsForEnv(context.Background(), ra, nil, test.env, test.deployImages, make(radixv1.EnvVarsMap), nil)
			require.NoError(t, err)
			require.Len(t, deployComponents, 1)
			deployComponent := deployComponents[0]
			actualRuntime := deployComponent.Runtime
			if test.expectedRuntime == nil {
				assert.Nil(t, actualRuntime)
			} else {
				assert.Equal(t, test.expectedRuntime, actualRuntime)
			}
		})
	}
}

func Test_Test_GetRadixComponentsForEnv_NetworkIngressPublicConfig(t *testing.T) {
	exp2 := func(n int) int {
		return int(math.Exp2(float64(n)))
	}

	type setIngressFuncs []func(*radixv1.IngressPublic)
	// Defines a list of functions that will set a single component specific IngressPublic field value.
	// The field value that each function set must be different from the value set by the corresponding (same field)
	// function in setEnvCfg
	setCommonCfg := setIngressFuncs{
		func(cfg *radixv1.IngressPublic) {
			cfg.Allow = &[]radixv1.IPOrCIDR{radixv1.IPOrCIDR("10.10.10.10"), radixv1.IPOrCIDR("20.20.20.20")}
		},
		func(cfg *radixv1.IngressPublic) {
			cfg.ProxyBodySize = pointers.Ptr(radixv1.NginxSizeFormat("20m"))
		},
		func(cfg *radixv1.IngressPublic) {
			cfg.ProxyReadTimeout = pointers.Ptr[uint](100)
		},
		func(cfg *radixv1.IngressPublic) {
			cfg.ProxySendTimeout = pointers.Ptr[uint](150)
		},
	}
	// Defines a list of functions that will set a single environment specific IngressPublic field value.
	// The field value that each function set must be different from the value set by the corresponding (same field)
	// function in setCommonCfg
	setEnvCfg := setIngressFuncs{
		func(cfg *radixv1.IngressPublic) {
			cfg.Allow = &[]radixv1.IPOrCIDR{radixv1.IPOrCIDR("1.1.1.1"), radixv1.IPOrCIDR("2.2.2.2")}
		},
		func(cfg *radixv1.IngressPublic) {
			cfg.ProxyBodySize = pointers.Ptr(radixv1.NginxSizeFormat("10m"))
		},
		func(cfg *radixv1.IngressPublic) {
			cfg.ProxyReadTimeout = pointers.Ptr[uint](10)
		},
		func(cfg *radixv1.IngressPublic) {
			cfg.ProxySendTimeout = pointers.Ptr[uint](15)
		},
	}

	/*
		The tests will check every possible combination of component and environment specific configuration of the IngressPublic spec.
		exp2 is used in the two for-loops to create a bitmap representation of each function in setCommonCfg and setEnvCfg.
		The function is called with the corresponding config (common or env) and expectedCfg if the function's bit is set.

		How it works:
		We have 4 functions in each slice. To iterate over every possible combination of function call (call none, some or all),
		we calculate 2 pow 4 = 16, and iterate from 0 to 15. This binary representation for each value will then be:
		0:  0000 (no functions will be called)
		1:  0001 (function with index 0 will be called)
		2:  0010 (function with index 1 will be called)
		3:  0011 (functions with indexes 0 and 1 will be called)
		4:  0100 (function with index 2 will be called)
		...
		15: 1111 (all functions will be called)

		It is imortant that the setCommonCfg functions are applied to expectedCfg first and setEnvCfg last,
		since we excpect environment config to take precedence over common config if the field is non-nil.
	*/
	for c := range exp2(len(setCommonCfg)) {
		for e := range exp2(len(setEnvCfg)) {
			// Include bitmap representation of which functions in common and env config that must be called
			// This makes it a bit easier to identity what fields are set in common and env config in case a test fails
			testName := fmt.Sprintf("common bitmap: %.4b - env bitmap: %.4b", c, e)
			t.Run(testName, func(t *testing.T) {
				commonCfg := &radixv1.IngressPublic{}
				envCfg := &radixv1.IngressPublic{}
				expectedCfg := &radixv1.IngressPublic{}
				for i := range len(setCommonCfg) {
					if c&exp2(i) > 0 {
						setCommonCfg[i](commonCfg)
						setCommonCfg[i](expectedCfg)
					}
				}
				for i := range len(setEnvCfg) {
					if e&exp2(i) > 0 {
						setEnvCfg[i](envCfg)
						setEnvCfg[i](expectedCfg)
					}
				}

				const envName = "anyenv"
				ra := utils.ARadixApplication().
					WithComponents(
						utils.AnApplicationComponent().
							WithName("anycomponent").
							WithNetwork(&radixv1.Network{Ingress: &radixv1.Ingress{Public: commonCfg}}).
							WithEnvironmentConfigs(
								utils.AnEnvironmentConfig().
									WithEnvironment(envName).
									WithNetwork(&radixv1.Network{Ingress: &radixv1.Ingress{Public: envCfg}}),
							),
					).BuildRA()
				components, err := GetRadixComponentsForEnv(context.Background(), ra, nil, envName, make(pipeline.DeployComponentImages), make(radixv1.EnvVarsMap), nil)
				require.NoError(t, err)
				assert.Equal(t, expectedCfg, components[0].Network.Ingress.Public)
			})
		}
	}
}

func convertRadixDeployComponentToNameSet(deployComponents []radixv1.RadixDeployComponent) map[string]bool {
	set := make(map[string]bool)
	for _, deployComponent := range deployComponents {
		set[deployComponent.Name] = true
	}
	return set
}

func convertRadixDeployJobComponentsToNameSet(deployComponents []radixv1.RadixDeployJobComponent) map[string]bool {
	set := make(map[string]bool)
	for _, deployComponent := range deployComponents {
		set[deployComponent.Name] = true
	}
	return set
}
