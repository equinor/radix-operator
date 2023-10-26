package dnsalias

import (
	"context"
	"fmt"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/config"
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
type Handler struct {
	kubeClient  kubernetes.Interface
	kubeUtil    *kube.Kube
	radixClient radixclient.Interface
	hasSynced   common.HasSynced
	config      *config.ClusterConfig
}

// NewHandler creates a handler for managing RadixDNSAlias resources
func NewHandler(kubeclient kubernetes.Interface, kubeutil *kube.Kube, radixclient radixclient.Interface, config *config.ClusterConfig, hasSynced common.HasSynced) *Handler {

	return &Handler{
		kubeClient:  kubeclient,
		kubeUtil:    kubeutil,
		radixClient: radixclient,
		config:      config,
		hasSynced:   hasSynced,
	}
}

// Sync is called by kubernetes after the Controller Enqueues a work-item
func (h *Handler) Sync(_, name string, eventRecorder record.EventRecorder) error {
	radixDNSAlias, err := h.radixClient.RadixV1().RadixDNSAliases().Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("RadixDNSAlias %s in work queue no longer exists", name))
			return nil
		}
		return err
	}

	syncDNSAlias := radixDNSAlias.DeepCopy()
	logger.Debugf("Sync RadixDNSAlias %s", name)

	appName := syncDNSAlias.Spec.AppName
	radixApplication, err := h.radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).
		Get(context.Background(), appName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	syncer, err := dnsalias.NewDNSAliasSyncer(h.kubeClient, h.kubeUtil, h.radixClient, syncDNSAlias, radixApplication, logger)
	if err != nil {
		return err
	}

	err = syncer.OnSync(metav1.NewTime(time.Now().UTC()))
	if err != nil {
		return err
	}

	h.hasSynced(true)
	eventRecorder.Event(syncer.GetRadixDNSAlias(), corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}
