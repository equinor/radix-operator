package defaults

import (
	"fmt"
	"os"
)

// Environment variables that define default rolling update parameters for containers
const (
	OperatorRollingUpdateMaxUnavailable = "RADIXOPERATOR_APP_ROLLING_UPDATE_MAX_UNAVAILABLE"
	OperatorRollingUpdateMaxSurge       = "RADIXOPERATOR_APP_ROLLING_UPDATE_MAX_SURGE"
)

// GetDefaultRollingUpdateMaxUnavailable Gets the default rolling update max unavailable defined as an environment variable
func GetDefaultRollingUpdateMaxUnavailable() (string, error) {
	defaultRollingUpdateMaxUnavailable := os.Getenv(OperatorRollingUpdateMaxUnavailable)
	if defaultRollingUpdateMaxUnavailable == "" {
		return "", fmt.Errorf("empty rolling update max unavailable")
	}
	return defaultRollingUpdateMaxUnavailable, nil
}

// GetDefaultRollingUpdateMaxSurge Gets the default rolling update max surge defined as an environment variable
func GetDefaultRollingUpdateMaxSurge() (string, error) {
	defaultRollingUpdateMaxSurge := os.Getenv(OperatorRollingUpdateMaxSurge)
	if defaultRollingUpdateMaxSurge == "" {
		return "", fmt.Errorf("empty rolling update max unavailable")
	}
	return defaultRollingUpdateMaxSurge, nil
}
