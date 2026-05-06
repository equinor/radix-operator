package certificate

import "time"

type AutomationConfig struct {
	GatewayClusterIssuer string
	Duration             time.Duration
	RenewBefore          time.Duration
}
