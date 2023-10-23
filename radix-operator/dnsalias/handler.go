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
	kubeclient  kubernetes.Interface
	kubeutil    *kube.Kube
	radixclient radixclient.Interface
	hasSynced   common.HasSynced
}

// NewHandler creates a handler for managing RadixDNSAlias resources
func NewHandler(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	hasSynced common.HasSynced) Handler {

	handler := Handler{
		kubeclient:  kubeclient,
		kubeutil:    kubeutil,
		radixclient: radixclient,
		hasSynced:   hasSynced,
	}

	return handler
}

// Sync is called by kubernetes after the Controller Enqueues a work-item
// and collects components and determines whether state must be reconciled.
func (t *Handler) Sync(namespace, name string, eventRecorder record.EventRecorder) error {
	radixDNSAlias, err := t.radixclient.RadixV1().RadixDNSAliases().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		// The RadixDNSAlias resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("radix DNS alias %s in work queue no longer exists", name))
			return nil
		}
		return err
	}

	syncDNSAlias := radixDNSAlias.DeepCopy()
	logger.Debugf("Sync DNS alias %s", name)

	appName := syncDNSAlias.Spec.AppName
	radixApplication, err := t.radixclient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).
		Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	dnsAlias, err := dnsalias.NewDNSAlias(t.kubeclient, t.kubeutil, t.radixclient, syncDNSAlias, radixApplication, logger)
	if err != nil {
		return err
	}

	err = dnsAlias.OnSync(metav1.NewTime(time.Now().UTC()))
	if err != nil {
		return err
	}

	t.hasSynced(true)
	eventRecorder.Event(dnsAlias.GetConfig(), corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}
