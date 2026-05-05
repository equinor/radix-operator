package certificate

import "time"

type AutomationConfig struct {
	// Deprecated: ClusterIssuer is deprecated and will be removed in a future release. Use GatewayClusterIssuer instead.
	ClusterIssuer        string
	GatewayClusterIssuer string
	Duration             time.Duration
	RenewBefore          time.Duration
}
