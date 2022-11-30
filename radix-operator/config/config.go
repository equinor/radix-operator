package config

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/spf13/viper"
	strconv "strconv"
)

type config struct {
}

type LogLevel string

const (
	LogLevelInfo  LogLevel = "INFO"
	LogLevelError LogLevel = "ERROR"
	LogLevelDebug LogLevel = "DEBUG"
)

var logLevels = map[string]bool{string(LogLevelInfo): true, string(LogLevelDebug): true, string(LogLevelError): true}

// GetLogLevel Gets log level
func (e *config) GetLogLevel() string {
	logLevel := viper.GetString(defaults.LogLevel)
	if _, ok := logLevels[logLevel]; ok {
		return logLevel
	}
	return string(LogLevelInfo)
}

// GetPipelineJobsHistoryLimit Gets pipeline job history limit per each list, grouped by pipeline branch and job status
func (e *config) GetPipelineJobsHistoryLimit() int {
	return getIntFromEnvVar(defaults.PipelineJobsHistoryLimitEnvironmentVariable, 0)
}

// GetDeploymentsHistoryLimitPerEnvironment Gets radix deployment history limit per application environment
func (e *config) GetDeploymentsHistoryLimitPerEnvironment() int {
	return getIntFromEnvVar(defaults.DeploymentsHistoryLimitEnvironmentVariable, 0)
}

func getIntFromEnvVar(envVarName string, defaultValue int) int {
	val, err := strconv.Atoi(viper.GetString(envVarName))
	if err != nil {
		return defaultValue
	}
	return val
}

// Config Config from environment variables
type Config interface {
	GetLogLevel() string
	GetPipelineJobsHistoryLimit() int
	GetDeploymentsHistoryLimitPerEnvironment() int
}

// NewConfig New instance of the Config
func NewConfig() Config {
	viper.AutomaticEnv()
	return &config{}
}
