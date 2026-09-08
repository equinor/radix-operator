package config

type DeploymentSyncerConfig struct {
	DeploymentHistoryLimit int   `envconfig:"RADIX_DEPLOYMENTS_PER_ENVIRONMENT_HISTORY_LIMIT" required:"true"`
}
