package config2_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/config2"
	"github.com/equinor/radix-operator/pkg/apis/scheme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func testReader(configYaml string) config2.ConfigReader {
	return func(_ context.Context) (string, error) {
		return configYaml, nil
	}
}

func testFileReader(t *testing.T, path string) config2.ConfigReader {
	t.Helper()
	configYaml, err := os.ReadFile(path)
	require.NoError(t, err)
	return testReader(string(configYaml))
}

func TestParse_HappyPath(t *testing.T) {
	cfg, err := config2.Parse(t.Context(), testFileReader(t, "testdata/config-happypath.yaml"))

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "info", cfg.Operator.LogLevel)
	assert.True(t, cfg.Operator.LogPrettyPrint)
	assert.Equal(t, "test-cluster", cfg.Common.ClusterName)
}

func TestParse_EnvOverride(t *testing.T) {
	t.Setenv("OPERATOR_LOG_LEVEL", "debug")

	cfg, err := config2.Parse(t.Context(), testFileReader(t, "testdata/config-happypath.yaml"))

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "debug", cfg.Operator.LogLevel)
}

func TestParse_RequiredFieldFromEnvOverride(t *testing.T) {
	t.Setenv("CLUSTER_NAME", "env-cluster")
	configYaml, err := os.ReadFile("testdata/config-happypath.yaml")
	require.NoError(t, err)
	configYaml = []byte(strings.ReplaceAll(string(configYaml), "common:\n  clusterName: test-cluster\n", "common: {}\n"))

	cfg, err := config2.Parse(t.Context(), testReader(string(configYaml)))

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "env-cluster", cfg.Common.ClusterName)
}

func TestParse_MissingRequiredField(t *testing.T) {
	cfg, err := config2.Parse(t.Context(), testFileReader(t, "testdata/config-missing-required.yaml"))

	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestParse_ReaderError(t *testing.T) {
	readerErr := errors.New("reader failed")
	reader := func(_ context.Context) (string, error) { return "", readerErr }

	cfg, err := config2.Parse(t.Context(), reader)

	require.ErrorIs(t, err, readerErr)
	assert.Nil(t, cfg)
}

func TestParse_AllOperatorConfig(t *testing.T) {
	cfg, err := config2.Parse(t.Context(), testFileReader(t, "testdata/config-happypath.yaml"))

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, config2.OperatorConfig{
		LogLevel:                      "info",
		LogPrettyPrint:                true,
		RegistrationControllerThreads: 1,
		ApplicationControllerThreads:  2,
		EnvironmentControllerThreads:  3,
		DeploymentControllerThreads:   4,
		JobControllerThreads:          5,
		AlertControllerThreads:        6,
		KubeClientRateLimitBurst:      100,
		KubeClientRateLimitQPS:        50.5,
	}, cfg.Operator)
}

func TestEnvConfigMapReader(t *testing.T) {
	configYaml, err := os.ReadFile("testdata/config-happypath.yaml")
	require.NoError(t, err)

	tests := map[string]struct {
		env       map[string]string
		configMap *corev1.ConfigMap
		expectErr bool
	}{
		"defaults are used when env vars are not set": {
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "radix-common-config", Namespace: "default"},
				Data:       map[string]string{"configYaml": string(configYaml)},
			},
		},
		"env vars select namespace, name and key": {
			env: map[string]string{
				"POD_NAMESPACE":              "radix-system",
				"RADIX_OPERATOR_CONFIG_NAME": "custom-config",
				"RADIX_OPERATOR_CONFIG_KEY":  "customKey",
			},
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "custom-config", Namespace: "radix-system"},
				Data:       map[string]string{"customKey": string(configYaml)},
			},
		},
		"configmap not found": {
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "other-config", Namespace: "default"},
				Data:       map[string]string{"configYaml": string(configYaml)},
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

			cfg, err := config2.Parse(t.Context(), config2.EnvConfigMapReader(client))

			if test.expectErr {
				require.Error(t, err)
				assert.Nil(t, cfg)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, cfg)
			assert.Equal(t, "test-cluster", cfg.Common.ClusterName)
			assert.Equal(t, "info", cfg.Operator.LogLevel)
		})
	}
}
