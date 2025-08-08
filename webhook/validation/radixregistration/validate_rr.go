package radixregistration

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/webhook/validation/genericvalidator"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// validatorFunc defines a validatorFunc function for a RadixRegistration
type validatorFunc func(ctx context.Context, radixRegistration *radixv1.RadixRegistration) (string, error)

type Validator struct {
	validators []validatorFunc
}

var _ genericvalidator.Validator[*radixv1.RadixRegistration] = &Validator{}

func CreateOnlineValidator(kubeClient client.Client, httpClient *http.Client, requireAdGroups, requireConfigurationItem bool, validateConfigurationItem *url.URL) *Validator {
	return &Validator{
		validators: []validatorFunc{
			createRequireUniqueAppIdValidator(kubeClient),
			createRequireAdGroupsValidator(requireAdGroups),
			CreateRequireConfigurationItemValidator(requireConfigurationItem),
			createConfigurationItemUrlValidator(httpClient, validateConfigurationItem),
		},
	}
}

func CreateOfflineValidator(requireAdGroups, requireConfigurationItem bool) Validator {
	return Validator{
		validators: []validatorFunc{
			createRequireAdGroupsValidator(requireAdGroups),
			CreateRequireConfigurationItemValidator(requireConfigurationItem),
		},
	}
}

func (validator *Validator) Validate(ctx context.Context, rr *radixv1.RadixRegistration) (admission.Warnings, error) {
	var errs []error
	var wrns admission.Warnings
	for _, v := range validator.validators {
		wrn, err := v(ctx, rr)
		if err != nil {
			errs = append(errs, err)
		}
		if wrn != "" {
			wrns = append(wrns, wrn)
		}
	}

	return wrns, errors.Join(errs...)
}

// RequireAdGroups validates that AdGroups contains minimum one item
func createRequireAdGroupsValidator(required bool) validatorFunc {
	return func(ctx context.Context, rr *radixv1.RadixRegistration) (string, error) {
		if len(rr.Spec.AdGroups) == 0 && required {
			return "", ErrAdGroupIsRequired
		}

		if len(rr.Spec.AdGroups) == 0 && !required {
			return WarningAdGroupsShouldHaveAtleastOneItem, nil
		}

		return "", nil
	}
}

func CreateRequireConfigurationItemValidator(required bool) validatorFunc {
	return func(ctx context.Context, rr *radixv1.RadixRegistration) (string, error) {
		if rr.Spec.ConfigurationItem == "" && required {
			return "", ErrConfigurationItemIsRequired
		}

		return "", nil
	}
}

func createRequireUniqueAppIdValidator(client client.Client) validatorFunc {
	return func(ctx context.Context, rr *radixv1.RadixRegistration) (string, error) {
		if rr.Spec.AppID.IsZero() {
			return "", nil // AppID is not required
		}

		existingRRs := radixv1.RadixRegistrationList{}
		err := client.List(ctx, &existingRRs)
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("failed to list existing RadixRegistrations")
			return "", ErrUnknownServerError // Something went wrong while listing existing RadixRegistrations, let the user try again
		}

		for _, existingRR := range existingRRs.Items {
			if existingRR.Spec.AppID == rr.Spec.AppID && existingRR.Name != rr.Name {
				return "", ErrAppIdMustBeUnique
			}
		}

		return "", nil
	}
}

func createConfigurationItemUrlValidator(client *http.Client, validateConfigurationItem *url.URL) validatorFunc {
	return func(ctx context.Context, rr *radixv1.RadixRegistration) (string, error) {
		if validateConfigurationItem == nil {
			return "", nil // No validation URL provided
		}

		if rr.Spec.ConfigurationItem == "" {
			return "", nil // No configuration item to validate
		}

		actual, err := url.Parse(validateConfigurationItem.String())
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Stringer("url", validateConfigurationItem).Msg("Failed to parse validate configuration item URL")
			return "", fmt.Errorf("failed to parse validate configuration item URL: %w", err)
		}

		actual.Path = strings.Replace(actual.Path, "{appId}", rr.Spec.ConfigurationItem, 1)
		log.Ctx(ctx).Debug().Str("url", actual.String()).Msg("Validating configuration item URL")

		resp, err := client.Get(actual.String())
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("Failed to validate configuration item URL")
			return "", fmt.Errorf("failed to validate configuration item URL: %w", err)
		}
		defer resp.Body.Close()

		ok := resp.StatusCode >= 200 && resp.StatusCode < 300
		if !ok {
			log.Ctx(ctx).Info().Int("statusCode", resp.StatusCode).Msg("Configuration item validation failed")
			return "", ErrConfigurationItemIsNotValid
		}

		return "", nil
	}
}
