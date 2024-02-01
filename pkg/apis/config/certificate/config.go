package certificate

import "time"

type AutomationConfig struct {
	ClusterIssuer string
	Duration      time.Duration
	RenewBefore   time.Duration
}
