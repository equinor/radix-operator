package config

import (
	"time"

	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/config/pipelinejob"
	log "github.com/sirupsen/logrus"
)

// Config from environment variables
type Config struct {
	LogLevel          log.Level
	DNSConfig         *dnsalias.DNSConfig
	PipelineJobConfig *pipelinejob.Config
	DeploymentSyncer  DeploymentSyncerConfig
}

type DeploymentSyncerConfig struct {
	CertificateAutomation CertificateAutomationConfig
}

type CertificateAutomationConfig struct {
	ClusterIssuer string
	Duration      time.Duration
	RenewBefore   time.Duration
}
