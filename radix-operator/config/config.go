package config

import (
	"strconv"

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
	return getIntFromEnvVar(defaults.DeploymentsHistoryLimitEnvironmentVariable, 10)
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
	PipelineJobConfig *job.Config
}

// NewConfig New instance of the Config
func NewConfig() *Config {
	viper.AutomaticEnv()
	return &Config{
		LogLevel: getLogLevel(),
		PipelineJobConfig: &job.Config{
			PipelineJobsHistoryLimit:              getPipelineJobsHistoryLimit(),
			DeploymentsHistoryLimitPerEnvironment: getDeploymentsHistoryLimitPerEnvironment(),
			AppBuilderResourcesLimitsMemory:       defaults.GetResourcesLimitsMemoryForAppBuilderNamespace(),
			AppBuilderResourcesRequestsCPU:        defaults.GetResourcesRequestsCPUForAppBuilderNamespace(),
			AppBuilderResourcesRequestsMemory:     defaults.GetResourcesRequestsMemoryForAppBuilderNamespace(),
		},
	}
}
