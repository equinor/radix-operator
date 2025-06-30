package config

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

type Config struct {
	Port           int    `envconfig:"PORT" default:"9443" desc:"The address the health endpoint binds to"`
	MetricsPort    int    `envconfig:"METRICS_PORT" default:"9000"  desc:"The address the metric endpoint binds to."`
	HealthPort     int    `envconfig:"HEALTH_PORT" default:"9440" desc:"The address the health endpoint binds to"`
	LogLevel       string `envconfig:"LOG_LEVEL" default:"info"`
	LogPrettyPrint bool   `envconfig:"LOG_PRETTY" default:"false"`

	RequireAdGroups          bool `envconfig:"REQUIRE_AD_GROUPS" default:"false" desc:"Require AD groups for authentication"`
	RequireConfigurationItem bool `envconfig:"REQUIRE_CONFIGURATION_ITEM" default:"false" desc:"Require configuration item for authentication"`

	SecretName      string `envconfig:"SECRET_NAME" default:"radix-webhook-certs" desc:"Name of the secret where the webhook TLS certificate is stored"`
	SecretNamespace string `envconfig:"SECRET_NAMESPACE" default:"default" desc:"Namespace of the secret where the webhook TLS certificate is stored"`

	DisableCertRotation      bool     `envconfig:"DISABLE_CERT_ROTATION" default:"false" desc:"Disable automatic certificate rotation"`
	CertDir                  string   `envconfig:"CERT_DIR" default:"/certs" desc:"Directory where the webhook TLS certificate is stored"`
	CaName                   string   `envconfig:"CA_NAME" default:"radix-webhook-ca" desc:"Name of the CA secret"`
	CaOrganization           string   `envconfig:"CA_ORGANIZATION" default:"Radix Webhook CA" desc:"Organization of the CA"`
	DnsName                  string   `envconfig:"DNS_NAME" default:"radix-webhook.default.svc" desc:"DNS name of the webhook service"`
	ExtraDnsNames            []string `envconfig:"EXTRA_DNS_NAMES" default:"" desc:"Additional DNS names for the webhook service, separated by commas"`
	WebhookConfigurationName string   `envconfig:"WEBHOOK_CONFIGURATION_NAME" default:"radix-webhook-configuration" desc:"Name of the webhook service"`
}

func MustParseConfig() Config {
	var s Config
	err := envconfig.Process("", &s)
	if err != nil {
		_ = envconfig.Usage("", &s)
		log.Fatal().Msg(err.Error())
	}

	if s.CertDir == "" {
		s.CertDir = os.TempDir() + "/k8s-webhook-server/serving-certs"
		log.Warn().Msgf("CERT_DIR is not set, using default: %s", s.CertDir)
	}

	return s
}

func ParseVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev (no build info)"
	}

	commit := "unknown"
	modified := ""
	version := "dev"

	if info.Main.Version != "" {
		version = info.Main.Version
	}

	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			commit = setting.Value
		case "vcs.modified":
			if setting.Value == "true" {
				modified = "-modified"
			}
		}
	}

	version = fmt.Sprintf("%s (%s%s)", version, commit, modified)

	return version
}
