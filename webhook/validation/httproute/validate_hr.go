package httproute

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/webhook/validation/genericvalidator"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// validatorFunc defines a validatorFunc function for a HTTPRoute
type validatorFunc func(ctx context.Context, httpRoute *gatewayapiv1.HTTPRoute) ([]string, []error)

type Validator struct {
	validators []validatorFunc
}

var _ genericvalidator.Validator[*gatewayapiv1.HTTPRoute] = &Validator{}

func CreateOnlineValidator(client client.Client) *Validator {
	return &Validator{
		validators: []validatorFunc{
			createHttpRouteUsableValidator(client),
		},
	}
}

func (validator *Validator) Validate(ctx context.Context, hr *gatewayapiv1.HTTPRoute) (admission.Warnings, error) {
	var errs []error
	var wrns admission.Warnings
	for _, v := range validator.validators {
		wrn, err := v(ctx, hr)
		if len(err) > 0 {
			errs = append(errs, err...)
		}
		if len(wrn) > 0 {
			wrns = append(wrns, wrn...)
		}
	}

	return wrns, errors.Join(errs...)
}

func createHttpRouteUsableValidator(kubeClient client.Client) validatorFunc {
	return func(ctx context.Context, route *gatewayapiv1.HTTPRoute) ([]string, []error) {
		var errs []error

		existingHttpRoutes := &gatewayapiv1.HTTPRouteList{}
		if err := kubeClient.List(ctx, existingHttpRoutes); err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("failed to list existing HttpRoutes")
			return nil, []error{ErrInternalError}
		}

		existingHostnames := make(map[string]bool)
		for _, existingRoute := range existingHttpRoutes.Items {
			for _, hostname := range existingRoute.Spec.Hostnames {
				normalizedHostname := normalizeHostname(hostname)
				existingHostnames[normalizedHostname] = true
			}
		}

		for _, hostname := range route.Spec.Hostnames {
			normalizedHostname := normalizeHostname(hostname)
			if existingHostnames[normalizedHostname] {
				errs = append(errs, fmt.Errorf("hostname %s is already in use", hostname))
			}
		}

		return nil, errs
	}
}

func normalizeHostname(hostname gatewayapiv1.Hostname) string {
	h := strings.ToLower(string(hostname))
	return strings.TrimSuffix(h, ".")
}
