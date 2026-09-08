package config

import (
	"testing"
	"time"

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

		// Gateway
		defaults.RadixIngressGatewayNameVariable:        "radix-gateway",
		defaults.RadixIngressGatewayNamespaceVariable:   "radix-gateway-ns",
		defaults.RadixIngressGatewaySectionNameVariable: "https",

		// CertificateAutomation
		defaults.RadixCertificateAutomationGatewayClusterIssuerVariable: "letsencrypt",
		defaults.RadixCertificateAutomationDurationVariable:             "2160h",
		defaults.RadixCertificateAutomationRenewBeforeVariable:          "720h",

		// PipelineJobConfig
		defaults.PipelineJobsHistoryLimitEnvironmentVariable:       "5",
		defaults.PipelineJobsHistoryPeriodLimitEnvironmentVariable: "720h",
		defaults.DeploymentsHistoryLimitEnvironmentVariable:        "10",
		defaults.RadixGitCloneGitImageEnvironmentVariable:          "docker.io/alpine/git:2.45.2",
		defaults.RadixPipelineImageEnvironmentVariable:             "radixdev.azurecr.io/radix-pipeline:latest",

		// DeploymentSyncer
		defaults.KubernetesApiPortEnvironmentVariable: "443",

		// TaskConfig
		defaults.RadixOrphanedEnvironmentsRetentionPeriodVariable: "720h",
		defaults.RadixOrphanedEnvironmentsCleanupCronVariable:     "0 0 * * *",
	}

	for k, v := range envVars {
		t.Setenv(k, v)
	}

	cfg := MustParse()

	// Config top-level fields

	// Gateway
	assert.Equal(t, "radix-gateway", cfg.Gateway.Name)
	assert.Equal(t, "radix-gateway-ns", cfg.Gateway.Namespace)
	assert.Equal(t, "https", cfg.Gateway.SectionName)

	// CertificateAutomation
	assert.Equal(t, "letsencrypt", cfg.CertificateAutomation.GatewayClusterIssuer)
	assert.Equal(t, 2160*time.Hour, cfg.CertificateAutomation.Duration)
	assert.Equal(t, 720*time.Hour, cfg.CertificateAutomation.RenewBefore)

	// PipelineJobConfig
	require.NotNil(t, cfg.PipelineJobConfig)
	assert.Equal(t, 5, cfg.PipelineJobConfig.PipelineJobsHistoryLimit)
	assert.Equal(t, 720*time.Hour, cfg.PipelineJobConfig.PipelineJobsHistoryPeriodLimit)
	assert.Equal(t, 10, cfg.PipelineJobConfig.DeploymentsHistoryLimitPerEnvironment)
	assert.Equal(t, "docker.io/alpine/git:2.45.2", cfg.PipelineJobConfig.GitCloneImage)
	assert.Equal(t, "radixdev.azurecr.io/radix-pipeline:latest", cfg.PipelineJobConfig.PipelineImage)

	// DeploymentSyncer
	assert.Equal(t, int32(443), cfg.DeploymentSyncer.KubernetesAPIPort)
	assert.Equal(t, 10, cfg.DeploymentSyncer.DeploymentHistoryLimit)

	// TaskConfig
	require.NotNil(t, cfg.TaskConfig)
	assert.Equal(t, 720*time.Hour, cfg.TaskConfig.OrphanedRadixEnvironmentsRetentionPeriod)
	assert.Equal(t, "0 0 * * *", cfg.TaskConfig.OrphanedEnvironmentsCleanupCron)
}
