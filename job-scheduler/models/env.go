package models

import (
	"os"
	"strconv"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/utils"
)

// Env instance variables
type Env struct {
	UseSwagger                                   bool
	UseProfiler                                  bool
	RadixComponentName                           string
	RadixDeploymentName                          string
	RadixDeploymentNamespace                     string
	RadixJobSchedulersPerEnvironmentHistoryLimit int
	RadixPort                                    *string
	LogLevel                                     string
	RadixAppName                                 string
	RadixEnvironmentName                         string
}

// NewEnv Constructor
func NewEnv() *Env {
	var (
		useSwagger, _                                = strconv.ParseBool(os.Getenv("USE_SWAGGER"))
		useProfiler, _                               = strconv.ParseBool(os.Getenv("USE_PROFILER"))
		radixAppName                                 = strings.TrimSpace(os.Getenv(defaults.RadixAppEnvironmentVariable))
		radixEnv                                     = strings.TrimSpace(os.Getenv(defaults.EnvironmentnameEnvironmentVariable))
		radixComponentName                           = strings.TrimSpace(os.Getenv(defaults.RadixComponentEnvironmentVariable))
		radixDeployment                              = strings.TrimSpace(os.Getenv(defaults.RadixDeploymentEnvironmentVariable))
		radixJobSchedulersPerEnvironmentHistoryLimit = strings.TrimSpace(os.Getenv("RADIX_JOB_SCHEDULERS_PER_ENVIRONMENT_HISTORY_LIMIT"))
		logLevel                                     = os.Getenv("LOG_LEVEL")
	)
	env := Env{
		RadixAppName:             radixAppName,
		RadixEnvironmentName:     radixEnv,
		RadixComponentName:       radixComponentName,
		RadixDeploymentName:      radixDeployment,
		RadixDeploymentNamespace: utils.GetEnvironmentNamespace(radixAppName, radixEnv),
		UseSwagger:               useSwagger,
		RadixJobSchedulersPerEnvironmentHistoryLimit: 10,
		LogLevel:    logLevel,
		UseProfiler: useProfiler,
	}
	setHistoryLimit(radixJobSchedulersPerEnvironmentHistoryLimit, &env)
	return &env
}

func setHistoryLimit(radixJobSchedulersPerEnvironmentHistoryLimit string, env *Env) {
	if len(radixJobSchedulersPerEnvironmentHistoryLimit) > 0 {
		if historyLimit, err := strconv.Atoi(radixJobSchedulersPerEnvironmentHistoryLimit); err == nil && historyLimit > 0 {
			env.RadixJobSchedulersPerEnvironmentHistoryLimit = historyLimit
		}
	}
}
