package application

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	log "github.com/sirupsen/logrus"
)

const (
	limitRangeName = "mem-cpu-limit-range-app"
)

func (app *Application) createLimitRangeOnAppNamespace(namespace string) error {
	defaultCPURequest := defaults.GetDefaultCPURequestForAppNamespace()
	defaultMemoryRequest := defaults.GetDefaultMemoryRequestForAppNamespace()

	// If not all limits are defined, then don't put any limits on namespace
	if defaultCPURequest == nil ||
		defaultMemoryRequest == nil {
		log.Warningf("Not all limits are defined for the Operator, so no limitrange will be put on namespace %s", namespace)
		return nil
	}

	limitRange := app.kubeutil.BuildLimitRange(namespace, limitRangeName, app.registration.Name, defaultCPURequest, defaultMemoryRequest)

	return app.kubeutil.ApplyLimitRange(namespace, limitRange)
}
