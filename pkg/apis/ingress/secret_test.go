package ingress

import (
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
)

func Test_IsSecretRequiredForClientCertificate(t *testing.T) {
	type scenario struct {
		clientCertificate *v1.ClientCertificate
		expectedResult    bool
	}
	scenarios := []scenario{
		{clientCertificate: nil, expectedResult: false},
		{clientCertificate: &v1.ClientCertificate{Verification: getVerificationTypePtr(v1.VerificationTypeOff)}, expectedResult: false},
		{clientCertificate: &v1.ClientCertificate{Verification: getVerificationTypePtr(v1.VerificationTypeOptional)}, expectedResult: true},
		{clientCertificate: &v1.ClientCertificate{Verification: getVerificationTypePtr(v1.VerificationTypeOptionalNoCa)}, expectedResult: true},
		{clientCertificate: &v1.ClientCertificate{Verification: getVerificationTypePtr(v1.VerificationTypeOn)}, expectedResult: true},
		{clientCertificate: &v1.ClientCertificate{}, expectedResult: false},
		{clientCertificate: &v1.ClientCertificate{PassCertificateToUpstream: pointers.Ptr(false)}, expectedResult: false},
		{clientCertificate: &v1.ClientCertificate{PassCertificateToUpstream: pointers.Ptr(true)}, expectedResult: true},
		{clientCertificate: &v1.ClientCertificate{Verification: getVerificationTypePtr(v1.VerificationTypeOff), PassCertificateToUpstream: pointers.Ptr(true)}, expectedResult: true},
		{clientCertificate: &v1.ClientCertificate{Verification: getVerificationTypePtr(v1.VerificationTypeOff), PassCertificateToUpstream: pointers.Ptr(false)}, expectedResult: false},
		{clientCertificate: &v1.ClientCertificate{Verification: getVerificationTypePtr(v1.VerificationTypeOptional), PassCertificateToUpstream: pointers.Ptr(true)}, expectedResult: true},
		{clientCertificate: &v1.ClientCertificate{Verification: getVerificationTypePtr(v1.VerificationTypeOptional), PassCertificateToUpstream: pointers.Ptr(false)}, expectedResult: true},
	}

	for _, ts := range scenarios {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, ts.expectedResult, IsSecretRequiredForClientCertificate(ts.clientCertificate))
		})
	}
}

func getVerificationTypePtr(verificationType v1.VerificationType) *v1.VerificationType {
	return &verificationType
}
