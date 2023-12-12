package deployment

import "time"

type CertificateAutomationConfig struct {
	ClusterIssuer string
	Duration      time.Duration
	RenewBefore   time.Duration
}
