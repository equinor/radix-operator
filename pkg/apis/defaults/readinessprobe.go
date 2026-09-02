package defaults

import (
	"os"
	"strconv"
)

// Environment variables that define default readiness probe parameters for containers
const (
	OperatorReadinessProbePeriodSeconds = "RADIXOPERATOR_APP_READINESS_PROBE_PERIOD_SECONDS"
)

// GetDefaultReadinessProbePeriodSeconds Gets the default readiness probe period seconds defined as an environment variable
func GetDefaultReadinessProbePeriodSeconds() (int32, error) {
	periodSecondsString := os.Getenv(OperatorReadinessProbePeriodSeconds)
	periodSecondsInt, err := strconv.ParseInt(periodSecondsString, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(periodSecondsInt), nil
}
