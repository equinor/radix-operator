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
	defaultCPU := getDefaultCPU()
	defaultMemory := getDefaultMemory()
	defaultCPURequest := getDefaultCPURequest()
	defaultMemoryRequest := getDefaultMemoryRequest()

	// If not all limits are defined, then don't put any limits on namespace
	if defaultCPU == nil ||
		defaultMemory == nil ||
		defaultCPURequest == nil ||
		defaultMemoryRequest == nil {
		log.Warningf("Not all limits are defined for the Operator, so no limitrange will be put on namespace %s", namespace)
		return nil
	}

	limitRange := app.kubeutil.BuildLimitRange(namespace,
		limitRangeName, app.registration.Name,
		*defaultCPU,
		*defaultMemory,
		*defaultCPURequest,
		*defaultMemoryRequest)

	return app.kubeutil.ApplyLimitRange(namespace, limitRange)
}

func getDefaultCPU() *resource.Quantity {
	defaultCPUSetting := os.Getenv(OperatorLimitDefaultCPUEnvironmentVariable)
	if defaultCPUSetting == "" {
		return nil
	}

	defaultCPU := resource.MustParse(defaultCPUSetting)
	return &defaultCPU
}

func getDefaultMemory() *resource.Quantity {
	defaultMemorySetting := os.Getenv(OperatorLimitDefaultMemoryEnvironmentVariable)
	if defaultMemorySetting == "" {
		return nil
	}

	defaultMemory := resource.MustParse(defaultMemorySetting)
	return &defaultMemory
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
