package internal

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	dnsAliasIngressNameTemplate = "%s.%s.custom-domain" // <component-name>.<dns-alias-domain>.custom-domain
)

// CreateRadixDNSAliasIngress Create an Ingress for a RadixDNSAlias
func CreateRadixDNSAliasIngress(kubeClient kubernetes.Interface, appName, envName string, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	return kubeClient.NetworkingV1().Ingresses(utils.GetEnvironmentNamespace(appName, envName)).Create(context.Background(), ingress, metav1.CreateOptions{})
}

// BuildRadixDNSAliasIngress Build an Ingress for a RadixDNSAlias
func BuildRadixDNSAliasIngress(appName, domain, service string, port int32) *networkingv1.Ingress {
	pathTypeImplementationSpecific := networkingv1.PathTypeImplementationSpecific
	host := fmt.Sprintf(dnsAliasIngressNameTemplate, service, domain)
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:   domain,
			Labels: labels.Merge(labels.ForApplicationName(appName), labels.ForComponentName(service)),
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{
				{
					Hosts:      []string{host},
					SecretName: defaults.TLSSecretName,
				},
			},
			Rules: []networkingv1.IngressRule{
				{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{Paths: []networkingv1.HTTPIngressPath{
							{Path: "/", PathType: &pathTypeImplementationSpecific, Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{Name: service, Port: networkingv1.ServiceBackendPort{
									Number: port,
								}},
							}}},
						}},
				},
			},
		}}
}
