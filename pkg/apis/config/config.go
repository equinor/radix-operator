package config

import (
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

	PipelineJobConfig       PipelineJobConfig
	DeploymentSyncer        DeploymentSyncerConfig
	ContainerRegistryConfig ContainerRegistryConfig
	TaskConfig              TaskConfig
	CertificateAutomation   CertificateAutomationConfig
	Gateway                 GatewayConfig

	// SafeToRestartBatchJobThreshold is the threshold in seconds for determining the cluster-autoscaler safe-to-evict annotation on batch jobs.
	// Jobs with timeLimitSeconds >= SafeToRestartBatchJobThreshold are marked as safe to evict.
	SafeToRestartBatchJobThreshold int64 `envconfig:"RADIXOPERATOR_SAFE_TO_RESTART_BATCH_JOB_THRESHOLD" default:"259200"`
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
