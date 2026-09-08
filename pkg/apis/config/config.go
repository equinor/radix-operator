package config

import (
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

// Config from environment variables
type Config struct {
	PipelineJobConfig       PipelineJobConfig
	DeploymentSyncer        DeploymentSyncerConfig
	ContainerRegistryConfig ContainerRegistryConfig
	TaskConfig              TaskConfig
	CertificateAutomation   CertificateAutomationConfig
	Gateway                 GatewayConfig
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
