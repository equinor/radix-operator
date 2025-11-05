package alert

import (
	"context"

	"github.com/equinor/radix-operator/operator/common"
	"github.com/equinor/radix-operator/pkg/apis/alert"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/rs/zerolog/log"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

// HandlerConfigOption defines a configuration function used for additional configuration of Handler
type HandlerConfigOption func(*handler)

// WithAlertSyncerFactory configures the alertSyncerFactory for the Handler
func WithAlertSyncerFactory(factory alert.AlertSyncerFactory) HandlerConfigOption {
	return func(h *handler) {
		h.alertSyncerFactory = factory
	}
}

// handler Instance variables
type handler struct {
	kubeclient              kubernetes.Interface
	radixclient             radixclient.Interface
	prometheusperatorclient monitoring.Interface
	kubeutil                *kube.Kube
	alertSyncerFactory      alert.AlertSyncerFactory
	events                  common.SyncedEventRecorder
}

// NewHandler Constructor
func NewHandler(kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	prometheusperatorclient monitoring.Interface,
	eventRecorder record.EventRecorder,
	options ...HandlerConfigOption) common.Handler {

	handler := &handler{
		kubeclient:              kubeclient,
		radixclient:             radixclient,
		prometheusperatorclient: prometheusperatorclient,
		kubeutil:                kubeutil,
		alertSyncerFactory:      alert.AlertSyncerFactoryFunc(alert.New),
		events:                  common.SyncedEventRecorder{EventRecorder: eventRecorder},
	}

	for _, option := range options {
		option(handler)
	}

	return handler
}

// Sync Is created on sync of resource
func (t *handler) Sync(ctx context.Context, namespace, name string, _ record.EventRecorder) error {
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
		t.events.RecordFailedEvent(syncRAL, err)
		return err
	}

	t.events.RecordSuccessEvent(syncRAL)
	return nil
}
