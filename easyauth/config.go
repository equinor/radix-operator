package main

import "time"

type EasyAuthConfig struct {
	// When true, RadixApplications without RequiredAnnotation are rejected by the validating webhook
	Enforced   bool   `json:"enforced"`
	Annotation string `json:"annotation" required:"true"`
	IssuerURL  string `json:"issuerUrl" required:"true"`

	// Must match config.common of radix-operator in the same cluster, used to build the redirect URIs of public hosts
	ClusterName     string `json:"clusterName" required:"true"`
	DNSZone         string `json:"dnsZone" required:"true"`
	AppAliasBaseURL string `json:"appAliasBaseURL" required:"true"`

	LeaderId string `json:"leaderId"`
	// Validated here since validation is skipped for zero-value fields (empty leaderId)
	EnableLeaderElection bool `json:"enableLeaderElection" validate:"!self || config.leaderId != ''"`

	LogLevel       string `json:"logLevel" required:"true"`
	LogPrettyPrint bool   `json:"logPrettyPrint"`
	Port           int    `json:"port" required:"true"`
	MetricsPort    int    `json:"metricsPort" required:"true"`
	HealthPort     int    `json:"healthPort" required:"true"`

	CertsDir string `json:"certsDir" required:"true"`

	// Set by the Helm chart from the Dex installed by the chart
	Dex DexConfig `json:"dex" required:"true"`

	// When true, Dex clients that no longer belong to an application are deleted. Must only be enabled in one
	// cluster when several clusters share the Dex database, see easyauth/README.md.
	CleanupEnabled bool `json:"cleanupEnabled" validate:"!self || compareDuration(config.cleanupInterval, '1m') >= 0"`
	// How often the cleanup runs. Required, and at least 1m, when CleanupEnabled is true.
	CleanupInterval time.Duration `json:"cleanupInterval"`

	// How old a client secret can be before it is rotated. Must be the same in all clusters sharing the Dex database.
	SecretRotationInterval time.Duration `json:"secretRotationInterval" required:"true" validate:"compareDuration(self, '1h') >= 0"`
}

// DexConfig is the connection to the Dex gRPC API, authenticated with a client certificate
type DexConfig struct {
	GRPCAddress string `json:"grpcAddress" required:"true"`
	// Directory with tls.crt, tls.key and ca.crt for the gRPC client certificate
	CertsDir string `json:"certsDir" required:"true"`
}
