package ingress_test

import (
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	oauthutil "github.com/equinor/radix-operator/pkg/apis/utils/oauth"
	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
)

func Test_BuildIngressSpecForComponent(t *testing.T) {
	const (
		hostName      = "any.domain.com"
		tlsSecret     = "any-tls-secret"
		componentName = "any-component"
	)
	component := &radixv1.RadixDeployComponent{
		Name: componentName,
		Ports: []radixv1.ComponentPort{
			{Name: "port1", Port: 1000},
			{Name: "port2", Port: 2000},
			{Name: "port3", Port: 3000},
		},
		PublicPort: "port2",
	}

	t.Run("with tls secret name", func(t *testing.T) {
		actualSpec := ingress.BuildIngressSpecForComponent(component, hostName, tlsSecret)
		expectedSpec := networkingv1.IngressSpec{
			IngressClassName: pointers.Ptr("nginx"),
			TLS: []networkingv1.IngressTLS{{
				Hosts:      []string{hostName},
				SecretName: tlsSecret,
			}},
			Rules: []networkingv1.IngressRule{{
				Host: hostName,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/",
							PathType: pointers.Ptr(networkingv1.PathTypeImplementationSpecific),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: componentName,
									Port: networkingv1.ServiceBackendPort{
										Number: 2000,
									},
								},
							},
						}},
					},
				},
			}},
		}
		assert.Equal(t, expectedSpec, actualSpec)
	})

	t.Run("without tls secret name", func(t *testing.T) {
		actualSpec := ingress.BuildIngressSpecForComponent(component, hostName, "")
		expectedSpec := networkingv1.IngressSpec{
			IngressClassName: pointers.Ptr("nginx"),
			TLS: []networkingv1.IngressTLS{{
				Hosts:      []string{hostName},
				SecretName: defaults.TLSSecretName,
			}},
			Rules: []networkingv1.IngressRule{{
				Host: hostName,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/",
							PathType: pointers.Ptr(networkingv1.PathTypeImplementationSpecific),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: componentName,
									Port: networkingv1.ServiceBackendPort{
										Number: 2000,
									},
								},
							},
						}},
					},
				},
			}},
		}
		assert.Equal(t, expectedSpec, actualSpec)
	})
}

func Test_BuildIngressSpecForOAuth2Component(t *testing.T) {
	const (
		hostName        = "any.domain.com"
		tlsSecret       = "any-tls-secret"
		componentName   = "any-component"
		proxyPrefixPath = "/any/path/../path2"
	)
	component := &radixv1.RadixDeployComponent{
		Name: componentName,
		Ports: []radixv1.ComponentPort{
			{Name: "port", Port: 1000},
		},
		PublicPort: "port",
		Authentication: &radixv1.Authentication{
			OAuth2: &radixv1.OAuth2{
				ProxyPrefix: proxyPrefixPath,
			},
		},
	}

	t.Run("proxy mode false", func(t *testing.T) {
		actualSpec := ingress.BuildIngressSpecForOAuth2Component(component, hostName, tlsSecret, false)
		expectedSpec := networkingv1.IngressSpec{
			IngressClassName: pointers.Ptr("nginx"),
			TLS: []networkingv1.IngressTLS{{
				Hosts:      []string{hostName},
				SecretName: tlsSecret,
			}},
			Rules: []networkingv1.IngressRule{{
				Host: hostName,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     oauthutil.SanitizePathPrefix(proxyPrefixPath),
							PathType: pointers.Ptr(networkingv1.PathTypeImplementationSpecific),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: utils.GetAuxOAuthProxyComponentServiceName(componentName),
									Port: networkingv1.ServiceBackendPort{
										Number: defaults.OAuthProxyPortNumber,
									},
								},
							},
						}},
					},
				},
			}},
		}
		assert.Equal(t, expectedSpec, actualSpec)
	})

	t.Run("proxy mode true", func(t *testing.T) {
		actualSpec := ingress.BuildIngressSpecForOAuth2Component(component, hostName, tlsSecret, true)
		expectedSpec := networkingv1.IngressSpec{
			IngressClassName: pointers.Ptr("nginx"),
			TLS: []networkingv1.IngressTLS{{
				Hosts:      []string{hostName},
				SecretName: tlsSecret,
			}},
			Rules: []networkingv1.IngressRule{{
				Host: hostName,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/",
							PathType: pointers.Ptr(networkingv1.PathTypeImplementationSpecific),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: utils.GetAuxOAuthProxyComponentServiceName(componentName),
									Port: networkingv1.ServiceBackendPort{
										Number: defaults.OAuthProxyPortNumber,
									},
								},
							},
						}},
					},
				},
			}},
		}
		assert.Equal(t, expectedSpec, actualSpec)
	})
}
