package config

import (
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestMustParse(t *testing.T) {
	envVars := map[string]string{
		// Config
		defaults.LogLevel: "INFO",
		"LOG_PRETTY":      "true",
		defaults.OperatorDNSZoneEnvironmentVariable:          "dev.radix.equinor.com",
		defaults.OperatorClusterTypeEnvironmentVariable:      "development",
		defaults.ClusternameEnvironmentVariable:              "weekly-e2e",
		defaults.ContainerRegistryEnvironmentVariable:        "radixdev.azurecr.io",
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
		defaults.PipelineJobsHistoryLimitEnvironmentVariable:                  "5",
		defaults.PipelineJobsHistoryPeriodLimitEnvironmentVariable:            "720h",
		defaults.DeploymentsHistoryLimitEnvironmentVariable:                   "10",
		defaults.OperatorAppBuilderResourcesLimitsCPUEnvironmentVariable:      "2000m",
		defaults.OperatorAppBuilderResourcesLimitsMemoryEnvironmentVariable:   "500M",
		defaults.OperatorAppBuilderResourcesRequestsCPUEnvironmentVariable:    "200m",
		defaults.OperatorAppBuilderResourcesRequestsMemoryEnvironmentVariable: "500M",
		defaults.RadixGitCloneGitImageEnvironmentVariable:                     "docker.io/alpine/git:2.45.2",
		defaults.RadixPipelineImageEnvironmentVariable:                        "radixdev.azurecr.io/radix-pipeline:latest",

		// DeploymentSyncer
		defaults.OperatorTenantIdEnvironmentVariable:  "3aa4a235-b6e2-48d5-9195-7fcf05b459b0",
		defaults.KubernetesApiPortEnvironmentVariable: "443",

		// ContainerRegistryConfig
		defaults.RadixExternalRegistryDefaultAuthEnvironmentVariable: "radix-external-registry-auth",

		// TaskConfig
		defaults.RadixOrphanedEnvironmentsRetentionPeriodVariable: "720h",
		defaults.RadixOrphanedEnvironmentsCleanupCronVariable:     "0 0 * * *",
	}

	for k, v := range envVars {
		t.Setenv(k, v)
	}

	cfg := MustParse()

	// Config top-level fields
	assert.Equal(t, "INFO", cfg.LogLevel)
	assert.Equal(t, true, cfg.LogPretty)
	assert.Equal(t, "dev.radix.equinor.com", cfg.DNSZone)
	assert.Equal(t, "development", cfg.ClusterType)
	assert.Equal(t, "weekly-e2e", cfg.ClusterName)
	assert.Equal(t, "radixdev.azurecr.io", cfg.ContainerRegistryName)
	assert.Equal(t, int64(259200), cfg.SafeToRestartBatchJobThreshold)

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
	assert.Equal(t, resource.MustParse("2000m"), cfg.PipelineJobConfig.AppBuilderResourcesLimitsCPU.Quantity)
	assert.Equal(t, resource.MustParse("500M"), cfg.PipelineJobConfig.AppBuilderResourcesLimitsMemory.Quantity)
	assert.Equal(t, resource.MustParse("200m"), cfg.PipelineJobConfig.AppBuilderResourcesRequestsCPU.Quantity)
	assert.Equal(t, resource.MustParse("500M"), cfg.PipelineJobConfig.AppBuilderResourcesRequestsMemory.Quantity)
	assert.Equal(t, "docker.io/alpine/git:2.45.2", cfg.PipelineJobConfig.GitCloneImage)
	assert.Equal(t, "radixdev.azurecr.io/radix-pipeline:latest", cfg.PipelineJobConfig.PipelineImage)

	// DeploymentSyncer
	assert.Equal(t, "3aa4a235-b6e2-48d5-9195-7fcf05b459b0", cfg.DeploymentSyncer.TenantID)
	assert.Equal(t, int32(443), cfg.DeploymentSyncer.KubernetesAPIPort)
	assert.Equal(t, 10, cfg.DeploymentSyncer.DeploymentHistoryLimit)

	// ContainerRegistryConfig
	assert.Equal(t, "radix-external-registry-auth", cfg.ContainerRegistryConfig.ExternalRegistryAuthSecret)

	// TaskConfig
	require.NotNil(t, cfg.TaskConfig)
	assert.Equal(t, 720*time.Hour, cfg.TaskConfig.OrphanedRadixEnvironmentsRetentionPeriod)
	assert.Equal(t, "0 0 * * *", cfg.TaskConfig.OrphanedEnvironmentsCleanupCron)
}

func Test_ImagePullSecretsFromDefaultAuth(t *testing.T) {
	cfg := ContainerRegistryConfig{ExternalRegistryAuthSecret: ""}
	assert.Len(t, cfg.ImagePullSecretsFromExternalRegistryAuth(), 0)

	secretName := "a-secret"
	cfg = ContainerRegistryConfig{ExternalRegistryAuthSecret: secretName}
	expected := []corev1.LocalObjectReference{{Name: secretName}}
	assert.ElementsMatch(t, expected, cfg.ImagePullSecretsFromExternalRegistryAuth())
}
