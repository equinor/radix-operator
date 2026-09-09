package config2

import "time"

type CertificateAutomationConfig struct {
	GatewayClusterIssuer string        `json:"gatewayClusterIssuer" required:"true"`
	Duration             time.Duration `json:"duration" required:"true"`
	RenewBefore          time.Duration `json:"renewBefore" required:"true"`
}
