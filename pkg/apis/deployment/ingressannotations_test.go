package deployment

import (
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
)

func Test_ForceSslRedirectAnnotations(t *testing.T) {
	sslAnnotations := forceSslRedirectAnnotations{}
	actual := sslAnnotations.GetAnnotations(&v1.RadixDeployComponent{})
	expected := map[string]string{"nginx.ingress.kubernetes.io/force-ssl-redirect": "true"}
	assert.Equal(t, expected, actual)
}

func Test_IngressConfigurationAnnotations(t *testing.T) {
	config := IngressConfiguration{
		AnnotationConfigurations: []AnnotationConfiguration{
			{Name: "ewma", Annotations: map[string]string{"ewma1": "x", "ewma2": "y"}},
			{Name: "socket", Annotations: map[string]string{"socket1": "x", "socket2": "y", "socket3": "z"}},
			{Name: "round-robin", Annotations: map[string]string{"round-robin1": "1"}},
		},
	}
	componentIngress := ingressConfigurationAnnotations{config: config}

	annotations := componentIngress.GetAnnotations(&v1.RadixDeployComponent{IngressConfiguration: []string{"socket"}})
	assert.Equal(t, 3, len(annotations))

	annotations = componentIngress.GetAnnotations(&v1.RadixDeployComponent{IngressConfiguration: []string{"socket", "round-robin"}})
	assert.Equal(t, 4, len(annotations))

	annotations = componentIngress.GetAnnotations(&v1.RadixDeployComponent{IngressConfiguration: []string{"non-existing"}})
	assert.Equal(t, 0, len(annotations))
}

func Test_ClientCertificateAnnotations(t *testing.T) {
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

	ingressAnnotations := clientCertificateAnnotations{namespace: "ns"}
	result := ingressAnnotations.GetAnnotations(&v1.RadixDeployComponent{Name: "name", Authentication: config1})
	assert.Equal(t, expect1, result)

	result = ingressAnnotations.GetAnnotations(&v1.RadixDeployComponent{Name: "name", Authentication: config2})
	assert.Equal(t, expect2, result)

	result = ingressAnnotations.GetAnnotations(&v1.RadixDeployComponent{Name: "name", Authentication: config3})
	assert.Equal(t, expect3, result)

	result = ingressAnnotations.GetAnnotations(&v1.RadixDeployComponent{Name: "name"})
	assert.Empty(t, result, "Expected Annotations to be empty")
}

func Test_OAuth2Annotations(t *testing.T) {
	type scenarioDef struct {
		name      string
		component v1.RadixDeployComponent
		expected  map[string]string
	}
	scenarios := []scenarioDef{
		{name: "with no oauth", component: v1.RadixDeployComponent{PublicPort: "http"}, expected: make(map[string]string)},
		{
			name:      "with default prefix",
			component: v1.RadixDeployComponent{PublicPort: "http", Authentication: &v1.Authentication{OAuth2: &v1.OAuth2{}}},
			expected: map[string]string{
				"nginx.ingress.kubernetes.io/auth-signin": "https://$host/oauth2/start?rd=$escaped_request_uri",
				"nginx.ingress.kubernetes.io/auth-url":    "https://$host/oauth2/auth",
			},
		},
		{
			name:      "with SetXAuthRequestHeaders",
			component: v1.RadixDeployComponent{PublicPort: "http", Authentication: &v1.Authentication{OAuth2: &v1.OAuth2{SetXAuthRequestHeaders: utils.BoolPtr(true)}}},
			expected: map[string]string{
				"nginx.ingress.kubernetes.io/auth-signin":           "https://$host/oauth2/start?rd=$escaped_request_uri",
				"nginx.ingress.kubernetes.io/auth-url":              "https://$host/oauth2/auth",
				"nginx.ingress.kubernetes.io/auth-response-headers": "X-Auth-Request-Access-Token,X-Auth-Request-User,X-Auth-Request-Groups,X-Auth-Request-Email,X-Auth-Request-Preferred-Username",
			},
		},
		{
			name:      "with SetXAuthRequestHeaders and SetAuthorizationHeader",
			component: v1.RadixDeployComponent{PublicPort: "http", Authentication: &v1.Authentication{OAuth2: &v1.OAuth2{SetXAuthRequestHeaders: utils.BoolPtr(true), SetAuthorizationHeader: utils.BoolPtr(true)}}},
			expected: map[string]string{
				"nginx.ingress.kubernetes.io/auth-signin":           "https://$host/oauth2/start?rd=$escaped_request_uri",
				"nginx.ingress.kubernetes.io/auth-url":              "https://$host/oauth2/auth",
				"nginx.ingress.kubernetes.io/auth-response-headers": "X-Auth-Request-Access-Token,X-Auth-Request-User,X-Auth-Request-Groups,X-Auth-Request-Email,X-Auth-Request-Preferred-Username,Authorization",
			},
		},
		{
			name:      "with only SetAuthorizationHeader",
			component: v1.RadixDeployComponent{PublicPort: "http", Authentication: &v1.Authentication{OAuth2: &v1.OAuth2{SetAuthorizationHeader: utils.BoolPtr(true)}}},
			expected: map[string]string{
				"nginx.ingress.kubernetes.io/auth-signin":           "https://$host/oauth2/start?rd=$escaped_request_uri",
				"nginx.ingress.kubernetes.io/auth-url":              "https://$host/oauth2/auth",
				"nginx.ingress.kubernetes.io/auth-response-headers": "Authorization",
			},
		},
		{
			name:      "non public component",
			component: v1.RadixDeployComponent{Authentication: &v1.Authentication{OAuth2: &v1.OAuth2{}}},
			expected:  make(map[string]string),
		},
	}
	oauthAnnotations := oauth2Annotations{}

	for _, scenario := range scenarios {
		actual := oauthAnnotations.GetAnnotations(&scenario.component)
		assert.Equal(t, scenario.expected, actual, scenario.name)
	}

}
