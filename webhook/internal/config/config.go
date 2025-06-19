package config

import (
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

type Config struct {
	Port           int    `envconfig:"PORT" default:"3002" desc:"Port where API will be served"`
	MetricsPort    int    `envconfig:"METRICS_PORT" default:"9090"  desc:"Port where Metrics will be served"`
	ProfilePort    int    `envconfig:"PROFILE_PORT" default:"7070"  desc:"Port where Profiler will be served"`
	UseProfiler    bool   `envconfig:"USE_PROFILER" default:"false" desc:"Enable Profiler"`
	LogLevel       string `envconfig:"LOG_LEVEL" default:"info"`
	LogPrettyPrint bool   `envconfig:"LOG_PRETTY" default:"false"`

	SecretName         string   `envconfig:"SECRET_NAME" default:"radix-webhook-certs" desc:"Name of the secret where the webhook TLS certificate is stored"`
	SecretNamespace    string   `envconfig:"SECRET_NAMESPACE" default:"default" desc:"Namespace of the secret where the webhook TLS certificate is stored"`
	CaName             string   `envconfig:"CA_NAME" default:"radix-webhook-ca" desc:"Name of the CA secret"`
	CaOrganization     string   `envconfig:"CA_ORGANIZATION" default:"Radix Webhook CA" desc:"Organization of the CA"`
	DnsName            string   `envconfig:"DNS_NAME" default:"radix-webhook.default.svc" desc:"DNS name of the webhook service"`
	ExtraDnsNames      []string `envconfig:"EXTRA_DNS_NAMES" default:"" desc:"Additional DNS names for the webhook service, separated by commas"`
	WebhookServiceName string   `envconfig:"WEBHOOK_SERVICE_NAME" default:"radix-webhook" desc:"Name of the webhook service"`
}

func MustParseConfig() Config {
	var s Config
	err := envconfig.Process("", &s)
	if err != nil {
		_ = envconfig.Usage("", &s)
		log.Fatal().Msg(err.Error())
	}

	return s
}
