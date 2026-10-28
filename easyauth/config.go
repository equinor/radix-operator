package main

type EasyAuthConfig struct {
	Enforced bool `json:"enforced"`

	LeaderId string `json:"leaderId"`
	// Validated here since validation is skipped for zero-value fields (empty leaderId)
	EnableLeaderElection bool `json:"enableLeaderElection" validate:"!self || config.leaderId != ''"`

	LogLevel       string `json:"logLevel" required:"true"`
	LogPrettyPrint bool   `json:"logPrettyPrint"`
	Port           int    `json:"port" required:"true"`
	MetricsPort    int    `json:"metricsPort" required:"true"`
	HealthPort     int    `json:"healthPort" required:"true"`

	CertsDir string `json:"certsDir" required:"true"`
}
