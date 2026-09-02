package application

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
)

const (
	limitRangeName = "mem-cpu-limit-range-app"
)

func (app *Application) createLimitRangeOnAppNamespace(ctx context.Context) error {
	namespace := utils.GetAppNamespace(app.registration.Name)
	defaultMemoryLimit := app.config2.Operator.AppNsLimitRange.DefaultMemory
	defaultCPURequest := defaults.GetDefaultCPURequestForAppNamespace()
	defaultMemoryRequest := app.config2.Operator.AppNsLimitRange.DefaultRequestMemory

	// If not all limits are defined, then don't put any limits on namespace
	if defaultCPURequest == nil {
		log.Ctx(ctx).Warn().Msgf("Not all limits are defined for the Operator, so no limitrange will be put on namespace %s", namespace)
		return nil
	}

	limitRange := app.kubeutil.BuildLimitRange(namespace, limitRangeName, app.registration.Name, defaultMemoryLimit, defaultCPURequest, defaultMemoryRequest)

	return app.kubeutil.ApplyLimitRange(ctx, namespace, limitRange)
}
