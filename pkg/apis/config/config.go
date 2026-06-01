package config

import (
	"time"

	"github.com/equinor/radix-operator/pkg/apis/config/containerregistry"
	"github.com/equinor/radix-operator/pkg/apis/config/deployment"
	"github.com/equinor/radix-operator/pkg/apis/config/pipelinejob"
	"github.com/equinor/radix-operator/pkg/apis/config/task"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

// Config from environment variables
type Config struct {
	LogLevel  string `envconfig:"LOG_LEVEL" required:"true"`
	LogPretty bool   `envconfig:"LOG_PRETTY" default:"false"`
	// DNSConfig Settings for the cluster DNS
	DNSZone               string `envconfig:"DNS_ZONE" required:"true"`
	ClusterType           string `envconfig:"RADIXOPERATOR_CLUSTER_TYPE" required:"true"`
	ClusterName           string `envconfig:"RADIX_CLUSTERNAME" required:"true"`
	ContainerRegistryName string `envconfig:"RADIX_CONTAINER_REGISTRY" required:"true"`

	PipelineJobConfig       *pipelinejob.Config
	DeploymentSyncer        deployment.SyncerConfig
	ContainerRegistryConfig containerregistry.Config
	TaskConfig              *task.Config
	CertificateAutomation   CertificateAutomationConfig
	Gateway                 GatewayConfig

	// SafeToRestartBatchJobThreshold is the threshold in seconds for determining the cluster-autoscaler safe-to-evict annotation on batch jobs.
	// Jobs with timeLimitSeconds >= SafeToRestartBatchJobThreshold are marked as safe to evict.
	SafeToRestartBatchJobThreshold int64 `envconfig:"RADIXOPERATOR_SAFE_TO_RESTART_BATCH_JOB_THRESHOLD" default:"259200"`
}
type CertificateAutomationConfig struct {
	GatewayClusterIssuer string        `envconfig:"RADIXOPERATOR_CERTIFICATE_AUTOMATION_GATEWAY_CLUSTER_ISSUER" required:"true"`
	Duration             time.Duration `envconfig:"RADIXOPERATOR_CERTIFICATE_AUTOMATION_DURATION" required:"true"`
	RenewBefore          time.Duration `envconfig:"RADIXOPERATOR_CERTIFICATE_AUTOMATION_RENEW_BEFORE" required:"true"`
}
type GatewayConfig struct {
	Name        string `envconfig:"RADIXOPERATOR_INGRESS_GATEWAY_NAME" required:"true"`
	Namespace   string `envconfig:"RADIXOPERATOR_INGRESS_GATEWAY_NAMESPACE" required:"true"`
	SectionName string `envconfig:"RADIXOPERATOR_INGRESS_GATEWAY_SECTION_NAME" required:"true"`
}

func MustParse() *Config {
	var c Config
	if err := envconfig.Process("", &c); err != nil {
		_ = envconfig.Usage("", &c)
		log.Fatal().Msg(err.Error())
	}
	c.PipelineJobConfig.MustValidate()
	c.TaskConfig.MustValidate()
	return &c
}
