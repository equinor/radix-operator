package defaults

import (
	"fmt"
	"os"
	"strconv"
)

// GetEnvVar returns the string value of an environment variable. Error is returned if environment variable is not set.
func GetEnvVar(name string) (string, error) {
	envVar := os.Getenv(name)
	if len(envVar) > 0 {
		return envVar, nil
	}
	return "", fmt.Errorf("not set environment variable %s", name)
}

// GetIntEnvVar returns the integer value of an environment variable. Error is returned if environment variable is not
// set, or if the value is not a valid integer.
func GetIntEnvVar(name string) (int, error) {
	envVarStr, err := GetEnvVar(name)
	if err != nil {
		return 0, err
	}

	envVarInt, err := strconv.Atoi(envVarStr)
	if err != nil {
		return 0, err
	}

	return envVarInt, nil
}
