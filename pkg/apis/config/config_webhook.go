package config

type WebhookConfig struct {
	RequireGroups            bool `json:"requireGroups"`
	RequireConfigurationItem bool `json:"requireConfigurationItem"`

	LogLevel       string `json:"logLevel" required:"true"`
	LogPrettyPrint bool   `json:"logPrettyPrint"`
	Port           int    `json:"port" required:"true"`
	MetricsPort    int    `json:"metricsPort" required:"true"`
	HealthPort     int    `json:"healthPort" required:"true"`

	SecretName                         string   `json:"secretName" required:"true"`
	SecretNamespace                    string   `json:"secretNamespace" required:"true"`
	CertsDir                           string   `json:"certsDir" required:"true"`
	DisableCertRotation                bool     `json:"disableCertRotation"`
	CAName                             string   `json:"caName" required:"true"`
	CAOrganization                     string   `json:"caOrganization" required:"true"`
	DNSName                            string   `json:"dnsName" required:"true"`
	ExtraDNSNames                      []string `json:"extraDnsNames" required:"true"`
	ValidatingWebhookConfigurationName string   `json:"validatingWebhookConfigurationName" required:"true"`

	ReservedDNSAliases    []string          `json:"reservedDNSAliases"`
	ReservedDNSAppAliases map[string]string `json:"reservedAppDNSAliases"`
}
