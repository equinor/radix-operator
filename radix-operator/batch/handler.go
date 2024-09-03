package batch

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/batch"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/batch/internal"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

const (
	// Synced is the Event reason when a RadixBatch is synced without errors
	Synced = "Synced"

	// SyncFailed is the Event reason when an error occurs while syncing a RadixBatch
	SyncFailed = "SyncFailed"

	// MessageResourceSynced is the message used for an Event fired when a RadixBatch
	// is synced successfully
	MessageResourceSynced = "RadixBatch synced successfully"
)

var _ common.Handler = &handler{}

// HandlerConfigOption defines a configuration function used for additional configuration of handler
type HandlerConfigOption func(*handler)

// WithSyncerFactory configures the SyncerFactory for the handler
func WithSyncerFactory(factory internal.SyncerFactory) HandlerConfigOption {
	return func(h *handler) {
		h.syncerFactory = factory
	}
}

type handler struct {
	kubeclient    kubernetes.Interface
	radixclient   radixclient.Interface
	kubeutil      *kube.Kube
	syncerFactory internal.SyncerFactory
	config        *config.Config
}

func NewHandler(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	config *config.Config,
	options ...HandlerConfigOption) common.Handler {

	h := &handler{
		kubeclient:  kubeclient,
		kubeutil:    kubeutil,
		radixclient: radixclient,
		config:      config,
	}

	configureDefaultSyncerFactory(h)

	for _, option := range options {
		option(h)
	}

	return h
}

func (h *handler) Sync(ctx context.Context, namespace, name string, eventRecorder record.EventRecorder) error {
	radixBatch, err := h.radixclient.RadixV1().RadixBatches(namespace).Get(context.Background(), name, v1.GetOptions{})
	if err != nil {
		// The resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			log.Ctx(ctx).Info().Msgf("RadixBatch %s/%s in work queue no longer exists", namespace, name)
			return nil
		}

		return err
	}
	ctx = log.Ctx(ctx).With().Str("app_name", radixBatch.Labels[kube.RadixAppLabel]).Logger().WithContext(ctx)
	syncBatch := radixBatch.DeepCopy()
	syncer := h.syncerFactory.CreateSyncer(h.kubeclient, h.kubeutil, h.radixclient, syncBatch, h.config)
	err = syncer.OnSync(ctx)
	if err != nil {
		eventRecorder.Event(syncBatch, corev1.EventTypeWarning, SyncFailed, err.Error())
		// Put back on queue
		return err
	}

	eventRecorder.Event(syncBatch, corev1.EventTypeNormal, Synced, MessageResourceSynced)
	return nil
}

func configureDefaultSyncerFactory(h *handler) {
	WithSyncerFactory(internal.SyncerFactoryFunc(batch.NewSyncer))(h)
}
