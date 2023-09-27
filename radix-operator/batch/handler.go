package batch

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/batch"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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

var _ common.Handler = &Handler{}

// HandlerConfigOption defines a configuration function used for additional configuration of Handler
type HandlerConfigOption func(*Handler)

// WithSyncerFactory configures the SyncerFactory for the Handler
func WithSyncerFactory(factory batch.SyncerFactory) HandlerConfigOption {
	return func(h *Handler) {
		h.syncerFactory = factory
	}
}

type Handler struct {
	kubeclient    kubernetes.Interface
	radixclient   radixclient.Interface
	kubeutil      *kube.Kube
	syncerFactory batch.SyncerFactory
}

func NewHandler(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	options ...HandlerConfigOption) *Handler {

	handler := &Handler{
		kubeclient:  kubeclient,
		kubeutil:    kubeutil,
		radixclient: radixclient,
	}

	configureDefaultSyncerFactory(handler)

	for _, option := range options {
		option(handler)
	}

	return handler
}

func (h *Handler) Sync(namespace, name string, eventRecorder record.EventRecorder) error {
	radixBatch, err := h.kubeutil.GetRadixBatch(namespace, name)
	if err != nil {
		// The resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("radix batch %s in work queue no longer exists", name))
			return nil
		}

		return err
	}

	syncRSC := radixBatch.DeepCopy()
	syncer := h.syncerFactory.CreateSyncer(h.kubeclient, h.kubeutil, h.radixclient, syncRSC)
	err = syncer.OnSync()
	if err != nil {
		eventRecorder.Event(syncRSC, corev1.EventTypeWarning, SyncFailed, err.Error())
		// Put back on queue
		return err
	}

	eventRecorder.Event(syncRSC, corev1.EventTypeNormal, Synced, MessageResourceSynced)
	return nil
}

func configureDefaultSyncerFactory(h *Handler) {
	WithSyncerFactory(batch.SyncerFactoryFunc(batch.NewSyncer))(h)
}
