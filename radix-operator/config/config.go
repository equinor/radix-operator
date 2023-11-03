package config

import (
	"strconv"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/spf13/viper"
)

type LogLevel string

const (
	LogLevelInfo  LogLevel = "INFO"
	LogLevelError LogLevel = "ERROR"
	LogLevelDebug LogLevel = "DEBUG"
)

var logLevels = map[string]bool{string(LogLevelInfo): true, string(LogLevelDebug): true, string(LogLevelError): true}

func getLogLevel() string {
	logLevel := viper.GetString(defaults.LogLevel)
	if _, ok := logLevels[logLevel]; ok {
		return logLevel
	}
	return string(LogLevelInfo)
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
	return convertToMap(viper.GetString(defaults.RadixDNSAliasAppReservedEnvironmentVariable))
}

func convertToMap(keyValuePairs string) map[string]string {
	pair := strings.Split(keyValuePairs, ",")
	keyValues := make(map[string]string)
	for _, part := range pair {
		kv := strings.Split(part, "=")
		if len(kv) == 2 {
			keyValues[kv[0]] = kv[1]
		}
	}
	return keyValues
}

func getDNSAliasReserved() []string {
	envVar := viper.GetString(defaults.RadixDNSAliasReservedEnvironmentVariable)
	return strings.Split(envVar, ",")
}

func getIntFromEnvVar(envVarName string, defaultValue int) int {
	val, err := strconv.Atoi(viper.GetString(envVarName))
	if err != nil {
		return defaultValue
	}
	return val
}

// Config from environment variables
type Config struct {
	LogLevel          string
	ClusterConfig     *ClusterConfig
	PipelineJobConfig *job.Config
}

// NewConfig New instance of the Config
func NewConfig() *Config {
	viper.AutomaticEnv()
	return &Config{
		LogLevel: getLogLevel(),
		ClusterConfig: &ClusterConfig{
			DNSZone:             getDNSZone(),
			DNSAliasAppReserved: getDNSAliasAppReserved(),
			DNSAliasReserved:    getDNSAliasReserved(),
		},
		PipelineJobConfig: &job.Config{
			PipelineJobsHistoryLimit:              getPipelineJobsHistoryLimit(),
			DeploymentsHistoryLimitPerEnvironment: getDeploymentsHistoryLimitPerEnvironment(),
			AppBuilderResourcesLimitsMemory:       defaults.GetResourcesLimitsMemoryForAppBuilderNamespace(),
			AppBuilderResourcesRequestsCPU:        defaults.GetResourcesRequestsCPUForAppBuilderNamespace(),
			AppBuilderResourcesRequestsMemory:     defaults.GetResourcesRequestsMemoryForAppBuilderNamespace(),
		},
	}
}
