package e2e

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestOpertorIsRunningWithCorrectVariables(t *testing.T) {
	c := getClient(t)
	deployment := &appsv1.Deployment{}
	err := c.Get(context.Background(), types.NamespacedName{Name: "radix-operator", Namespace: "radix-system"}, deployment)
	require.NoError(t, err, "failed to get radix-operator deployment")

	actualEnvVars := make(map[string]string)
	for _, ev := range deployment.Spec.Template.Spec.Containers[0].Env {
		actualEnvVars[ev.Name] = ev.Value
	}

	// Expected env vars: empty string means presence-only check, non-empty means value check
	expectedEnvVars := map[string]string{
		defaults.OperatorDNSZoneEnvironmentVariable:                           "dev.radix.equinor.com",
		defaults.RadixZoneEnvironmentVariable:                                 "xx",
		defaults.ClusternameEnvironmentVariable:                               "weekly-e2e",
		defaults.OperatorAppAliasBaseURLEnvironmentVariable:                   "app.dev.radix.equinor.com",
		defaults.OperatorClusterTypeEnvironmentVariable:                       "development",
		defaults.OperatorDefaultAppAdminGroupsEnvironmentVariable:             "123",
		defaults.OperatorAppLimitDefaultMemoryEnvironmentVariable:             "450M",
		defaults.OperatorAppLimitDefaultRequestMemoryEnvironmentVariable:      "450M",
		defaults.OperatorAppLimitDefaultRequestCPUEnvironmentVariable:         "100m",
		defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable:             "500M",
		defaults.OperatorEnvLimitDefaultRequestMemoryEnvironmentVariable:      "500M",
		defaults.OperatorEnvLimitDefaultRequestCPUEnvironmentVariable:         "100m",
		defaults.OperatorAppBuilderResourcesLimitsMemoryEnvironmentVariable:   "500M",
		defaults.OperatorAppBuilderResourcesLimitsCPUEnvironmentVariable:      "2000m",
		defaults.OperatorAppBuilderResourcesRequestsMemoryEnvironmentVariable: "500M",
		defaults.OperatorAppBuilderResourcesRequestsCPUEnvironmentVariable:    "200m",
		defaults.OperatorReadinessProbeInitialDelaySeconds:                    "5",
		defaults.OperatorReadinessProbePeriodSeconds:                          "10",
		defaults.OperatorRollingUpdateMaxUnavailable:                          "25%",
		defaults.OperatorRollingUpdateMaxSurge:                                "25%",
		defaults.DeploymentsHistoryLimitEnvironmentVariable:                   "10",
		defaults.PipelineJobsHistoryLimitEnvironmentVariable:                  "5",
		defaults.PipelineJobsHistoryPeriodLimitEnvironmentVariable:            "720h",
		defaults.RadixImageBuilderEnvironmentVariable:                         "ghcr.io/equinor/radix/image-builder:latest",
		defaults.OperatorRadixJobSchedulerEnvironmentVariable:                 "", // e2e override
		defaults.ContainerRegistryEnvironmentVariable:                         "radixdev.azurecr.io",
		defaults.AppContainerRegistryEnvironmentVariable:                      "radixdevapp.azurecr.io",
		defaults.OperatorTenantIdEnvironmentVariable:                          "xx",
		defaults.RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable:       "https://login.microsoftonline.com/3aa4a235-b6e2-48d5-9195-7fcf05b459b0/v2.0",
		defaults.RadixOAuthProxyImageEnvironmentVariable:                      "quay.io/oauth2-proxy/oauth2-proxy:v7.9.0",
		defaults.RadixOAuthRedisImageEnvironmentVariable:                      "docker.io/redis:8.0",
		defaults.RegistrationControllerThreadsEnvironmentVariable:             "1",
		defaults.ApplicationControllerThreadsEnvironmentVariable:              "1",
		defaults.EnvironmentControllerThreadsEnvironmentVariable:              "1",
		defaults.DeploymentControllerThreadsEnvironmentVariable:               "1",
		defaults.JobControllerThreadsEnvironmentVariable:                      "1",
		defaults.AlertControllerThreadsEnvironmentVariable:                    "1",
		defaults.KubeClientRateLimitBurstEnvironmentVariable:                  "5",
		defaults.KubeClientRateLimitQpsEnvironmentVariable:                    "5",
		defaults.SeccompProfileFileNameEnvironmentVariable:                    "allow-buildah.json",
		defaults.RadixBuildKitImageBuilderEnvironmentVariable:                 "ghcr.io/equinor/radix/buildkit-builder:latest",
		defaults.RadixGitCloneGitImageEnvironmentVariable:                     "docker.io/alpine/git:2.45.2",
		defaults.RadixCertificateAutomationGatewayClusterIssuerVariable:       "",
		defaults.RadixCertificateAutomationDurationVariable:                   "2160h",
		defaults.RadixCertificateAutomationRenewBeforeVariable:                "720h",
		defaults.RadixExternalRegistryDefaultAuthEnvironmentVariable:          "",
		defaults.RadixOrphanedEnvironmentsRetentionPeriodVariable:             "720h",
		defaults.RadixOrphanedEnvironmentsCleanupCronVariable:                 "0 0 * * *",
		defaults.RadixPipelineImageEnvironmentVariable:                        "", // e2e override
		defaults.RadixIngressGatewayNameVariable:                              "", // e2e override
		defaults.RadixIngressGatewayNamespaceVariable:                         "", // e2e override
		defaults.RadixIngressGatewaySectionNameVariable:                       "https",
		defaults.RadixSafeToRestartBatchJobThresholdVariable:                  "259200",
	}

	for name, expectedValue := range expectedEnvVars {
		actualValue, exists := actualEnvVars[name]
		if !exists {
			assert.Fail(t, "env var %s is missing", name)
			continue
		}
		if expectedValue != "" {
			assert.Equal(t, expectedValue, actualValue, "env var %s has incorrect value", name)
		}
	}
}
