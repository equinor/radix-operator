package config

import (
	"github.com/equinor/radix-operator/pkg/apis/config/certificate"
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

	PipelineJobConfig       *pipelinejob.Config          `ignored:"true"`
	DeploymentSyncer        deployment.SyncerConfig      `ignored:"true"`
	ContainerRegistryConfig containerregistry.Config     `ignored:"true"`
	TaskConfig              *task.Config                 `ignored:"true"`
	CertificateAutomation   certificate.AutomationConfig `envconfig:"RADIXOPERATOR_CERTIFICATE_AUTOMATION"`
	Gateway                 GatewayConfig                `envconfig:"RADIXOPERATOR_INGRESS_GATEWAY"`

	// SafeToRestartBatchJobThreshold is the threshold in seconds for determining the cluster-autoscaler safe-to-evict annotation on batch jobs.
	// Jobs with timeLimitSeconds >= SafeToRestartBatchJobThreshold are marked as safe to evict.
	SafeToRestartBatchJobThreshold int64 `envconfig:"RADIXOPERATOR_SAFE_TO_RESTART_BATCH_JOB_THRESHOLD" default:"259200"`
}

type GatewayConfig struct {
	Name        string `envconfig:"NAME" required:"true"`
	Namespace   string `envconfig:"NAMESPACE" required:"true"`
	SectionName string `envconfig:"SECTION_NAME" required:"true"`
}

func MustParse() *Config {
	var c Config
	if err := envconfig.Process("", &c); err != nil {
		_ = envconfig.Usage("", &c)
		log.Fatal().Msg(err.Error())
	}

	c.PipelineJobConfig = pipelinejob.MustParseConfig()

	var ds deployment.SyncerConfig
	if err := envconfig.Process("", &ds); err != nil {
		_ = envconfig.Usage("", &ds)
		log.Fatal().Msg(err.Error())
	}
	c.DeploymentSyncer = ds

	var cr containerregistry.Config
	if err := envconfig.Process("", &cr); err != nil {
		_ = envconfig.Usage("", &cr)
		log.Fatal().Msg(err.Error())
	}
	c.ContainerRegistryConfig = cr

	c.TaskConfig = task.MustParseConfig()

	return &c
}
