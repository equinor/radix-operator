package application

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	log "github.com/sirupsen/logrus"
)

const (
	limitRangeName = "mem-cpu-limit-range-app"
)

func (app *Application) createLimitRangeOnAppNamespace(namespace string) error {
	defaultCPULimit := defaults.GetDefaultCPULimit()
	defaultMemoryLimit := defaults.GetDefaultMemoryLimit()
	defaultCPURequest := defaults.GetDefaultCPURequest()
	defaultMemoryRequest := defaults.GetDefaultMemoryRequest()

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
