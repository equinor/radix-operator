package radixapplication

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/config"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

func createCertificateAutomationValidator(certificateAutomationConfig config.CertificateAutomationConfig) validatorFunc {
	return func(ctx context.Context, ra *radixv1.RadixApplication) ([]string, []error) {
		var errs []error
		var wrns []string

		if ra.Spec.DNSExternalAlias == nil {
			return nil, errs
		}

		if len(certificateAutomationConfig.Issuers) == 0 {
			errs = append(errs, fmt.Errorf("certificate automation: %w", ErrMissingConfiguredCertificateAutomationIssuers))
		}

		for _, extAlias := range ra.Spec.DNSExternalAlias {
			if !extAlias.UseCertificateAutomation {
				if extAlias.CertificateAutomation != nil {
					wrns = append(wrns, fmt.Sprintf("external alias %s: %s", extAlias.Alias, WarnUnusedCertificateAutomation))
				}

				continue
			}

			if extAlias.CertificateAutomation == nil && certificateAutomationConfig.DefaultIssuer == "" {
				errs = append(errs, fmt.Errorf("external alias %s: %w", extAlias.Alias, ErrMissingCertificateAutomationIssuer))
			}

			selectedIssuer := certificateAutomationConfig.DefaultIssuer
			if extAlias.CertificateAutomation != nil && extAlias.CertificateAutomation.Issuer != "" {
				selectedIssuer = extAlias.CertificateAutomation.Issuer
			}

			if _, ok := certificateAutomationConfig.Issuers[selectedIssuer]; !ok {
				errs = append(errs, fmt.Errorf("external alias %s: %w", extAlias.Alias, ErrInvalidCertificateAutomationIssuer))
			}
		}

		return wrns, errs
	}
}
