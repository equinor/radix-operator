package env

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/spf13/viper"
)

type env struct {
}

type LogLevel string

const (
	LogLevelInfo  LogLevel = "INFO"
	LogLevelError LogLevel = "ERROR"
	LogLevelDebug LogLevel = "DEBUG"
)

var logLevels = map[string]bool{string(LogLevelInfo): true, string(LogLevelDebug): true, string(LogLevelError): true}

//GetLogLevel Gets log level
func (e *env) GetLogLevel() string {
	logLevel := viper.GetString(defaults.LogLevel)
	if _, ok := logLevels[logLevel]; ok {
		return logLevel
	}
	return string(LogLevelInfo)
}

//Env Environment from environment variables
type Env interface {
	GetLogLevel() string
}

//NewEnvironment New instance of Env
func NewEnvironment() Env {
	viper.AutomaticEnv()
	return &env{}
}
