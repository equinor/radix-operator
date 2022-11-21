package config

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/spf13/viper"
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

//GetLogLevel Gets log level
func (e *config) GetLogLevel() string {
	logLevel := viper.GetString(defaults.LogLevel)
	if _, ok := logLevels[logLevel]; ok {
		return logLevel
	}
	return string(LogLevelInfo)
}

//Config Config from environment variables
type Config interface {
	GetLogLevel() string
}

//NewConfig New instance of Config
func NewConfig() Config {
	viper.AutomaticEnv()
	return &config{}
}
