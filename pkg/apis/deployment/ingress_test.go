package deployment

import (
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
)

var testIngressConfiguration = `
configuration:
  - name: ewma
    annotations:
      nginx.ingress.kubernetes.io/load-balance: ewma
  - name: round-robin
    annotations:
      nginx.ingress.kubernetes.io/load-balance: round_robin
  - name: socket
    annotations:
      nginx.ingress.kubernetes.io/proxy-connect-timeout: 3600
      nginx.ingress.kubernetes.io/proxy-read-timeout: 3600
      nginx.ingress.kubernetes.io/proxy-send-timeout: 3600
`

func TestGetAnnotationsFromConfigurations_ReturnsCorrectConfig(t *testing.T) {
	config := getConfigFromStringData(testIngressConfiguration)
	annotations := getAnnotationsFromConfigurations(config, "socket")
	assert.Equal(t, 3, len(annotations))

	annotations = getAnnotationsFromConfigurations(config, "socket", "round-robin")
	assert.Equal(t, 4, len(annotations))

	annotations = getAnnotationsFromConfigurations(config, "non-existing")
	assert.Equal(t, 0, len(annotations))
}

func TestGetAuthenticationAnnotationsFromConfiguration(t *testing.T) {
	verification := v1.VerificationTypeOptional

	expect1 := make(map[string]string)
	expect1["nginx.ingress.kubernetes.io/auth-tls-pass-certificate-to-upstream"] = "true"
	expect1["nginx.ingress.kubernetes.io/auth-tls-verify-client"] = string(v1.VerificationTypeOff)
	expect1["nginx.ingress.kubernetes.io/auth-tls-secret"] = utils.GetComponentClientCertificateSecretName("ns/name")

	expect2 := make(map[string]string)
	expect2["nginx.ingress.kubernetes.io/auth-tls-pass-certificate-to-upstream"] = "false"
	expect2["nginx.ingress.kubernetes.io/auth-tls-verify-client"] = string(v1.VerificationTypeOff)

	expect3 := make(map[string]string)
	expect3["nginx.ingress.kubernetes.io/auth-tls-pass-certificate-to-upstream"] = "false"
	expect3["nginx.ingress.kubernetes.io/auth-tls-verify-client"] = string(verification)
	expect3["nginx.ingress.kubernetes.io/auth-tls-secret"] = utils.GetComponentClientCertificateSecretName("ns/name")

	config1 := &v1.Authentication{
		ClientCertificate: &v1.ClientCertificate{
			PassCertificateToUpstream: utils.BoolPtr(true),
		},
	}

	config2 := &v1.Authentication{
		ClientCertificate: &v1.ClientCertificate{
			PassCertificateToUpstream: utils.BoolPtr(false),
		},
	}

	config3 := &v1.Authentication{
		ClientCertificate: &v1.ClientCertificate{
			Verification: &verification,
		},
	}

	result := getAuthenticationAnnotationsFromConfiguration(config1, "name", "ns")
	assert.Equal(t, expect1, result)

	result = getAuthenticationAnnotationsFromConfiguration(config2, "name", "ns")
	assert.Equal(t, expect2, result)

	result = getAuthenticationAnnotationsFromConfiguration(config3, "name", "ns")
	assert.Equal(t, expect3, result)

	result = getAuthenticationAnnotationsFromConfiguration(nil, "name", "ns")
	assert.Empty(t, result, "Expected Annotations to be empty")
}
