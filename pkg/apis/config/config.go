package config

import (
	"github.com/equinor/radix-operator/pkg/apis/config/certificate"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/config/pipelinejob"
	log "github.com/sirupsen/logrus"
)

// Config from environment variables
type Config struct {
	LogLevel              log.Level
	DNSConfig             *dnsalias.DNSConfig
	PipelineJobConfig     *pipelinejob.Config
	CertificateAutomation *certificate.AutomationConfig
}
