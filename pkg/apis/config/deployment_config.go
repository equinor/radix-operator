package config

type DeploymentSyncerConfig struct {
	KubernetesAPIPort      int32 `envconfig:"KUBERNETES_SERVICE_PORT" required:"true"`
	DeploymentHistoryLimit int   `envconfig:"RADIX_DEPLOYMENTS_PER_ENVIRONMENT_HISTORY_LIMIT" required:"true"`
}
