package config2_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/config2"
	"github.com/equinor/radix-operator/pkg/apis/scheme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestParse_HappyPath(t *testing.T) {
	configYaml, err := os.ReadFile("testdata/config-happypath.yaml")
	require.NoError(t, err)

	cm := &corev1.ConfigMap{
		Name:      "radix-common-config",
		Namespace: "default",
		Data:      map[string]string{"config": string(configYaml)},
	}
	client := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(cm).Build()

	cfg, err := config2.Parse(context.Background(), client, "default", "radix-common-config")

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "info", cfg.Operator.LogLevel)
	assert.True(t, cfg.Operator.LogPrettyPrint)
	assert.Equal(t, "test-cluster", cfg.Common.ClusterName)
}

func TestParse_EnvOverride(t *testing.T) {
	t.Setenv("OPERATOR_LOG_LEVEL", "debug")
	configYaml, err := os.ReadFile("testdata/config-happypath.yaml")
	require.NoError(t, err)

	cm := &corev1.ConfigMap{
		Name:      "radix-common-config",
		Namespace: "default",
		Data:      map[string]string{"config": string(configYaml)},
	}
	client := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(cm).Build()

	cfg, err := config2.Parse(context.Background(), client, "default", "radix-common-config")

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "debug", cfg.Operator.LogLevel)

}

func TestParse_RequiredFieldFromEnvOverride(t *testing.T) {
	t.Setenv("CLUSTER_NAME", "env-cluster")
	configYaml, err := os.ReadFile("testdata/config-happypath.yaml")
	require.NoError(t, err)
	configYaml = []byte(strings.ReplaceAll(string(configYaml), "common:\n  clusterName: test-cluster\n", "common: {}\n"))

	cm := &corev1.ConfigMap{
		Name:      "radix-common-config",
		Namespace: "default",
		Data:      map[string]string{"config": string(configYaml)},
	}
	client := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(cm).Build()

	cfg, err := config2.Parse(context.Background(), client, "default", "radix-common-config")

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "env-cluster", cfg.Common.ClusterName)
}

func TestParse_MissingRequiredField(t *testing.T) {
	configYaml, err := os.ReadFile("testdata/config-missing-required.yaml")
	require.NoError(t, err)

	cm := &corev1.ConfigMap{
		Name:      "radix-common-config",
		Namespace: "default",
		Data:      map[string]string{"config": string(configYaml)},
	}
	client := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(cm).Build()

	cfg, err := config2.Parse(context.Background(), client, "default", "radix-common-config")

	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestParse_AllOperatorConfig(t *testing.T) {
	configYaml, err := os.ReadFile("testdata/config-happypath.yaml")
	require.NoError(t, err)

	cm := &corev1.ConfigMap{
		Name:      "radix-common-config",
		Namespace: "default",
		Data:      map[string]string{"config": string(configYaml)},
	}
	client := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(cm).Build()

	cfg, err := config2.Parse(context.Background(), client, "default", "radix-common-config")

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
