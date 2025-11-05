package deployment

import (
	"context"

	"github.com/equinor/radix-operator/operator/common"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/rs/zerolog/log"

	certclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

var hasSyncedNoop common.HasSynced = func(b bool) {}

// HandlerConfigOption defines a configuration function used for additional configuration of Handler
type HandlerConfigOption func(*handler)

// WithHasSyncedCallback configures Handler callback when RD has synced successfully
func WithHasSyncedCallback(callback common.HasSynced) HandlerConfigOption {
	return func(h *handler) {
		h.hasSynced = callback
	}
}

// WithOAuth2DefaultConfig configures default OAuth2 settings
func WithOAuth2DefaultConfig(oauth2Config defaults.OAuth2Config) HandlerConfigOption {
	return func(h *handler) {
		h.oauth2DefaultConfig = oauth2Config
	}
}

// WithOAuth2ProxyDockerImage configures the Docker image to use for OAuth2 proxy auxiliary component
func WithOAuth2ProxyDockerImage(image string) HandlerConfigOption {
	return func(h *handler) {
		h.oauth2ProxyDockerImage = image
	}
}

// WithOAuth2RedisDockerImage configures the Docker image to use for OAuth2 redis auxiliary component
func WithOAuth2RedisDockerImage(image string) HandlerConfigOption {
	return func(h *handler) {
		h.oauth2RedisDockerImage = image
	}
}

// WithIngressConfiguration sets the list of custom ingress confiigurations
func WithIngressConfiguration(config ingress.IngressConfiguration) HandlerConfigOption {
	return func(h *handler) {
		h.ingressConfiguration = config
	}
}

// WithDeploymentSyncerFactory configures the deploymentSyncerFactory for the Handler
func WithDeploymentSyncerFactory(factory deployment.DeploymentSyncerFactory) HandlerConfigOption {
	return func(h *handler) {
		h.deploymentSyncerFactory = factory
	}
}

// handler Instance variables
type handler struct {
	kubeclient              kubernetes.Interface
	radixclient             radixclient.Interface
	prometheusperatorclient monitoring.Interface
	certClient              certclient.Interface
	kedaClient              kedav2.Interface
	events                  common.SyncEventRecorder
	kubeutil                *kube.Kube
	hasSynced               common.HasSynced
	oauth2DefaultConfig     defaults.OAuth2Config
	oauth2ProxyDockerImage  string
	oauth2RedisDockerImage  string
	ingressConfiguration    ingress.IngressConfiguration
	deploymentSyncerFactory deployment.DeploymentSyncerFactory
	config                  *config.Config
}

// NewHandler Constructor
func NewHandler(kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	kedaClient kedav2.Interface,
	prometheusperatorclient monitoring.Interface,
	certClient certclient.Interface,
	eventRecorder record.EventRecorder,
	config *config.Config,
	options ...HandlerConfigOption) common.Handler {

	handler := &handler{
		kubeclient:              kubeclient,
		radixclient:             radixclient,
		kedaClient:              kedaClient,
		prometheusperatorclient: prometheusperatorclient,
		certClient:              certClient,
		kubeutil:                kubeutil,
		hasSynced:               hasSyncedNoop,
		deploymentSyncerFactory: deployment.DeploymentSyncerFactoryFunc(deployment.NewDeploymentSyncer),
		events:                  common.SyncEventRecorder{EventRecorder: eventRecorder},
		config:                  config,
	}

	for _, option := range options {
		option(handler)
	}

	return handler
}

// Sync Is created on sync of resource
func (t *handler) Sync(ctx context.Context, namespace, name string) error {
	rd, err := t.kubeutil.GetRadixDeployment(ctx, namespace, name)
	if err != nil {
		// The Deployment resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			log.Ctx(ctx).Info().Msgf("RadixDeployment %s/%s in work queue no longer exists", namespace, name)
			return nil
		}

		return err
	}
	logger := log.Ctx(ctx).With().Str("app_name", rd.Spec.AppName).Logger()
	ctx = logger.WithContext(ctx)

	if deployment.IsRadixDeploymentInactive(rd) {
		logger.Debug().Msgf("Ignoring RadixDeployment %s/%s as it's inactive.", rd.GetNamespace(), rd.GetName())
		return nil
	}

	syncRD := rd.DeepCopy()
	logger.Debug().Msgf("Sync deployment %s", syncRD.Name)

	radixRegistration, err := t.radixclient.RadixV1().RadixRegistrations().Get(ctx, syncRD.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		// The Registration resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			logger.Debug().Msgf("RadixRegistration %s no longer exists", syncRD.Spec.AppName)
			return nil
		}

		return err
	}

	ingressAnnotations := ingress.GetAnnotationProvider(t.ingressConfiguration, syncRD.Namespace, t.oauth2DefaultConfig)

	auxResourceManagers := []deployment.AuxiliaryResourceManager{
		deployment.NewOAuthProxyResourceManager(syncRD, radixRegistration, t.kubeutil, t.oauth2DefaultConfig, ingress.GetAuxOAuthProxyAnnotationProviders(), t.oauth2ProxyDockerImage, t.config.ContainerRegistryConfig.ExternalRegistryAuthSecret),
		deployment.NewOAuthRedisResourceManager(syncRD, radixRegistration, t.kubeutil, t.oauth2RedisDockerImage, t.config.ContainerRegistryConfig.ExternalRegistryAuthSecret),
	}

	deployment := t.deploymentSyncerFactory.CreateDeploymentSyncer(t.kubeclient, t.kubeutil, t.radixclient, t.prometheusperatorclient, t.certClient, radixRegistration, syncRD, ingressAnnotations, auxResourceManagers, t.config)
	err = deployment.OnSync(ctx)
	if err != nil {
		t.events.RecordSyncErrorEvent(syncRD, err)
		return err
	}

	if t.hasSynced != nil {
		t.hasSynced(true)
	}

	t.events.RecordSyncSuccessEvent(syncRD)
	return nil
}
