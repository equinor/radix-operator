package ingress

import "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// IsSecretRequiredForClientCertificate Check is Secret is required for thr ClientCertificate
func IsSecretRequiredForClientCertificate(clientCertificate *v1.ClientCertificate) bool {
	if clientCertificate != nil {
		certificateConfig := ParseClientCertificateConfiguration(*clientCertificate)
		if *certificateConfig.PassCertificateToUpstream || *certificateConfig.Verification != v1.VerificationTypeOff {
			return true
		}
	}

	return false
}
