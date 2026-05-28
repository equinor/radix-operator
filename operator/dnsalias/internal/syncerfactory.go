package internal

import (
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	dnsaliasapi "github.com/equinor/radix-operator/pkg/apis/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncerFactory defines a factory to create a DNS alias Syncer
type SyncerFactory interface {
	CreateSyncer(radixDNSAlias *radixv1.RadixDNSAlias, kubeUtil *kube.Kube, radixClient radixclient.Interface, dynamicClient client.Client, config config.Config, oauth2Config defaults.OAuth2Config) dnsaliasapi.Syncer
}

// SyncerFactoryFunc is an adapter that can be used to convert
// a function into a SyncerFactory
type SyncerFactoryFunc func(
	radixDNSAlias *radixv1.RadixDNSAlias,
	kubeUtil *kube.Kube,
	radixClient radixclient.Interface,
	dynamicClient client.Client,
	config config.Config,
	oauth2Config defaults.OAuth2Config,
) dnsaliasapi.Syncer

// CreateSyncer Create a DNS alias Syncer
func (f SyncerFactoryFunc) CreateSyncer(radixDNSAlias *radixv1.RadixDNSAlias, kubeUtil *kube.Kube, radixClient radixclient.Interface, dynamicClient client.Client, config config.Config, oauth2Config defaults.OAuth2Config) dnsaliasapi.Syncer {
	return f(radixDNSAlias, kubeUtil, radixClient, dynamicClient, config, oauth2Config)
}
