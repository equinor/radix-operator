package config

import (
	"github.com/equinor/radix-operator/pkg/apis/job"
)

type LogLevel string

const (
	LogLevelInfo  LogLevel = "INFO"
	LogLevelError LogLevel = "ERROR"
	LogLevelDebug LogLevel = "DEBUG"
)

// Config from environment variables
type Config struct {
	LogLevel          LogLevel
	DNSConfig         *DNSConfig
	PipelineJobConfig *job.Config
}
