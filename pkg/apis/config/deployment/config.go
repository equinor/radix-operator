package deployment

type SyncerConfig struct {
	TenantID               string
	KubernetesAPIPort      int32
	DeploymentHistoryLimit int
}
