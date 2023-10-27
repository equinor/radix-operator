package dnsalias

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/config"
	"github.com/equinor/radix-operator/radix-operator/dnsalias/internal"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a DNSAlias is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a DNSAlias
	// is synced successfully
	MessageResourceSynced = "Radix DNSAlias synced successfully"
)

// Handler Handler for radix dns aliases
type handler struct {
	kubeClient    kubernetes.Interface
	kubeUtil      *kube.Kube
	radixClient   radixclient.Interface
	syncerFactory internal.SyncerFactory
	hasSynced     common.HasSynced
	clusterConfig *config.ClusterConfig
}

// NewHandler creates a handler for managing RadixDNSAlias resources
func NewHandler(
	kubeClient kubernetes.Interface,
	kubeUtil *kube.Kube,
	radixClient radixclient.Interface,
	clusterConfig *config.ClusterConfig,
	hasSynced common.HasSynced,
	options ...HandlerConfigOption) common.Handler {
	h := &handler{
		kubeClient:    kubeClient,
		kubeUtil:      kubeUtil,
		radixClient:   radixClient,
		hasSynced:     hasSynced,
		clusterConfig: clusterConfig,
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

// Sync is called by kubernetes after the Controller Enqueues a work-item
func (h *handler) Sync(_, name string, eventRecorder record.EventRecorder) error {
	radixDNSAlias, err := h.radixClient.RadixV1().RadixDNSAliases().Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("RadixDNSAlias %s in work queue no longer exists", name))
			return nil
		}
		return err
	}

	syncingAlias := radixDNSAlias.DeepCopy()
	logger.Debugf("Sync RadixDNSAlias %s", name)

	syncer := h.syncerFactory.CreateSyncer(h.kubeClient, h.kubeUtil, h.radixClient, h.clusterConfig, syncingAlias)
	err = syncer.OnSync()
	if err != nil {
		return err
	}

	h.hasSynced(true)
	eventRecorder.Event(syncingAlias, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func configureDefaultSyncerFactory(h *handler) {
	WithSyncerFactory(internal.SyncerFactoryFunc(dnsalias.NewSyncer))(h)
}
