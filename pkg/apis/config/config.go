package config

type Config struct {
	Operator       OperatorConfig       `json:"operator"`
	PipelineRunner PipelineRunnerConfig `json:"pipelineRunner"`
	Common         CommonConfig         `json:"common"`
	Webhook        WebhookConfig        `json:"webhook"`
	ApiServer      ApiServerConfig      `json:"apiServer"`
}
