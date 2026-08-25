package config2_test

import (
	"context"
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/config2"
	"github.com/equinor/radix-operator/pkg/apis/scheme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestParse_HappyPath(t *testing.T) {
	configYaml, err := os.ReadFile("testdata/config-happypath.yaml")
	require.NoError(t, err)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "radix-common-config", Namespace: "default"},
		Data:       map[string]string{"config": string(configYaml)},
	}
	client := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(cm).Build()

	cfg, err := config2.Parse(context.Background(), client)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "info", cfg.Operator.LogLevel)
	assert.True(t, cfg.Operator.LogPrettyPrint)
	assert.Equal(t, "test-cluster", cfg.Common.ClusterName)
}

func TestParse_EnvOverride(t *testing.T) {
	t.Setenv("LOG_LEVEL", "debug")
	configYaml, err := os.ReadFile("testdata/config-happypath.yaml")
	require.NoError(t, err)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "radix-common-config", Namespace: "default"},
		Data:       map[string]string{"config": string(configYaml)},
	}
	client := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(cm).Build()

	cfg, err := config2.Parse(context.Background(), client)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "debug", cfg.Operator.LogLevel)

}

func TestParse_MissingRequiredField(t *testing.T) {
	configYaml, err := os.ReadFile("testdata/config-missing-required.yaml")
	require.NoError(t, err)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "radix-common-config", Namespace: "default"},
		Data:       map[string]string{"config": string(configYaml)},
	}
	client := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(cm).Build()

	cfg, err := config2.Parse(context.Background(), client)

	require.Error(t, err)
	assert.Nil(t, cfg)
}
