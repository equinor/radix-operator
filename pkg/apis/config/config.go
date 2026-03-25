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

	// SafeToEvictBatchJobThreshold is the threshold in seconds for determining the cluster-autoscaler safe-to-evict annotation on batch jobs.
	// Jobs with timeLimitSeconds >= SafeToEvictBatchJobThreshold are marked as safe to evict.
	SafeToEvictBatchJobThreshold int64
}

type GatewayConfig struct {
	Name        string
	Namespace   string
	SectionName string
}
