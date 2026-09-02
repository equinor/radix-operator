package config2_test

import (
	"strings"
	"testing"

	_ "embed"

	"github.com/equinor/radix-operator/pkg/apis/config2"
	"github.com/equinor/radix-operator/pkg/apis/scheme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
			ClusterName: "test-cluster",
			OAuth2Proxy: config2.OAuth2ProxyConfig{
				DefaultOIDCIssuer: "https://example.com",
				ProxyImage: config2.ContainerImage{
					Repository: "quay.io/oauth2-proxy/oauth2-proxy",
					Tag:        "v7.6.2",
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

			AppNsLimitRange: config2.LimitRangeConfig{
				DefaultMemory: new(resource.MustParse("500M")),
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
