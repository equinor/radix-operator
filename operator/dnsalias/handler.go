package dnsalias

import (
	"context"

	"github.com/equinor/radix-operator/operator/common"
	"github.com/equinor/radix-operator/operator/dnsalias/internal"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Handler Handler for radix dns aliases
type handler struct {
	kubeClient           kubernetes.Interface
	kubeUtil             *kube.Kube
	radixClient          radixclient.Interface
	dynamicClient        client.Client
	syncerFactory        internal.SyncerFactory
	events               common.SyncEventRecorder
	dnsZone              string
	ingressConfiguration ingress.IngressConfiguration
	oauth2DefaultConfig  defaults.OAuth2Config
}

// NewHandler creates a handler for managing RadixDNSAlias resources
func NewHandler(
	kubeClient kubernetes.Interface,
	kubeUtil *kube.Kube,
	radixClient radixclient.Interface,
	dynamicClient client.Client,
	eventRecorder record.EventRecorder,
	dnsZone string,
	options ...HandlerConfigOption) common.Handler {

	h := &handler{
		kubeClient:    kubeClient,
		kubeUtil:      kubeUtil,
		radixClient:   radixClient,
		dynamicClient: dynamicClient,
		syncerFactory: internal.SyncerFactoryFunc(dnsalias.NewSyncer),
		events:        common.NewSyncEventRecorder(eventRecorder),
		dnsZone:       dnsZone,
	}

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
func (h *handler) Sync(ctx context.Context, _, name string) error {
	radixDNSAlias, err := h.radixClient.RadixV1().RadixDNSAliases().Get(ctx, name, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			log.Ctx(ctx).Info().Msgf("RadixDNSAlias %s in work queue no longer exists", name)
			return nil
		}
		return err
	}
	ctx = log.Ctx(ctx).With().Str("app_name", radixDNSAlias.Spec.AppName).Logger().WithContext(ctx)

	syncingAlias := radixDNSAlias.DeepCopy()
	log.Ctx(ctx).Debug().Msgf("Sync RadixDNSAlias %s", name)
	targetIngressNamespace := utils.GetEnvironmentNamespace(syncingAlias.Spec.AppName, syncingAlias.Spec.Environment)
	componentIngressAnnotations := ingress.GetComponentAnnotationProvider(h.ingressConfiguration, targetIngressNamespace, h.oauth2DefaultConfig)
	oauthIngressAnnotations := ingress.GetOAuthAnnotationProviders()
	oauthProxyModeIngressAnnotations := ingress.GetOAuthProxyModeAnnotationProviders(h.ingressConfiguration, targetIngressNamespace)
	syncer := h.syncerFactory.CreateSyncer(syncingAlias, h.kubeClient, h.kubeUtil, h.radixClient, h.dynamicClient, h.dnsZone, h.oauth2DefaultConfig, componentIngressAnnotations, oauthIngressAnnotations, oauthProxyModeIngressAnnotations)
	err = syncer.OnSync(ctx)
	if err != nil {
		h.events.RecordSyncErrorEvent(syncingAlias, err)
		return err
	}

	h.events.RecordSyncSuccessEvent(syncingAlias)
	return nil
}
