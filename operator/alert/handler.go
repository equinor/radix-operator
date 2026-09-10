package alert

import (
	"context"

	"github.com/equinor/radix-operator/operator/common"
	"github.com/equinor/radix-operator/pkg/apis/alert"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	kubeclient         kubernetes.Interface
	dynamicClient      client.Client
	kubeutil           *kube.Kube
	alertSyncerFactory alert.AlertSyncerFactory
	events             common.SyncEventRecorder
}

// NewHandler Constructor
func NewHandler(kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	dynamicClient client.Client,
	eventRecorder record.EventRecorder,
	options ...HandlerConfigOption) common.Handler {

	handler := &handler{
		kubeclient:         kubeclient,
		dynamicClient:      dynamicClient,
		kubeutil:           kubeutil,
		alertSyncerFactory: alert.AlertSyncerFactoryFunc(alert.New),
		events:             common.NewSyncEventRecorder(eventRecorder),
	}

	for _, option := range options {
		option(handler)
	}

	return handler
}

// Sync Is created on sync of resource
func (t *handler) Sync(ctx context.Context, namespace, name string) error {
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

	alertSyncer := t.alertSyncerFactory.CreateAlertSyncer(t.dynamicClient, syncRAL)
	err = alertSyncer.OnSync(ctx)
	if err != nil {
		t.events.RecordSyncErrorEvent(syncRAL, err)
		return err
	}

	t.events.RecordSyncSuccessEvent(syncRAL)
	return nil
}
