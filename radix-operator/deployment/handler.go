package deployment

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"

	certclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Deployment is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Deployment
	// is synced successfully
	MessageResourceSynced = "Radix Deployment synced successfully"
)

var hasSyncedNoop common.HasSynced = func(b bool) {}

// HandlerConfigOption defines a configuration function used for additional configuration of Handler
type HandlerConfigOption func(*Handler)

// WithHasSyncedCallback configures Handler callback when RD has synced successfully
func WithHasSyncedCallback(callback common.HasSynced) HandlerConfigOption {
	return func(h *Handler) {
		h.hasSynced = callback
	}
}

// WithOAuth2DefaultConfig configures default OAuth2 settings
func WithOAuth2DefaultConfig(oauth2Config defaults.OAuth2Config) HandlerConfigOption {
	return func(h *Handler) {
		h.oauth2DefaultConfig = oauth2Config
	}
}

// WithOAuth2ProxyDockerImage configures the Docker image to use for OAuth2 proxy auxiliary component
func WithOAuth2ProxyDockerImage(image string) HandlerConfigOption {
	return func(h *Handler) {
		h.oauth2ProxyDockerImage = image
	}
}

// WithIngressConfiguration sets the list of custom ingress confiigurations
func WithIngressConfiguration(config ingress.IngressConfiguration) HandlerConfigOption {
	return func(h *Handler) {
		h.ingressConfiguration = config
	}
}

// WithDeploymentSyncerFactory configures the deploymentSyncerFactory for the Handler
func WithDeploymentSyncerFactory(factory deployment.DeploymentSyncerFactory) HandlerConfigOption {
	return func(h *Handler) {
		h.deploymentSyncerFactory = factory
	}
}

// Handler Instance variables
type Handler struct {
	kubeclient              kubernetes.Interface
	radixclient             radixclient.Interface
	prometheusperatorclient monitoring.Interface
	certClient              certclient.Interface
	kubeutil                *kube.Kube
	hasSynced               common.HasSynced
	oauth2DefaultConfig     defaults.OAuth2Config
	oauth2ProxyDockerImage  string
	ingressConfiguration    ingress.IngressConfiguration
	deploymentSyncerFactory deployment.DeploymentSyncerFactory
	config                  *config.Config
}

// NewHandler Constructor
func NewHandler(kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	prometheusperatorclient monitoring.Interface,
	certClient certclient.Interface,
	config *config.Config,
	options ...HandlerConfigOption) *Handler {

	handler := &Handler{
		kubeclient:              kubeclient,
		radixclient:             radixclient,
		prometheusperatorclient: prometheusperatorclient,
		certClient:              certClient,
		kubeutil:                kubeutil,
		config:                  config,
	}

	configureDefaultDeploymentSyncerFactory(handler)
	configureDefaultHasSynced(handler)

	for _, option := range options {
		option(handler)
	}

	return handler
}

// Sync Is created on sync of resource
func (t *Handler) Sync(namespace, name string, eventRecorder record.EventRecorder) error {
	rd, err := t.kubeutil.GetRadixDeployment(namespace, name)
	if err != nil {
		// The Deployment resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("radix deployment %s in work queue no longer exists", name))
			return nil
		}

		return err
	}
	if deployment.IsRadixDeploymentInactive(rd) {
		logger.Debug().Msgf("Ignoring RadixDeployment %s/%s as it's inactive.", rd.GetNamespace(), rd.GetName())
		return nil
	}

	syncRD := rd.DeepCopy()
	logger.Debug().Msgf("Sync deployment %s", syncRD.Name)

	radixRegistration, err := t.radixclient.RadixV1().RadixRegistrations().Get(context.TODO(), syncRD.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		// The Registration resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("failed to get RadixRegistartion object: %v", err))
			return nil
		}

		return err
	}

	ingressAnnotations := ingress.GetAnnotationProvider(t.ingressConfiguration, syncRD.Namespace, t.oauth2DefaultConfig)

	auxResourceManagers := []deployment.AuxiliaryResourceManager{
		deployment.NewOAuthProxyResourceManager(syncRD, radixRegistration, t.kubeutil, t.oauth2DefaultConfig, ingress.GetAuxOAuthProxyAnnotationProviders(), t.oauth2ProxyDockerImage),
	}

	deployment := t.deploymentSyncerFactory.CreateDeploymentSyncer(t.kubeclient, t.kubeutil, t.radixclient, t.prometheusperatorclient, t.certClient, radixRegistration, syncRD, ingressAnnotations, auxResourceManagers, t.config)
	err = deployment.OnSync()
	if err != nil {
		// Put back on queue
		return err
	}

	if t.hasSynced != nil {
		t.hasSynced(true)
	}

	eventRecorder.Event(syncRD, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func configureDefaultDeploymentSyncerFactory(h *Handler) {
	WithDeploymentSyncerFactory(deployment.DeploymentSyncerFactoryFunc(deployment.NewDeploymentSyncer))(h)
}

func configureDefaultHasSynced(h *Handler) {
	WithHasSyncedCallback(hasSyncedNoop)(h)
}
