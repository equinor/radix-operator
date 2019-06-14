package defaults

import (
	"os"
	"strconv"
)

// Environment variables that define default readiness probe parameters for containers
const (
	OperatorReadinessProbeInitialDelaySeconds = "RADIXOPERATOR_APP_READINESS_PROBE_INITIAL_DELAY_SECONDS"
	OperatorReadinessProbePeriodSeconds       = "RADIXOPERATOR_APP_READINESS_PROBE_PERIOD_SECONDS"
)

// GetDefaultReadinessProbeInitialDelaySeconds Gets the default readiness probe initial delay seconds defined as an environment variable
func GetDefaultReadinessProbeInitialDelaySeconds() (int32, error) {
	initialDelaySecondsString := os.Getenv(OperatorReadinessProbeInitialDelaySeconds)
	initialDelaySecondsInt, err := strconv.Atoi(initialDelaySecondsString)
	if err != nil {
		return 0, err
	}
	return int32(initialDelaySecondsInt), nil
}

// GetDefaultReadinessProbePeriodSeconds Gets the default readiness probe period seconds defined as an environment variable
func GetDefaultReadinessProbePeriodSeconds() (int32, error) {
	periodSecondsString := os.Getenv(OperatorReadinessProbePeriodSeconds)
	periodSecondsInt, err := strconv.Atoi(periodSecondsString)
	if err != nil {
		return 0, err
	}
	return int32(periodSecondsInt), nil
}
