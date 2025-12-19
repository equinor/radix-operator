package ingress

import (
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	oauthutil "github.com/equinor/radix-operator/pkg/apis/utils/oauth"
	networkingv1 "k8s.io/api/networking/v1"
)

const (
	ingressClass string = "nginx"
)

// BuildIngressSpecForComponent Builds ingress spec for a component
func BuildIngressSpecForComponent(component radixv1.RadixCommonDeployComponent, hostname, tlsSecretName string) networkingv1.IngressSpec {
	var servicePort int32
	if component.GetPublicPort() == "" {
		// For backwards compatibility
		servicePort = component.GetPorts()[0].Port
	} else {
		if port, ok := slice.FindFirst(component.GetPorts(), func(cp radixv1.ComponentPort) bool {
			return strings.EqualFold(cp.Name, component.GetPublicPort())
		}); ok {
			servicePort = port.Port
		}
	}

	return buildIngressSpec(hostname, component.GetName(), tlsSecretName, servicePort, "/")
}

// BuildIngressSpecForOAuth2Component Builds ingress spec for a component's oauth2 service
func BuildIngressSpecForOAuth2Component(component radixv1.RadixCommonDeployComponent, hostname, tlsSecretName string, proxyMode bool) networkingv1.IngressSpec {
	serviceName := utils.GetAuxOAuthProxyComponentServiceName(component.GetName())
	path := oauthutil.SanitizePathPrefix(component.GetAuthentication().GetOAuth2().ProxyPrefix)
	if proxyMode {
		path = "/"
	}

	return buildIngressSpec(hostname, serviceName, tlsSecretName, defaults.OAuthProxyPortNumber, path)
}

func buildIngressSpec(hostname, serviceName, tlsSecretName string, servicePort int32, path string) networkingv1.IngressSpec {
	pathType := networkingv1.PathTypeImplementationSpecific
	ingressClass := ingressClass

	if tlsSecretName == "" {
		tlsSecretName = defaults.TLSSecretName
	}

	return networkingv1.IngressSpec{
		IngressClassName: &ingressClass,
		TLS: []networkingv1.IngressTLS{
			{
				Hosts: []string{
					hostname,
				},
				SecretName: tlsSecretName,
			},
		},
		Rules: []networkingv1.IngressRule{
			{
				Host: hostname,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path:     path,
								PathType: &pathType,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: serviceName,
										Port: networkingv1.ServiceBackendPort{
											Number: servicePort,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
