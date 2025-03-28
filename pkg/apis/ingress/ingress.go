package ingress

import (
	"github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressConfiguration Holds all ingress annotation configurations
type IngressConfiguration struct {
	AnnotationConfigurations []AnnotationConfiguration `json:"configuration" yaml:"configuration"`
}

// AnnotationConfiguration Holds annotations for a single configuration
type AnnotationConfiguration struct {
	Name        string
	Annotations map[string]string
}

// GetIngressSpec Get Ingress spec
func GetIngressSpec(hostname, serviceName, tlsSecretName string, servicePort int32) networkingv1.IngressSpec {
	pathType := networkingv1.PathTypeImplementationSpecific
	ingressClass := "nginx"

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
								Path:     "/",
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

// GetIngressConfig Gets Ingress configuration
func GetIngressConfig(namespace string, appName string, component radixv1.RadixCommonDeployComponent, ingressName string, ingressSpec networkingv1.IngressSpec, ingressProviders []AnnotationProvider, ownerReference []metav1.OwnerReference) (*networkingv1.Ingress, error) {

	annotations := map[string]string{}
	for _, ingressProvider := range ingressProviders {
		providedAnnotations, err := ingressProvider.GetAnnotations(component, namespace)
		if err != nil {
			return nil, err
		}
		annotations = maps.MergeMaps(annotations, providedAnnotations)
	}

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ingressName,
			Annotations: annotations,
			Labels: map[string]string{
				kube.RadixAppLabel:       appName,
				kube.RadixComponentLabel: component.GetName(),
			},
			OwnerReferences: ownerReference,
		},
		Spec: ingressSpec,
	}
	return ing, nil
}

// GetAnnotationProvider Gets annotation provider
func GetAnnotationProvider(ingressConfiguration IngressConfiguration, certificateNamespace string, oauth2DefaultConfig defaults.OAuth2Config) []AnnotationProvider {
	return []AnnotationProvider{
		NewForceSslRedirectAnnotationProvider(),
		NewIngressConfigurationAnnotationProvider(ingressConfiguration),
		NewClientCertificateAnnotationProvider(certificateNamespace),
		NewOAuth2AnnotationProvider(oauth2DefaultConfig),
		NewIngressPublicAllowListAnnotationProvider(),
		NewIngressPublicConfigAnnotationProvider(),
		NewRedirectErrorPageAnnotationProvider(),
	}
}
