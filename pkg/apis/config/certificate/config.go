package certificate

import "time"

type AutomationConfig struct {
	ClusterIssuer        string
	GatewayClusterIssuer string
	Duration             time.Duration
	RenewBefore          time.Duration
}
