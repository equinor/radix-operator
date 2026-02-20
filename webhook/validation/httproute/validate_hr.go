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
		existingHttpRoutes := &gatewayapiv1.HTTPRouteList{}
		if err := kubeClient.List(ctx, existingHttpRoutes); err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("failed to list httproutes")
			return nil, []error{fmt.Errorf("failed to list httproutes: %w", err)}
		}

		var existingHostnames []string
		for _, existingRoute := range existingHttpRoutes.Items {
			if existingRoute.Namespace == route.Namespace {
				continue
			}
			for _, hostname := range existingRoute.Spec.Hostnames {
				existingHostnames = append(existingHostnames, normalizeHostname(hostname))
			}
		}

		var errs []error
		for _, incomingHostname := range route.Spec.Hostnames {
			if err := validateHostname(normalizeHostname(incomingHostname), existingHostnames, incomingHostname); err != nil {
				errs = append(errs, err)
			}
		}

		return nil, errs
	}
}

func normalizeHostname(hostname gatewayapiv1.Hostname) string {
	return strings.ToLower(string(hostname))
}

func validateHostname(incomingHostname string, existing []string, original gatewayapiv1.Hostname) error {
	for _, existingHostname := range existing {
		incomingToCompare := incomingHostname
		existingToCompare := existingHostname

		// When either hostname is a wildcard, compare their apex domains
		// E.g., "*.example.com" conflicts with "api.example.com"
		if isWildcardDomain(incomingHostname) || isWildcardDomain(existingHostname) {
			incomingToCompare = getApexDomain(incomingHostname)
			existingToCompare = getApexDomain(existingHostname)
		}

		if incomingToCompare == existingToCompare {
			return fmt.Errorf("failed to validate hostname %s: %w", original, ErrDuplicateHostname)
		}
	}
	return nil
}

func isWildcardDomain(fqdn string) bool {
	return len(fqdn) > 0 && fqdn[0] == '*'
}

func getApexDomain(fqdn string) string {
	parts := strings.Split(fqdn, ".")

	if len(parts) >= 2 {
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}

	return fqdn
}
