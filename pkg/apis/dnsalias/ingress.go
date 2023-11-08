package dnsalias

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias/internal"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateRadixDNSAliasIngress Create an Ingress for a RadixDNSAlias
func CreateRadixDNSAliasIngress(kubeClient kubernetes.Interface, appName, envName string, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	return kubeClient.NetworkingV1().Ingresses(utils.GetEnvironmentNamespace(appName, envName)).Create(context.Background(), ingress, metav1.CreateOptions{})
}

// BuildRadixDNSAliasIngress Build an Ingress for a RadixDNSAlias
func BuildRadixDNSAliasIngress(appName, domain, service string, port int32, owner *v1.RadixDNSAlias, config *config.DNSConfig) *networkingv1.Ingress {
	pathTypeImplementationSpecific := networkingv1.PathTypeImplementationSpecific
	ingressName := GetDNSAliasIngressName(service, domain)
	host := GetDNSAliasHost(domain, config.DNSZone)
	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ingressName,
			Labels:      labels.Merge(labels.ForApplicationName(appName), labels.ForComponentName(service)),
			Annotations: annotations.ForManagedByRadixDNSAliasIngress(domain),
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
	if owner != nil {
		ingress.SetOwnerReferences([]metav1.OwnerReference{internal.GetOwnerReference(owner)})
	}
	return &ingress
}

// GetDNSAliasIngressName Gets name of the ingress for the custom DNS alias
func GetDNSAliasIngressName(service string, domain string) string {
	return fmt.Sprintf("%s.%s.custom-domain", service, domain)
}

// GetDNSAliasHost Gets DNS alias domain host.
// Example for the domain "my-app" and the cluster "Playground": my-app.playground.radix.equinor.com
func GetDNSAliasHost(domain string, dnsZone string) string {
	return fmt.Sprintf("%s.%s", domain, dnsZone)
}
