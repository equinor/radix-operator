package config

import (
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/config/pipelinejob"
)

type LogLevel string

const (
	LogLevelError   LogLevel = "ERROR"
	LogLevelInfo    LogLevel = "INFO"
	LogLevelWarning LogLevel = "WARNING"
	LogLevelDebug   LogLevel = "DEBUG"
)

// Config from environment variables
type Config struct {
	LogLevel          LogLevel
	DNSConfig         *dnsalias.DNSConfig
	PipelineJobConfig *pipelinejob.Config
}
