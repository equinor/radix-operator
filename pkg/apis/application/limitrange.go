package application

import (
	"os"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	OperatorLimitDefaultMemoryEnvironmentVariable        = "RADIXOPERATOR_APP_LIMITS_DEFAULT_MEMORY"
	OperatorLimitDefaultCPUEnvironmentVariable           = "RADIXOPERATOR_APP_LIMITS_DEFAULT_CPU"
	OperatorLimitDefaultRequestMemoryEnvironmentVariable = "RADIXOPERATOR_APP_LIMITS_DEFAULT_REQUEST_MEMORY"
	OperatorLimitDefaultReqestCPUEnvironmentVariable     = "RADIXOPERATOR_APP_LIMITS_DEFAULT_REQUEST_CPU"

	limitRangeName = "mem-cpu-limit-range-app"
)

func (app *Application) createLimitRangeOnAppNamespace(namespace string) error {
	defaultCPULimit := getDefaultCPULimit()
	defaultMemoryLimit := getDefaultMemoryLimit()
	defaultCPURequest := getDefaultCPURequest()
	defaultMemoryRequest := getDefaultMemoryRequest()

	// If not all limits are defined, then don't put any limits on namespace
	if defaultCPULimit == nil ||
		defaultMemoryLimit == nil ||
		defaultCPURequest == nil ||
		defaultMemoryRequest == nil {
		log.Warningf("Not all limits are defined for the Operator, so no limitrange will be put on namespace %s", namespace)
		return nil
	}

	limitRange := app.kubeutil.BuildLimitRange(namespace,
		limitRangeName, app.registration.Name,
		*defaultCPULimit,
		*defaultMemoryLimit,
		*defaultCPURequest,
		*defaultMemoryRequest)

	return app.kubeutil.ApplyLimitRange(namespace, limitRange)
}

func getDefaultCPULimit() *resource.Quantity {
	defaultCPULimitSetting := os.Getenv(OperatorLimitDefaultCPUEnvironmentVariable)
	if defaultCPULimitSetting == "" {
		return nil
	}

	defaultCPULimit := resource.MustParse(defaultCPULimitSetting)
	return &defaultCPULimit
}

func getDefaultMemoryLimit() *resource.Quantity {
	defaultMemoryLimitSetting := os.Getenv(OperatorLimitDefaultMemoryEnvironmentVariable)
	if defaultMemoryLimitSetting == "" {
		return nil
	}

	defaultMemoryLimit := resource.MustParse(defaultMemoryLimitSetting)
	return &defaultMemoryLimit
}

func getDefaultCPURequest() *resource.Quantity {
	defaultCPURequestSetting := os.Getenv(OperatorLimitDefaultReqestCPUEnvironmentVariable)
	if defaultCPURequestSetting == "" {
		return nil
	}

	defaultCPURequest := resource.MustParse(defaultCPURequestSetting)
	return &defaultCPURequest
}

func getDefaultMemoryRequest() *resource.Quantity {
	defaultMemoryRequestSetting := os.Getenv(OperatorLimitDefaultRequestMemoryEnvironmentVariable)
	if defaultMemoryRequestSetting == "" {
		return nil
	}

	defaultMemoryRequest := resource.MustParse(defaultMemoryRequestSetting)
	return &defaultMemoryRequest
}
