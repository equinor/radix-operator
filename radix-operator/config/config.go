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

// GetJobsHistoryLimitPerEnvironment Gets job history limit per application environment.
// Applicable to stopped and failed jobs, which environment can be defined.
func (e *config) GetJobsHistoryLimitPerEnvironment() int {
	return getIntFromEnvVar(defaults.JobsHistoryLimitEnvironmentVariable, 0)
}

// GetJobsHistoryLimitOutOfEnvironment Gets job history limit for jobs, not bound to any application environment.
// Applicable to failed and stopped jobs, which environment cannot be defined.
// It is also applicable stopped-no-changes jobs
func (e *config) GetJobsHistoryLimitOutOfEnvironment() int {
	return getIntFromEnvVar(defaults.JobsHistoryLimitEnvironmentVariable, 0) //currently using this env-var - it can be changed to own env-var when needed
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
	GetJobsHistoryLimitPerEnvironment() int
	GetJobsHistoryLimitOutOfEnvironment() int
	GetDeploymentsHistoryLimitPerEnvironment() int
}

// NewConfig New instance of the Config
func NewConfig() Config {
	viper.AutomaticEnv()
	return &config{}
}
