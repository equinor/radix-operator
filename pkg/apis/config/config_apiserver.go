package config

import "net/url"

type ApiServerConfig struct {
	Port           int    `json:"port" required:"true"`
	MetricsPort    int    `json:"metricsPort" required:"true"`
	ProfilerPort   int    `json:"profilerPort" required:"true"`
	UseProfiler    bool   `json:"useProfiler"`
	LogLevel       string `json:"logLevel" required:"true"`
	LogPrettyPrint bool   `json:"logPrettyPrint"`

	ClusterEgressIps   []string `json:"clusterEgressIPs" required:"true"`
	ClusterOidcIssuers []string `json:"clusterOidcIssuers" required:"true"`

	AzureOidc      OidcConfig `json:"azureOidc" required:"true"`
	KubernetesOidc OidcConfig `json:"kubernetesOidc" required:"true"`
	PrometheusUrl  url.URL    `json:"prometheusUrl" required:"true"`
	PodNamespace   string     `json:"podNamespace" required:"true"`
}

type OidcConfig struct {
	Issuer   url.URL `json:"issuer" required:"true"`
	Audience string  `json:"audience" required:"true"`
}
