package main

type EasyAuthConfig struct {
	// When true, RadixApplications without RequiredAnnotation are rejected by the validating webhook
	Enforced   bool   `json:"enforced"`
	Annotation string `json:"annotation" required:"true"`

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
