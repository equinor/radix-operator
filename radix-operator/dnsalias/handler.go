package dnsalias

import (
	"context"

	dnsalias2 "github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/dnsalias/internal"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a DNSAlias is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a DNSAlias
	// is synced successfully
	MessageResourceSynced = "RadixDNSAlias synced successfully"
)

// Handler Handler for radix dns aliases
type handler struct {
	kubeClient           kubernetes.Interface
	kubeUtil             *kube.Kube
	radixClient          radixclient.Interface
	syncerFactory        internal.SyncerFactory
	hasSynced            common.HasSynced
	dnsConfig            *dnsalias2.DNSConfig
	ingressConfiguration ingress.IngressConfiguration
	oauth2DefaultConfig  defaults.OAuth2Config
}

// NewHandler creates a handler for managing RadixDNSAlias resources
func NewHandler(
	kubeClient kubernetes.Interface,
	kubeUtil *kube.Kube,
	radixClient radixclient.Interface,
	dnsConfig *dnsalias2.DNSConfig,
	hasSynced common.HasSynced,
	options ...HandlerConfigOption) common.Handler {
	h := &handler{
		kubeClient:  kubeClient,
		kubeUtil:    kubeUtil,
		radixClient: radixClient,
		hasSynced:   hasSynced,
		dnsConfig:   dnsConfig,
	}
	configureDefaultSyncerFactory(h)
	for _, option := range options {
		option(h)
	}
	return h
}

// HandlerConfigOption defines a configuration function used for additional configuration of handler
type HandlerConfigOption func(*handler)

// WithSyncerFactory configures the SyncerFactory for the handler
func WithSyncerFactory(factory internal.SyncerFactory) HandlerConfigOption {
	return func(h *handler) {
		h.syncerFactory = factory
	}
}

// WithIngressConfiguration sets the list of custom ingress confiigurations
func WithIngressConfiguration(config ingress.IngressConfiguration) HandlerConfigOption {
	return func(h *handler) {
		h.ingressConfiguration = config
	}
}

// WithOAuth2DefaultConfig configures default OAuth2 settings
func WithOAuth2DefaultConfig(oauth2Config defaults.OAuth2Config) HandlerConfigOption {
	return func(h *handler) {
		h.oauth2DefaultConfig = oauth2Config
	}
}

// Sync is called by kubernetes after the Controller Enqueues a work-item
func (h *handler) Sync(ctx context.Context, _, name string, eventRecorder record.EventRecorder) error {
	radixDNSAlias, err := h.kubeUtil.GetRadixDNSAlias(ctx, name)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Ctx(ctx).Info().Msgf("RadixDNSAlias %s in work queue no longer exists", name)
			return nil
		}
		return err
	}

	syncingAlias := radixDNSAlias.DeepCopy()
	log.Ctx(ctx).Debug().Msgf("Sync RadixDNSAlias %s", name)
	syncer := h.syncerFactory.CreateSyncer(h.kubeClient, h.kubeUtil, h.radixClient, h.dnsConfig, h.ingressConfiguration, h.oauth2DefaultConfig, ingress.GetAuxOAuthProxyAnnotationProviders(), syncingAlias)
	err = syncer.OnSync(ctx)
	if err != nil {
		// TODO: should we record a Warning event when there is an error, similar to batch handler? Possibly do it in common.Controller?
		return err
	}

	h.hasSynced(true)
	eventRecorder.Event(syncingAlias, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func configureDefaultSyncerFactory(h *handler) {
	WithSyncerFactory(internal.SyncerFactoryFunc(dnsalias.NewSyncer))(h)
}
