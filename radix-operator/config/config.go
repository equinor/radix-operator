package config

import (
	"strconv"
	"strings"

	"github.com/equinor/radix-common/utils/maps"
	apiconfig "github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/config/pipelinejob"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func getLogLevel() log.Level {
	logLevel := viper.GetString(defaults.LogLevel)
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		return log.InfoLevel
	}
	return level
}

// Gets pipeline job history limit per each list, grouped by pipeline branch and job status
func getPipelineJobsHistoryLimit() int {
	return getIntFromEnvVar(defaults.PipelineJobsHistoryLimitEnvironmentVariable, 0)
}

// Gets radix deployment history limit per application environment
func getDeploymentsHistoryLimitPerEnvironment() int {
	return getIntFromEnvVar(defaults.DeploymentsHistoryLimitEnvironmentVariable, 0)
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
		LogLevel: getLogLevel(),
		DNSConfig: &dnsalias.DNSConfig{
			DNSZone:               getDNSZone(),
			ReservedAppDNSAliases: getDNSAliasAppReserved(),
			ReservedDNSAliases:    getDNSAliasReserved(),
		},
		PipelineJobConfig: &pipelinejob.Config{
			PipelineJobsHistoryLimit:              getPipelineJobsHistoryLimit(),
			DeploymentsHistoryLimitPerEnvironment: getDeploymentsHistoryLimitPerEnvironment(),
			AppBuilderResourcesLimitsMemory:       defaults.GetResourcesLimitsMemoryForAppBuilderNamespace(),
			AppBuilderResourcesRequestsCPU:        defaults.GetResourcesRequestsCPUForAppBuilderNamespace(),
			AppBuilderResourcesRequestsMemory:     defaults.GetResourcesRequestsMemoryForAppBuilderNamespace(),
		},
		DeploymentSyncer: apiconfig.DeploymentSyncerConfig{
			CertificateAutomation: apiconfig.CertificateAutomationConfig{
				ClusterIssuer: viper.GetString(defaults.RadixCertificateAutomationClusterIssuerVariable),
				Duration:      viper.GetDuration(defaults.RadixCertificateAutomationDurationVariable),
				RenewBefore:   viper.GetDuration(defaults.RadixCertificateAutomationRenewBeforeVariable),
			},
		},
	}
}
