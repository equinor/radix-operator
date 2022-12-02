package job

// Config for pipeline jobs
type Config struct {
	PipelineJobsHistoryLimit              int
	DeploymentsHistoryLimitPerEnvironment int
}
