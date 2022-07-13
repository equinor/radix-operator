package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
)

const (
	appName       = "app"
	componentName = "comp"
	// containerRegistry = "radixdev.azurecr.io"
	env = "dev"

	anyImage     = componentName
	anyImagePath = anyImage
	anyImageTag  = "latest"
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
		{name: "should return component when environment is nil", comp: &v1.Authentication{}, expected: &v1.Authentication{}},
		{name: "should return environment when component is nil", env: &v1.Authentication{}, expected: &v1.Authentication{}},
	}

	for i, scenario := range scenarios {
		comp, _ := scenario.comp.(*v1.Authentication)
		env, _ := scenario.env.(*v1.Authentication)
		expected, _ := scenario.expected.(*v1.Authentication)
		actual, _ := GetAuthenticationForComponent(comp, env)
		assert.Equal(t, expected, actual, "%v: %v", i+1, scenario.name)
	}
}

func TestGetClientCertificateAuthenticationForComponent(t *testing.T) {
	verificationOptional := v1.VerificationTypeOptional
	verificationOff := v1.VerificationTypeOff

	scenarios := []scenarioDef{
		{name: "should return nil when component and environment is nil"},
		{name: "should return component when environment is nil", comp: &v1.ClientCertificate{}, expected: &v1.ClientCertificate{}},
		{name: "should return environment when component is nil", env: &v1.ClientCertificate{}, expected: &v1.ClientCertificate{}},
		{
			name:     "should use PassCertificateToUpstream from environment",
			comp:     &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(true)},
			env:      &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(false)},
			expected: &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(false)},
		},
		{
			name:     "should use PassCertificateToUpstream from environment",
			comp:     &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(false)},
			env:      &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(true)},
			expected: &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(true)},
		},
		{
			name:     "should use PassCertificateToUpstream from environment",
			comp:     &v1.ClientCertificate{},
			env:      &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(false)},
			expected: &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(false)},
		},
		{
			name:     "should use PassCertificateToUpstream from environment",
			comp:     &v1.ClientCertificate{},
			env:      &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(true)},
			expected: &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(true)},
		},
		{
			name:     "should use PassCertificateToUpstream from component when not set in environment",
			comp:     &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(false)},
			env:      &v1.ClientCertificate{},
			expected: &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(false)},
		},
		{
			name:     "should use PassCertificateToUpstream from component when not set in environment",
			comp:     &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(true)},
			env:      &v1.ClientCertificate{},
			expected: &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(true)},
		},
		{
			name:     "should use Verification from environment",
			comp:     &v1.ClientCertificate{Verification: &verificationOff},
			env:      &v1.ClientCertificate{Verification: &verificationOptional},
			expected: &v1.ClientCertificate{Verification: &verificationOptional},
		},
		{
			name:     "should use Verification from environment",
			comp:     &v1.ClientCertificate{},
			env:      &v1.ClientCertificate{Verification: &verificationOptional},
			expected: &v1.ClientCertificate{Verification: &verificationOptional},
		},
		{
			name:     "should use Verification from component",
			comp:     &v1.ClientCertificate{Verification: &verificationOff},
			env:      &v1.ClientCertificate{},
			expected: &v1.ClientCertificate{Verification: &verificationOff},
		},
		{
			name:     "should use Verification from component and PassCertificateToUpstream from environment",
			comp:     &v1.ClientCertificate{Verification: &verificationOff, PassCertificateToUpstream: utils.BoolPtr(true)},
			env:      &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(false)},
			expected: &v1.ClientCertificate{Verification: &verificationOff, PassCertificateToUpstream: utils.BoolPtr(false)},
		},
		{
			name:     "should use Verification from environment and PassCertificateToUpstream from component",
			comp:     &v1.ClientCertificate{Verification: &verificationOff, PassCertificateToUpstream: utils.BoolPtr(true)},
			env:      &v1.ClientCertificate{Verification: &verificationOptional},
			expected: &v1.ClientCertificate{Verification: &verificationOptional, PassCertificateToUpstream: utils.BoolPtr(true)},
		},
	}

	for i, scenario := range scenarios {
		comp, _ := scenario.comp.(*v1.ClientCertificate)
		env, _ := scenario.env.(*v1.ClientCertificate)
		expected, _ := scenario.expected.(*v1.ClientCertificate)
		actual, _ := GetAuthenticationForComponent(&v1.Authentication{ClientCertificate: comp}, &v1.Authentication{ClientCertificate: env})
		assert.Equal(t, expected, actual.ClientCertificate, "%v: %v", i+1, scenario.name)
	}
}

func TestGetOAuth2AuthenticationForComponent(t *testing.T) {
	scenarios := []scenarioDef{
		{name: "should return nil when component and environment is nil"},
		{name: "should return component when environment is nil", comp: &v1.OAuth2{}, expected: &v1.OAuth2{}},
		{name: "should return environment when component is nil", env: &v1.OAuth2{}, expected: &v1.OAuth2{}},
		{
			name:     "should override OAuth2 from environment",
			comp:     &v1.OAuth2{ClientID: "123", Scope: "openid", SetXAuthRequestHeaders: utils.BoolPtr(true), SessionStoreType: "cookie"},
			env:      &v1.OAuth2{Scope: "email", SetXAuthRequestHeaders: utils.BoolPtr(false), SetAuthorizationHeader: utils.BoolPtr(true), SessionStoreType: "redis"},
			expected: &v1.OAuth2{ClientID: "123", Scope: "email", SetXAuthRequestHeaders: utils.BoolPtr(false), SetAuthorizationHeader: utils.BoolPtr(true), SessionStoreType: "redis"},
		},
		{
			name:     "should override OAuth2.RedisStore from environment",
			comp:     &v1.OAuth2{RedisStore: &v1.OAuth2RedisStore{ConnectionURL: "foo"}},
			env:      &v1.OAuth2{RedisStore: &v1.OAuth2RedisStore{ConnectionURL: "bar"}},
			expected: &v1.OAuth2{RedisStore: &v1.OAuth2RedisStore{ConnectionURL: "bar"}},
		},
		{
			name:     "should override OAuth2.CookieStore from environment",
			comp:     &v1.OAuth2{CookieStore: &v1.OAuth2CookieStore{Minimal: utils.BoolPtr(true)}},
			env:      &v1.OAuth2{CookieStore: &v1.OAuth2CookieStore{Minimal: utils.BoolPtr(false)}},
			expected: &v1.OAuth2{CookieStore: &v1.OAuth2CookieStore{Minimal: utils.BoolPtr(false)}},
		},
		{
			name:     "should override OAuth2.Cookie from environment",
			comp:     &v1.OAuth2{Cookie: &v1.OAuth2Cookie{Name: "oauth", Expire: "1h"}},
			env:      &v1.OAuth2{Cookie: &v1.OAuth2Cookie{Name: "_oauth", Refresh: "2h"}},
			expected: &v1.OAuth2{Cookie: &v1.OAuth2Cookie{Name: "_oauth", Expire: "1h", Refresh: "2h"}},
		},
	}

	for i, scenario := range scenarios {
		comp, _ := scenario.comp.(*v1.OAuth2)
		env, _ := scenario.env.(*v1.OAuth2)
		expected, _ := scenario.expected.(*v1.OAuth2)
		actual, _ := GetAuthenticationForComponent(&v1.Authentication{OAuth2: comp}, &v1.Authentication{OAuth2: env})
		assert.Equal(t, expected, actual.OAuth2, "%v: %v", i+1, scenario.name)
	}
}

func TestGetRadixComponentsForEnv_PublicPort_OldPublic(t *testing.T) {
	// New publicPort does not exist, old public does not exist
	ra := utils.ARadixApplication().
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName(componentName).
				WithPort("http", 80).
				WithPort("https", 443)).BuildRA()

	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["app"] = pipeline.ComponentImage{ImageName: anyImage, ImagePath: anyImagePath}
	envVarsMap := make(v1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	deployComponent, _ := GetRadixComponentsForEnv(ra, env, componentImages, envVarsMap)
	assert.Equal(t, ra.Spec.Components[0].PublicPort, deployComponent[0].PublicPort)
	assert.Equal(t, ra.Spec.Components[0].Public, deployComponent[0].Public)
	assert.Equal(t, "", deployComponent[0].PublicPort)
	assert.Equal(t, false, deployComponent[0].Public)

	// New publicPort exists, old public does not exist
	ra = utils.ARadixApplication().
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName(componentName).
				WithPort("http", 80).
				WithPort("https", 443).
				WithPublicPort("http")).BuildRA()
	deployComponent, _ = GetRadixComponentsForEnv(ra, env, componentImages, envVarsMap)
	assert.Equal(t, ra.Spec.Components[0].PublicPort, deployComponent[0].PublicPort)
	assert.Equal(t, ra.Spec.Components[0].Public, deployComponent[0].Public)
	assert.Equal(t, "http", deployComponent[0].PublicPort)
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
	deployComponent, _ = GetRadixComponentsForEnv(ra, env, componentImages, envVarsMap)
	assert.Equal(t, ra.Spec.Components[0].PublicPort, deployComponent[0].PublicPort)
	assert.NotEqual(t, ra.Spec.Components[0].Public, deployComponent[0].Public)
	assert.Equal(t, "http", deployComponent[0].PublicPort)
	assert.Equal(t, false, deployComponent[0].Public)

	// New publicPort does not exist, old public exists (used)
	ra = utils.ARadixApplication().
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName(componentName).
				WithPort("http", 80).
				WithPort("https", 443).
				WithPublic(true)).BuildRA()
	deployComponent, _ = GetRadixComponentsForEnv(ra, env, componentImages, envVarsMap)
	assert.Equal(t, ra.Spec.Components[0].Ports[0].Name, deployComponent[0].PublicPort)
	assert.NotEqual(t, ra.Spec.Components[0].Public, deployComponent[0].Public)
	assert.Equal(t, false, deployComponent[0].Public)
}

func TestGetRadixComponentsForEnv_ListOfExternalAliasesForComponent_GetListOfAliases(t *testing.T) {
	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["app"] = pipeline.ComponentImage{ImageName: anyImage, ImagePath: anyImagePath}
	envVarsMap := make(v1.EnvVarsMap)
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
		WithDNSExternalAlias("some.alias.com", "prod", "componentA").
		WithDNSExternalAlias("another.alias.com", "prod", "componentA").
		WithDNSExternalAlias("athird.alias.com", "prod", "componentB").BuildRA()

	deployComponent, _ := GetRadixComponentsForEnv(ra, "prod", componentImages, envVarsMap)
	assert.Equal(t, 2, len(deployComponent))
	assert.Equal(t, 2, len(deployComponent[0].DNSExternalAlias))
	assert.Equal(t, "some.alias.com", deployComponent[0].DNSExternalAlias[0])
	assert.Equal(t, "another.alias.com", deployComponent[0].DNSExternalAlias[1])

	assert.Equal(t, 1, len(deployComponent[1].DNSExternalAlias))
	assert.Equal(t, "athird.alias.com", deployComponent[1].DNSExternalAlias[0])

	deployComponent, _ = GetRadixComponentsForEnv(ra, "dev", componentImages, envVarsMap)
	assert.Equal(t, 2, len(deployComponent))
	assert.Equal(t, 0, len(deployComponent[0].DNSExternalAlias))
}

func TestGetRadixComponentsForEnv_CommonEnvironmentVariables_No_Override(t *testing.T) {
	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["app"] = pipeline.ComponentImage{ImageName: anyImage, ImagePath: anyImagePath}
	envVarsMap := make(v1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	expectedGitTags := "anytag1 anytag2 anytag3"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = expectedGitTags

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

	deployComponentProd, _ := GetRadixComponentsForEnv(ra, "prod", componentImages, envVarsMap)
	assert.Equal(t, 2, len(deployComponentProd))

	assert.Equal(t, "comp_1", deployComponentProd[0].Name)
	assert.Equal(t, 4, len(deployComponentProd[0].EnvironmentVariables))
	assert.Equal(t, "environment_1", deployComponentProd[0].EnvironmentVariables["ENV_1"])
	assert.Equal(t, "environment_common_1", deployComponentProd[0].EnvironmentVariables["ENV_COMMON_1"])
	assert.Equal(t, "anycommit", deployComponentProd[0].EnvironmentVariables[defaults.RadixCommitHashEnvironmentVariable])
	assert.Equal(t, expectedGitTags, deployComponentProd[0].EnvironmentVariables[defaults.RadixGitTagsEnvironmentVariable])

	assert.Equal(t, "comp_2", deployComponentProd[1].Name)
	assert.Equal(t, 4, len(deployComponentProd[1].EnvironmentVariables))
	assert.Equal(t, "environment_3", deployComponentProd[1].EnvironmentVariables["ENV_3"])
	assert.Equal(t, "environment_common_2", deployComponentProd[1].EnvironmentVariables["ENV_COMMON_2"])
	assert.Equal(t, "anycommit", deployComponentProd[1].EnvironmentVariables[defaults.RadixCommitHashEnvironmentVariable])
	assert.Equal(t, expectedGitTags, deployComponentProd[1].EnvironmentVariables[defaults.RadixGitTagsEnvironmentVariable])

	deployComponentDev, _ := GetRadixComponentsForEnv(ra, "dev", componentImages, envVarsMap)
	assert.Equal(t, 2, len(deployComponentDev))

	assert.Equal(t, "comp_1", deployComponentDev[0].Name)
	assert.Equal(t, 4, len(deployComponentDev[0].EnvironmentVariables))
	assert.Equal(t, "environment_2", deployComponentDev[0].EnvironmentVariables["ENV_2"])
	assert.Equal(t, "environment_common_1", deployComponentDev[0].EnvironmentVariables["ENV_COMMON_1"])
	assert.Equal(t, "anycommit", deployComponentProd[0].EnvironmentVariables[defaults.RadixCommitHashEnvironmentVariable])
	assert.Equal(t, expectedGitTags, deployComponentProd[0].EnvironmentVariables[defaults.RadixGitTagsEnvironmentVariable])

	assert.Equal(t, "comp_2", deployComponentDev[1].Name)
	assert.Equal(t, 4, len(deployComponentDev[1].EnvironmentVariables))
	assert.Equal(t, "environment_4", deployComponentDev[1].EnvironmentVariables["ENV_4"])
	assert.Equal(t, "environment_common_2", deployComponentDev[1].EnvironmentVariables["ENV_COMMON_2"])
	assert.Equal(t, "anycommit", deployComponentProd[1].EnvironmentVariables[defaults.RadixCommitHashEnvironmentVariable])
	assert.Equal(t, expectedGitTags, deployComponentProd[1].EnvironmentVariables[defaults.RadixGitTagsEnvironmentVariable])
}

func TestGetRadixComponentsForEnv_CommonEnvironmentVariables_With_Override(t *testing.T) {
	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["app"] = pipeline.ComponentImage{ImageName: anyImage, ImagePath: anyImagePath}
	envVarsMap := make(v1.EnvVarsMap)
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
						WithEnvironmentVariable("ENV_COMMON_1", "environment_common_1_prod_override").
						WithEnvironmentVariable(defaults.RadixCommitHashEnvironmentVariable, "should_be_overriden_by_radixoperator").
						WithEnvironmentVariable(defaults.RadixGitTagsEnvironmentVariable, "should_be_overriden_by_radixoperator"),
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

	deployComponentProd, _ := GetRadixComponentsForEnv(ra, "prod", componentImages, envVarsMap)
	assert.Equal(t, 2, len(deployComponentProd))

	assert.Equal(t, "comp_1", deployComponentProd[0].Name)
	assert.Equal(t, 4, len(deployComponentProd[0].EnvironmentVariables))
	assert.Equal(t, "environment_1", deployComponentProd[0].EnvironmentVariables["ENV_1"])
	assert.Equal(t, "environment_common_1_prod_override", deployComponentProd[0].EnvironmentVariables["ENV_COMMON_1"])

	// these values should be overridden by system defaults
	assert.Equal(t, "anycommit", deployComponentProd[0].EnvironmentVariables[defaults.RadixCommitHashEnvironmentVariable])
	assert.Equal(t, "anytag", deployComponentProd[0].EnvironmentVariables[defaults.RadixGitTagsEnvironmentVariable])

	assert.Equal(t, "comp_2", deployComponentProd[1].Name)
	assert.Equal(t, 4, len(deployComponentProd[1].EnvironmentVariables))
	assert.Equal(t, "environment_3", deployComponentProd[1].EnvironmentVariables["ENV_3"])
	assert.Equal(t, "environment_common_2_prod_override", deployComponentProd[1].EnvironmentVariables["ENV_COMMON_2"])

	deployComponentDev, _ := GetRadixComponentsForEnv(ra, "dev", componentImages, envVarsMap)
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
	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["app"] = pipeline.ComponentImage{ImageName: anyImage, ImagePath: anyImagePath}
	envVarsMap := make(v1.EnvVarsMap)
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

	deployComponentProd, _ := GetRadixComponentsForEnv(ra, "prod", componentImages, envVarsMap)
	assert.Equal(t, 2, len(deployComponentProd))

	assert.Equal(t, "comp_1", deployComponentProd[0].Name)
	assert.Equal(t, 3, len(deployComponentProd[0].EnvironmentVariables))
	assert.Equal(t, "environment_common_1", deployComponentProd[0].EnvironmentVariables["ENV_COMMON_1"])

	assert.Equal(t, "comp_2", deployComponentProd[1].Name)
	assert.Equal(t, 3, len(deployComponentProd[1].EnvironmentVariables))
	assert.Equal(t, "environment_common_2", deployComponentProd[1].EnvironmentVariables["ENV_COMMON_2"])

	deployComponentDev, _ := GetRadixComponentsForEnv(ra, "dev", componentImages, envVarsMap)
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

	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["app"] = pipeline.ComponentImage{ImageName: anyImage, ImagePath: anyImagePath}
	envVarsMap := make(v1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	monitoringConfig := v1.MonitoringConfig{
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
						WithMonitoring(true),
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
						WithMonitoring(true),
				),
		).BuildRA()

	// check component(s) env
	comps, err := GetRadixComponentsForEnv(radApp, envs[0], componentImages, envVarsMap)
	assert.Nil(t, err)
	assert.True(t, comps[0].Monitoring)
	assert.Equal(t, monitoringConfig.PortName, comps[0].MonitoringConfig.PortName)
	assert.Equal(t, monitoringConfig.Path, comps[0].MonitoringConfig.Path)
	assert.False(t, comps[1].Monitoring)
	assert.Empty(t, comps[1].MonitoringConfig.PortName)
	assert.Empty(t, comps[1].MonitoringConfig.Path)

	// check other component(s) env
	comps, err = GetRadixComponentsForEnv(radApp, envs[1], componentImages, envVarsMap)
	assert.Nil(t, err)
	assert.False(t, comps[0].Monitoring)
	assert.Equal(t, monitoringConfig.PortName, comps[0].MonitoringConfig.PortName)
	assert.Equal(t, monitoringConfig.Path, comps[0].MonitoringConfig.Path)
	assert.True(t, comps[1].Monitoring)
	assert.Empty(t, comps[1].MonitoringConfig.PortName)
	assert.Empty(t, comps[1].MonitoringConfig.Path)
}

func TestGetRadixComponentsForEnv_CommonResources(t *testing.T) {
	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["app"] = pipeline.ComponentImage{ImageName: anyImage, ImagePath: anyImagePath}
	envVarsMap := make(v1.EnvVarsMap)
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

	deployComponentProd, _ := GetRadixComponentsForEnv(ra, "prod", componentImages, envVarsMap)
	assert.Equal(t, 1, len(deployComponentProd))
	assert.Equal(t, "comp_1", deployComponentProd[0].Name)
	assert.Equal(t, "500m", deployComponentProd[0].Resources.Requests["cpu"])
	assert.Equal(t, "128Mi", deployComponentProd[0].Resources.Requests["memory"])
	assert.Equal(t, "750m", deployComponentProd[0].Resources.Limits["cpu"])
	assert.Equal(t, "256Mi", deployComponentProd[0].Resources.Limits["memory"])

	deployComponentDev, _ := GetRadixComponentsForEnv(ra, "dev", componentImages, envVarsMap)
	assert.Equal(t, 1, len(deployComponentDev))
	assert.Equal(t, "comp_1", deployComponentDev[0].Name)
	assert.Equal(t, "250m", deployComponentDev[0].Resources.Requests["cpu"])
	assert.Equal(t, "64Mi", deployComponentDev[0].Resources.Requests["memory"])
	assert.Equal(t, "500m", deployComponentDev[0].Resources.Limits["cpu"])
	assert.Equal(t, "128Mi", deployComponentDev[0].Resources.Limits["memory"])
}

func Test_GetRadixComponents_NodeName(t *testing.T) {
	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["app"] = pipeline.ComponentImage{ImageName: anyImage, ImagePath: anyImagePath}
	envVarsMap := make(v1.EnvVarsMap)
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
				WithNode(v1.RadixNode{Gpu: compGpu, GpuCount: compGpuCount}).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("env1").
						WithNode(v1.RadixNode{Gpu: envGpu1, GpuCount: envGpuCount1}),
					utils.AnEnvironmentConfig().
						WithEnvironment("env2").
						WithNode(v1.RadixNode{GpuCount: envGpuCount2}),
					utils.AnEnvironmentConfig().
						WithEnvironment("env3").
						WithNode(v1.RadixNode{Gpu: envGpu3}),
					utils.AnEnvironmentConfig().
						WithEnvironment("env4"),
				),
		).BuildRA()

	t.Run("override job gpu and gpu-count with environment gpu and gpu-count", func(t *testing.T) {
		t.Parallel()
		deployComponent, _ := GetRadixComponentsForEnv(ra, "env1", componentImages, envVarsMap)
		assert.Equal(t, envGpu1, deployComponent[0].Node.Gpu)
		assert.Equal(t, envGpuCount1, deployComponent[0].Node.GpuCount)
	})
	t.Run("override job gpu-count with environment gpu-count", func(t *testing.T) {
		t.Parallel()
		deployComponent, _ := GetRadixComponentsForEnv(ra, "env2", componentImages, envVarsMap)
		assert.Equal(t, compGpu, deployComponent[0].Node.Gpu)
		assert.Equal(t, envGpuCount2, deployComponent[0].Node.GpuCount)
	})
	t.Run("override job gpu with environment gpu", func(t *testing.T) {
		t.Parallel()
		deployComponent, _ := GetRadixComponentsForEnv(ra, "env3", componentImages, envVarsMap)
		assert.Equal(t, envGpu3, deployComponent[0].Node.Gpu)
		assert.Equal(t, compGpuCount, deployComponent[0].Node.GpuCount)
	})
	t.Run("do not override job gpu or gpu-count with environment gpu or gpu-count", func(t *testing.T) {
		t.Parallel()
		deployComponent, _ := GetRadixComponentsForEnv(ra, "env4", componentImages, envVarsMap)
		assert.Equal(t, compGpu, deployComponent[0].Node.Gpu)
		assert.Equal(t, compGpuCount, deployComponent[0].Node.GpuCount)
	})
}
