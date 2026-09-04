package config2_test

import (
	"strings"
	"testing"

	_ "embed"

	"github.com/equinor/radix-operator/pkg/apis/config2"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/scheme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

//go:embed testdata/config-happypath.yaml
var configHappyYaml string

//go:embed testdata/config-missing-required.yaml
var configMissingRequiredYaml string

func TestParse_HappyPath(t *testing.T) {
	cfg, err := config2.Parse(configHappyYaml)
	require.NoError(t, err)

	expected := &config2.Config{
		Common: config2.CommonConfig{
			DNSZone:     "dev.local.radix.equinor.com",
			ClusterName: "test-cluster",
			OAuth2Proxy: config2.OAuth2ProxyConfig{
				ProxyImage: config2.ContainerImage{
					Repository: "quay.io/oauth2-proxy/oauth2-proxy",
					Tag:        "v7.6.2",
				},
				RedisImage: config2.ContainerImage{
					Repository: "docker.io/redis",
					Tag:        "v8.6.0",
				},
				ProxyDefaults: v1.OAuth2{
					Scope:                  "openid profile email",
					ProxyPrefix:            "/oauth2",
					SetXAuthRequestHeaders: new(false),
					SetAuthorizationHeader: new(false),
					SessionStoreType:       v1.SessionStoreCookie,
					Cookie: &v1.OAuth2Cookie{
						Name:     "_oauth2_proxy",
						Expire:   "168h0m0s",
						Refresh:  "60m0s",
						SameSite: v1.SameSiteLax,
					},
					OIDC: &v1.OAuth2OIDC{
						IssuerURL:     "https://issuer.com",
						SkipDiscovery: new(false),
					},
				},
			},
		},
		Operator: config2.OperatorConfig{
			LogLevel:                          "info",
			LogPrettyPrint:                    true,
			RegistrationControllerThreads:     1,
			ApplicationControllerThreads:      2,
			EnvironmentControllerThreads:      3,
			DeploymentControllerThreads:       4,
			JobControllerThreads:              5,
			AlertControllerThreads:            6,
			KubeClientRateLimitBurst:          100,
			KubeClientRateLimitQPS:            50.5,
			ReadinessProbeInitialDelaySeconds: 5,
			ReadinessProbePeriodSeconds:       10,

			DefaultRollingUpdateMaxUnavailable: "25%",
			DefaultRollingUpdateMaxSurge:       "35%",

			ContainerRegistry:    "any.registry.com",
			AppContainerRegistry: "app.registry.com",

			DefaultAppAdminGroups: []string{"default-app-admin-group1", "default-app-admin-group2"},

			AppNsLimitRange: config2.LimitRangeConfig{
				DefaultMemory:        new(resource.MustParse("500M")),
				DefaultRequestMemory: new(resource.MustParse("450M")),
				DefaultRequestCPU:    new(resource.MustParse("100m")),
			},
			EnvNsLimitRange: config2.LimitRangeConfig{
				DefaultMemory:        new(resource.MustParse("555M")),
				DefaultRequestMemory: new(resource.MustParse("444M")),
				DefaultRequestCPU:    new(resource.MustParse("111m")),
			},
			BuilderResources: config2.Resources{
				Limits: config2.ResourceRequirements{
					Memory: new(resource.MustParse("500M")),
					CPU:    new(resource.MustParse("2000m")),
				},
				Requests: config2.ResourceRequirements{
					Memory: new(resource.MustParse("500M")),
					CPU:    new(resource.MustParse("200m")),
				},
			},
		},
	}

	assert.Equal(t, expected, cfg)
}

func TestParse_EnvOverride(t *testing.T) {
	t.Setenv("RADIX_OPERATOR_LOGLEVEL", "debug")

	cfg, err := config2.Parse(configHappyYaml)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "debug", cfg.Operator.LogLevel)
}

// Only slice fields are comma separated, a scalar keeps the value as it is.
func TestParse_EnvOverrideDoesNotSplitStrings(t *testing.T) {
	t.Setenv("RADIX_OPERATOR_LOGLEVEL", "debug,info")

	cfg, err := config2.Parse(configHappyYaml)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "debug,info", cfg.Operator.LogLevel)
}
func TestParse_RequiredFieldFromEnvOverride(t *testing.T) {
	t.Setenv("RADIX_COMMON_CLUSTERNAME", "env-cluster")
	configYamlStr := strings.ReplaceAll(configHappyYaml, "  clusterName: test-cluster\n", "")

	cfg, err := config2.Parse(configYamlStr)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "env-cluster", cfg.Common.ClusterName)
}

// A field without an env tag is overridden by the uppercased field path, with dots replaced by underscores.
func TestParse_EnvOverrideFromFieldPath(t *testing.T) {
	t.Setenv("RADIX_COMMON_OAUTH2PROXY_PROXYIMAGE_REPOSITORY", "ghcr.io/equinor/oauth2-proxy")
	t.Setenv("RADIX_COMMON_OAUTH2PROXY_PROXYIMAGE_TAG", "v1.2.3")

	cfg, err := config2.Parse(configHappyYaml)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	expected := config2.ContainerImage{Repository: "ghcr.io/equinor/oauth2-proxy", Tag: "v1.2.3"}
	assert.Equal(t, expected, cfg.Common.OAuth2Proxy.ProxyImage)
}

func TestParse_EnvTagTakesPrecedenceOverFieldPath(t *testing.T) {
	t.Setenv("RADIX_COMMON_CLUSTERNAME", "env-cluster")

	cfg, err := config2.Parse(configHappyYaml)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "env-cluster", cfg.Common.ClusterName)
}

func TestParse_MissingRequiredField(t *testing.T) {
	cfg, err := config2.Parse(configMissingRequiredYaml)

	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestParse_BuilderResourceLimits(t *testing.T) {
	tests := map[string]struct {
		modifyConfig func(*config2.Config)
		errorPath    string
	}{
		"equivalent CPU quantities are valid": {
			modifyConfig: func(cfg *config2.Config) {
				cfg.Operator.BuilderResources.Limits.CPU = new(resource.MustParse("1"))
				cfg.Operator.BuilderResources.Requests.CPU = new(resource.MustParse("1000m"))
			},
		},
		"CPU limit below request is invalid": {
			modifyConfig: func(cfg *config2.Config) {
				cfg.Operator.BuilderResources.Limits.CPU = new(resource.MustParse("100m"))
			},
			errorPath: "Operator.BuilderResources.Limits.CPU",
		},
		"memory limit below request is invalid": {
			modifyConfig: func(cfg *config2.Config) {
				cfg.Operator.BuilderResources.Limits.Memory = new(resource.MustParse("499M"))
			},
			errorPath: "Operator.BuilderResources.Limits.Memory",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var sourceConfig config2.Config
			require.NoError(t, yaml.Unmarshal([]byte(configHappyYaml), &sourceConfig))
			test.modifyConfig(&sourceConfig)
			configYaml, err := yaml.Marshal(sourceConfig)
			require.NoError(t, err)

			cfg, err := config2.Parse(string(configYaml))
			if test.errorPath == "" {
				require.NoError(t, err)
				assert.NotNil(t, cfg)
				return
			}

			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.ErrorContains(t, err, test.errorPath)
		})
	}
}

func TestEnvConfigMapReader(t *testing.T) {

	tests := map[string]struct {
		env       map[string]string
		configMap *corev1.ConfigMap
		expectErr bool
	}{
		"defaults are used when env vars are not set": {
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "radix-common-config", Namespace: "default"},
				Data:       map[string]string{"configYaml": configHappyYaml},
			},
		},
		"env vars select namespace, name and key": {
			env: map[string]string{
				"POD_NAMESPACE":            "radix-system",
				"RADIX_COMMON_CONFIG_NAME": "custom-config",
				"RADIX_COMMON_CONFIG_KEY":  "customKey",
			},
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "custom-config", Namespace: "radix-system"},
				Data:       map[string]string{"customKey": configHappyYaml},
			},
		},
		"configmap not found": {
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "other-config", Namespace: "default"},
				Data:       map[string]string{"configYaml": configHappyYaml},
			},
			expectErr: true,
		},
		"key not found in configmap": {
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "radix-common-config", Namespace: "default"},
				Data:       map[string]string{"someOtherKey": configHappyYaml},
			},
			expectErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			for k, v := range test.env {
				t.Setenv(k, v)
			}
			client := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(test.configMap).Build()

			reader, err := config2.EnvConfigMapReader(t.Context(), client)
			if test.expectErr {
				require.Error(t, err)
				assert.Empty(t, reader)
				return
			}
			require.NoError(t, err)

			cfg, err := config2.Parse(reader)
			require.NoError(t, err)
			require.NotNil(t, cfg)
			assert.Equal(t, "test-cluster", cfg.Common.ClusterName)
			assert.Equal(t, "info", cfg.Operator.LogLevel)
		})
	}
}
