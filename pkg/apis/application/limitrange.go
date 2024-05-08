package application

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
)

const (
	limitRangeName = "mem-cpu-limit-range-app"
)

func (app *Application) createLimitRangeOnAppNamespace(ctx context.Context, namespace string) error {
	defaultMemoryLimit := defaults.GetDefaultMemoryLimitForAppNamespace()
	defaultCPURequest := defaults.GetDefaultCPURequestForAppNamespace()
	defaultMemoryRequest := defaults.GetDefaultMemoryRequestForAppNamespace()

	// If not all limits are defined, then don't put any limits on namespace
	if defaultMemoryLimit == nil ||
		defaultCPURequest == nil ||
		defaultMemoryRequest == nil {
		app.logger.Warn().Msgf("Not all limits are defined for the Operator, so no limitrange will be put on namespace %s", namespace)
		return nil
	}

	limitRange := app.kubeutil.BuildLimitRange(namespace, limitRangeName, app.registration.Name, defaultMemoryLimit, defaultCPURequest, defaultMemoryRequest)

	return app.kubeutil.ApplyLimitRange(ctx, namespace, limitRange)
}
