package config

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMustParse(t *testing.T) {
	envVars := map[string]string{
		// Config
		defaults.LogLevel: "INFO",
		"LOG_PRETTY":      "true",
		defaults.RadixSafeToRestartBatchJobThresholdVariable: "259200",

		// PipelineJobConfig
		defaults.DeploymentsHistoryLimitEnvironmentVariable: "10",
		defaults.RadixGitCloneGitImageEnvironmentVariable:   "docker.io/alpine/git:2.45.2",
		defaults.RadixPipelineImageEnvironmentVariable:      "radixdev.azurecr.io/radix-pipeline:latest",

		// DeploymentSyncer
		defaults.KubernetesApiPortEnvironmentVariable: "443",
	}

	for k, v := range envVars {
		t.Setenv(k, v)
	}

	cfg := MustParse()

	// Config top-level fields

	// PipelineJobConfig
	require.NotNil(t, cfg.PipelineJobConfig)
	assert.Equal(t, "docker.io/alpine/git:2.45.2", cfg.PipelineJobConfig.GitCloneImage)
	assert.Equal(t, "radixdev.azurecr.io/radix-pipeline:latest", cfg.PipelineJobConfig.PipelineImage)
}
