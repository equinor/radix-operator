package defaults

import "os"

// Environment variables that define default rolling update parameters for containers
const (
	OperatorRollingUpdateMaxUnavailable = "RADIXOPERATOR_APP_ROLLING_UPDATE_MAX_UNAVAILABLE"
	OperatorRollingUpdateMaxSurge       = "RADIXOPERATOR_APP_ROLLING_UPDATE_MAX_SURGE"
	RollingUpdateMaxUnavailableDefault  = "25%"
	RollingUpdateMaxSurgeDefault        = "25%"
)

// GetDefaultRollingUpdateMaxUnavailable Gets the default rolling update max unavailable defined as an environment variable
func GetDefaultRollingUpdateMaxUnavailable() string {
	defaultRollingUpdateMaxUnavailable := os.Getenv(OperatorRollingUpdateMaxUnavailable)
	if defaultRollingUpdateMaxUnavailable == "" {
		return RollingUpdateMaxUnavailableDefault
	}
	return defaultRollingUpdateMaxUnavailable
}

// GetDefaultRollingUpdateMaxSurge Gets the default rolling update max surge defined as an environment variable
func GetDefaultRollingUpdateMaxSurge() string {
	defaultRollingUpdateMaxSurge := os.Getenv(OperatorRollingUpdateMaxSurge)
	if defaultRollingUpdateMaxSurge == "" {
		return RollingUpdateMaxSurgeDefault
	}
	return defaultRollingUpdateMaxSurge
}
