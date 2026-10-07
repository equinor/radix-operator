package config

import "time"

type CertificateAutomationConfig struct {
	DefaultIssuer string                             `json:"defaultIssuer" validate:"self in config.common.certificateAutomation.issuers"`
	Issuers       map[string]CertificateIssuerConfig `json:"issuers"`
}

type CertificateIssuerConfig struct {
	ClusterIssuerName string        `json:"clusterIssuerName" required:"true"`
	Duration          time.Duration `json:"duration" required:"true"`
	RenewBefore       time.Duration `json:"renewBefore" required:"true"`
}
