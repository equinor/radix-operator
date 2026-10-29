package main

type EasyAuthConfig struct {
	// When true, RadixApplications without RequiredAnnotation are rejected by the validating webhook
	Enforced   bool   `json:"enforced"`
	Annotation string `json:"annotation" required:"true"`
	IssuerURL  string `json:"issuerUrl" required:"true"`

	LeaderId string `json:"leaderId"`
	// Validated here since validation is skipped for zero-value fields (empty leaderId)
	EnableLeaderElection bool `json:"enableLeaderElection" validate:"!self || config.leaderId != ''"`

	LogLevel       string `json:"logLevel" required:"true"`
	LogPrettyPrint bool   `json:"logPrettyPrint"`
	Port           int    `json:"port" required:"true"`
	MetricsPort    int    `json:"metricsPort" required:"true"`
	HealthPort     int    `json:"healthPort" required:"true"`

	CertsDir string `json:"certsDir" required:"true"`

	// Set by the Helm chart when Dex is enabled
	Dex DexConfig `json:"dex"`
}

// DexConfig is the connection to the Dex gRPC API, authenticated with a client certificate
type DexConfig struct {
	GRPCAddress string `json:"grpcAddress"`
	// Directory with tls.crt, tls.key and ca.crt for the gRPC client certificate
	CertsDir string `json:"certsDir"`
}
