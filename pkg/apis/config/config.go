package config

import (
	"github.com/equinor/radix-operator/pkg/apis/config/certificate"
	"github.com/equinor/radix-operator/pkg/apis/config/containerregistry"
	"github.com/equinor/radix-operator/pkg/apis/config/deployment"
	"github.com/equinor/radix-operator/pkg/apis/config/pipelinejob"
	"github.com/equinor/radix-operator/pkg/apis/config/task"
)

// Config from environment variables
type Config struct {
	LogLevel  string
	LogPretty bool
	// DNSConfig Settings for the cluster DNS
	DNSZone                 string
	PipelineJobConfig       *pipelinejob.Config
	CertificateAutomation   certificate.AutomationConfig
	DeploymentSyncer        deployment.SyncerConfig
	ContainerRegistryConfig containerregistry.Config
	TaskConfig              *task.Config

	Gateway GatewayConfig
}

type GatewayConfig struct {
	Name        string
	Namespace   string
	SectionName string
}
