package config

import (
	"strconv"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils/maps"
	apiconfig "github.com/equinor/radix-operator/pkg/apis/config"
	certificateconfig "github.com/equinor/radix-operator/pkg/apis/config/certificate"
	"github.com/equinor/radix-operator/pkg/apis/config/containerregistry"
	"github.com/equinor/radix-operator/pkg/apis/config/deployment"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/config/pipelinejob"
	"github.com/equinor/radix-operator/pkg/apis/config/task"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

const (
	minPipelineJobsHistoryLimit                = 3
	minDeploymentsHistoryLimit                 = 3
	minPipelineJobsHistoryPeriodLimit          = time.Hour * 24
	minOrphanedEnvironmentsRetentionPeriod     = time.Minute * 5
	defaultOrphanedEnvironmentsRetentionPeriod = time.Hour * 24 * 30
	defaultOrphanedEnvironmentsCleanupCron     = "0 0 * * *"
)

// Gets pipeline job history limit per each list, grouped by pipeline environment and job status
func getPipelineJobsHistoryLimit() int {
	historyLimit := getIntFromEnvVar(defaults.PipelineJobsHistoryLimitEnvironmentVariable, 0)
	if historyLimit < minPipelineJobsHistoryLimit {
		log.Error().Msgf("Invalid or too small pipeline job history limit %d, set default %d", historyLimit, minPipelineJobsHistoryLimit)
		return minPipelineJobsHistoryLimit
	}
	return historyLimit
}

// Gets pipeline job history period limit per each list, grouped by pipeline environment and job status
func getPipelineJobsHistoryPeriodLimit() time.Duration {
	period := viper.GetString(defaults.PipelineJobsHistoryPeriodLimitEnvironmentVariable)
	duration, err := time.ParseDuration(period)
	if err != nil || duration < minPipelineJobsHistoryPeriodLimit {
		log.Error().Msgf("Invalid or too short pipeline job history period limit %s, set minimum period %s", duration.String(), minPipelineJobsHistoryPeriodLimit.String())
		return minPipelineJobsHistoryPeriodLimit
	}
	return duration
}

// Gets radix deployment history limit per application environment
func getDeploymentsHistoryLimitPerEnvironment() int {
	historyLimit := getIntFromEnvVar(defaults.DeploymentsHistoryLimitEnvironmentVariable, 0)
	if historyLimit < minDeploymentsHistoryLimit {
		log.Error().Msgf("Invalid or too small RadixDeployment history limit %d, set minimum %d", historyLimit, minDeploymentsHistoryLimit)
		return minDeploymentsHistoryLimit
	}
	return historyLimit
}

func getDNSZone() string {
	return viper.GetString(defaults.OperatorDNSZoneEnvironmentVariable)
}

func getDNSAliasAppReserved() map[string]string {
	return maps.FromString(viper.GetString(defaults.RadixReservedAppDNSAliasesEnvironmentVariable))
}

func getDNSAliasReserved() []string {
	return strings.Split(viper.GetString(defaults.RadixReservedDNSAliasesEnvironmentVariable), ",")
}

func getIntFromEnvVar(envVarName string, defaultValue int) int {
	val, err := strconv.Atoi(viper.GetString(envVarName))
	if err != nil {
		return defaultValue
	}
	return val
}

// NewConfig New instance of the Config
func NewConfig() *apiconfig.Config {
	viper.AutomaticEnv()
	return &apiconfig.Config{
		LogLevel:  viper.GetString(defaults.LogLevel),
		LogPretty: viper.GetBool("LOG_PRETTY"),
		DNSConfig: &dnsalias.DNSConfig{
			DNSZone:               getDNSZone(),
			ReservedAppDNSAliases: getDNSAliasAppReserved(),
			ReservedDNSAliases:    getDNSAliasReserved(),
		},
		PipelineJobConfig: &pipelinejob.Config{
			PipelineJobsHistoryLimit:              getPipelineJobsHistoryLimit(),
			PipelineJobsHistoryPeriodLimit:        getPipelineJobsHistoryPeriodLimit(),
			DeploymentsHistoryLimitPerEnvironment: getDeploymentsHistoryLimitPerEnvironment(),
			AppBuilderResourcesLimitsMemory:       defaults.GetResourcesLimitsMemoryForAppBuilderNamespace(),
			AppBuilderResourcesLimitsCPU:          defaults.GetResourcesLimitsCPUForAppBuilderNamespace(),
			AppBuilderResourcesRequestsCPU:        defaults.GetResourcesRequestsCPUForAppBuilderNamespace(),
			AppBuilderResourcesRequestsMemory:     defaults.GetResourcesRequestsMemoryForAppBuilderNamespace(),
			PipelineImage:                         viper.GetString(defaults.RadixPipelineImageEnvironmentVariable),
			GitCloneImage:                         viper.GetString(defaults.RadixGitCloneGitImageEnvironmentVariable),
		},
		CertificateAutomation: certificateconfig.AutomationConfig{
			ClusterIssuer: viper.GetString(defaults.RadixCertificateAutomationClusterIssuerVariable),
			Duration:      viper.GetDuration(defaults.RadixCertificateAutomationDurationVariable),
			RenewBefore:   viper.GetDuration(defaults.RadixCertificateAutomationRenewBeforeVariable),
		},
		DeploymentSyncer: deployment.SyncerConfig{
			TenantID:               viper.GetString(defaults.OperatorTenantIdEnvironmentVariable),
			KubernetesAPIPort:      viper.GetInt32(defaults.KubernetesApiPortEnvironmentVariable),
			DeploymentHistoryLimit: viper.GetInt(defaults.DeploymentsHistoryLimitEnvironmentVariable),
		},
		ContainerRegistryConfig: containerregistry.Config{
			ExternalRegistryAuthSecret: viper.GetString(defaults.RadixExternalRegistryDefaultAuthEnvironmentVariable),
		},
		TaskConfig: &task.Config{
			OrphanedRadixEnvironmentsRetentionPeriod: getOrphanedRadixEnvironmentsRetentionPeriod(),
			OrphanedEnvironmentsCleanupCron:          getOrphanedEnvironmentsCleanupCron(),
		},
	}
}

func getOrphanedEnvironmentsCleanupCron() string {
	cronExpression := viper.GetString(defaults.RadixOrphanedEnvironmentsCleanupCronVariable)
	if cronExpression == "" {
		log.Error().Msgf("Invalid or empty orphaned Radix environment cleanup cron expression, set default expression %s", defaultOrphanedEnvironmentsCleanupCron)
		return defaultOrphanedEnvironmentsCleanupCron
	}
	return cronExpression
}

func getOrphanedRadixEnvironmentsRetentionPeriod() time.Duration {
	duration := viper.GetDuration(defaults.RadixOrphanedEnvironmentsRetentionPeriodVariable)
	if duration < minOrphanedEnvironmentsRetentionPeriod {
		log.Error().Msgf("Invalid or too short orphaned Radix environment retention period %s, set default period %s", duration.String(), defaultOrphanedEnvironmentsRetentionPeriod.String())
		return defaultOrphanedEnvironmentsRetentionPeriod
	}
	return duration
}
