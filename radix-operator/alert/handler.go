package alert

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/alert"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/rs/zerolog/log"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Alert is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Alert
	// is synced successfully
	MessageResourceSynced = "Radix Alert synced successfully"
)

// HandlerConfigOption defines a configuration function used for additional configuration of Handler
type HandlerConfigOption func(*Handler)

// WithAlertSyncerFactory configures the alertSyncerFactory for the Handler
func WithAlertSyncerFactory(factory alert.AlertSyncerFactory) HandlerConfigOption {
	return func(h *Handler) {
		h.alertSyncerFactory = factory
	}
}

// Handler Instance variables
type Handler struct {
	kubeclient              kubernetes.Interface
	radixclient             radixclient.Interface
	prometheusperatorclient monitoring.Interface
	kubeutil                *kube.Kube
	alertSyncerFactory      alert.AlertSyncerFactory
}

// NewHandler Constructor
func NewHandler(kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	prometheusperatorclient monitoring.Interface,
	options ...HandlerConfigOption) *Handler {

	handler := &Handler{
		kubeclient:              kubeclient,
		radixclient:             radixclient,
		prometheusperatorclient: prometheusperatorclient,
		kubeutil:                kubeutil,
	}

	configureDefaultAlertSyncerFactory(handler)

	for _, option := range options {
		option(handler)
	}

	return handler
}

// Sync Is created on sync of resource
func (t *Handler) Sync(ctx context.Context, namespace, name string, eventRecorder record.EventRecorder) error {
	alert, err := t.kubeutil.GetRadixAlert(ctx, namespace, name)
	if err != nil {
		// The Alert resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			log.Ctx(ctx).Info().Msgf("RadixAlert %s/%s in work queue no longer exists", namespace, name)
			return nil
		}

		return err
	}

	ctx = log.Ctx(ctx).With().Str("app_name", alert.Labels[kube.RadixAppLabel]).Logger().WithContext(ctx)

	syncRAL := alert.DeepCopy()
	log.Ctx(ctx).Debug().Msgf("Sync radix alert %s", syncRAL.Name)

	alertSyncer := t.alertSyncerFactory.CreateAlertSyncer(t.kubeclient, t.kubeutil, t.radixclient, t.prometheusperatorclient, syncRAL)
	err = alertSyncer.OnSync(ctx)
	if err != nil {
		// TODO: should we record a Warning event when there is an error, similar to batch handler? Possibly do it in common.Controller?
		// Put back on queue
		return err
	}

	eventRecorder.Event(syncRAL, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func configureDefaultAlertSyncerFactory(h *Handler) {
	WithAlertSyncerFactory(alert.AlertSyncerFactoryFunc(alert.New))(h)
}
