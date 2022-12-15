package scheduledjob

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/scheduledjob"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a RadixScheduleJob is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a RadixScheduleJob
	// is synced successfully
	MessageResourceSynced = "Radix Schedule Job synced successfully"
)

var _ common.Handler = &Handler{}

// HandlerConfigOption defines a configuration function used for additional configuration of Handler
type HandlerConfigOption func(*Handler)

// WithSyncerFactory configures the SyncerFactory for the Handler
func WithSyncerFactory(factory scheduledjob.SyncerFactory) HandlerConfigOption {
	return func(h *Handler) {
		h.syncerFactory = factory
	}
}

type Handler struct {
	kubeclient    kubernetes.Interface
	radixclient   radixclient.Interface
	kubeutil      *kube.Kube
	syncerFactory scheduledjob.SyncerFactory
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
	radixScheduledJob, err := h.radixclient.RadixV1().RadixScheduledJobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		// The resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("radix scheduled job %s in work queue no longer exists", name))
			return nil
		}

		return err
	}

	syncRSC := radixScheduledJob.DeepCopy()
	syncer := h.syncerFactory.CreateSyncer(h.kubeclient, h.kubeutil, h.radixclient, syncRSC)
	err = syncer.OnSync()
	if err != nil {
		// Put back on queue
		return err
	}

	eventRecorder.Event(syncRSC, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func configureDefaultSyncerFactory(h *Handler) {
	WithSyncerFactory(scheduledjob.SyncerFactoryFunc(scheduledjob.NewSyncer))(h)
}
