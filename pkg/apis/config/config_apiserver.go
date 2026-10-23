package config

import "net/url"

type ApiServerConfig struct {
	Port           int    `json:"port" required:"true"`
	MetricsPort    int    `json:"metricsPort" required:"true"`
	UseProfiler    bool   `json:"useProfiler"`
	LogLevel       string `json:"logLevel" required:"true"`
	LogPrettyPrint bool   `json:"logPrettyPrint"`

	ClusterEgressIps   []string `json:"clusterEgressIps" required:"true"`
	ClusterOidcIssuers []string `json:"clusterOidcIssuers" required:"true"`

	Authenticators map[string]OidcAuthenticatorConfig `json:"authenticators" required:"true" validate:"size(self) > 0"`
	PrometheusUrl  url.URL                            `json:"prometheusUrl" required:"true"`
	PodNamespace   string                             `json:"podNamespace" required:"true"`
}

type OidcAuthenticatorConfig struct {
	Issuer   url.URL `json:"issuer" required:"true"`
	Audience string  `json:"audience" required:"true"`
}
