package config

import (
	"github.com/equinor/radix-operator/pkg/apis/config/certificate"
	"github.com/equinor/radix-operator/pkg/apis/config/containerregistry"
	"github.com/equinor/radix-operator/pkg/apis/config/deployment"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/config/pipelinejob"
	"github.com/equinor/radix-operator/pkg/apis/config/task"
)

// Config from environment variables
type Config struct {
	LogLevel                string
	LogPretty               bool
	DNSConfig               *dnsalias.DNSConfig
	PipelineJobConfig       *pipelinejob.Config
	CertificateAutomation   certificate.AutomationConfig
	DeploymentSyncer        deployment.SyncerConfig
	ContainerRegistryConfig containerregistry.Config
	TaskConfig              *task.Config
}
