package internal

import (
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	dnsaliasapi "github.com/equinor/radix-operator/pkg/apis/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

// SyncerFactory defines a factory to create a DNS alias Syncer
type SyncerFactory interface {
	CreateSyncer(kubeClient kubernetes.Interface, kubeUtil *kube.Kube, radixClient radixclient.Interface, dnsConfig *dnsalias.DNSConfig, ingressConfiguration ingress.IngressConfiguration, oauth2Config defaults.OAuth2Config, ingressAnnotationProviders []ingress.AnnotationProvider, radixDNSAlias *radixv1.RadixDNSAlias) dnsaliasapi.Syncer
}

// SyncerFactoryFunc is an adapter that can be used to convert
// a function into a SyncerFactory
type SyncerFactoryFunc func(
	kubeClient kubernetes.Interface,
	kubeUtil *kube.Kube,
	radixClient radixclient.Interface,
	dnsConfig *dnsalias.DNSConfig,
	ingressConfiguration ingress.IngressConfiguration,
	oauth2Config defaults.OAuth2Config,
	ingressAnnotationProviders []ingress.AnnotationProvider,
	radixDNSAlias *radixv1.RadixDNSAlias,
) dnsaliasapi.Syncer

// CreateSyncer Create a DNS alias Syncer
func (f SyncerFactoryFunc) CreateSyncer(kubeClient kubernetes.Interface, kubeUtil *kube.Kube, radixClient radixclient.Interface, dnsConfig *dnsalias.DNSConfig, ingressConfiguration ingress.IngressConfiguration, oauth2Config defaults.OAuth2Config, ingressAnnotationProviders []ingress.AnnotationProvider, radixDNSAlias *radixv1.RadixDNSAlias) dnsaliasapi.Syncer {
	return f(kubeClient, kubeUtil, radixClient, dnsConfig, ingressConfiguration, oauth2Config, ingressAnnotationProviders, radixDNSAlias)
}
