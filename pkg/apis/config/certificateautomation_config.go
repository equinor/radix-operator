package config

import "time"

type CertificateAutomationConfig struct {
	GatewayClusterIssuer string        `envconfig:"RADIXOPERATOR_CERTIFICATE_AUTOMATION_GATEWAY_CLUSTER_ISSUER" required:"true"`
	Duration             time.Duration `envconfig:"RADIXOPERATOR_CERTIFICATE_AUTOMATION_DURATION" required:"true"`
	RenewBefore          time.Duration `envconfig:"RADIXOPERATOR_CERTIFICATE_AUTOMATION_RENEW_BEFORE" required:"true"`
}
