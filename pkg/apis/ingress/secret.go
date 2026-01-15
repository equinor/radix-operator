package ingress

import (
	"github.com/equinor/radix-common/utils/pointers"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// IsSecretRequiredForClientCertificate Check is Secret is required for thr ClientCertificate
func IsSecretRequiredForClientCertificate(clientCertificate *radixv1.ClientCertificate) bool {
	if clientCertificate != nil {
		certificateConfig := ParseClientCertificateConfiguration(*clientCertificate)
		if *certificateConfig.PassCertificateToUpstream || *certificateConfig.Verification != radixv1.VerificationTypeOff {
			return true
		}
	}

	return false
}

// ParseClientCertificateConfiguration Parses ClientCertificate configuration
func ParseClientCertificateConfiguration(clientCertificate radixv1.ClientCertificate) (certificate radixv1.ClientCertificate) {
	verification := radixv1.VerificationTypeOff
	certificate = radixv1.ClientCertificate{
		Verification:              &verification,
		PassCertificateToUpstream: pointers.Ptr(false),
	}

	if passUpstream := clientCertificate.PassCertificateToUpstream; passUpstream != nil {
		certificate.PassCertificateToUpstream = passUpstream
	}

	if verification := clientCertificate.Verification; verification != nil {
		certificate.Verification = verification
	}

	return
}
