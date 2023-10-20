package dnsalias

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"

	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	radixDNSAlias, err := t.radixclient.RadixV1().RadixDNSAliases().Get(context.TODO(), name, meta.GetOptions{})
	if err != nil {
		// The DNSAlias resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("radix dns alias %s in work queue no longer exists", name))
			return nil
		}

		return err
	}

	syncDNSAlias := radixDNSAlias.DeepCopy()
	logger.Debugf("Sync dns alias %s", syncDNSAlias.Name)

	// TOD sync
	// radixRegistration, err := t.kubeutil.GetRegistration(syncDNSAlias.Spec.AppName)
	// if err != nil {
	// 	// The Registration resource may no longer exist, in which case we stop
	// 	// processing.
	// 	if errors.IsNotFound(err) {
	// 		utilruntime.HandleError(fmt.Errorf("failed to get RadixRegistartion object: %v", err))
	// 		return nil
	// 	}
	// 	return err
	// }
	//
	// // get RA error is ignored because nil is accepted
	// radixApplication, _ := t.radixclient.RadixV1().RadixApplications(utils.GetAppNamespace(syncDNSAlias.Spec.AppName)).
	// 	Get(context.TODO(), syncDNSAlias.Spec.AppName, meta.GetOptions{})
	//
	// nw, err := networkpolicy.NewNetworkPolicy(t.kubeclient, t.kubeutil, logger, syncDNSAlias.Spec.AppName)
	// if err != nil {
	// 	return err
	// }
	//
	// env, err := dns alias.NewDNSAlias(t.kubeclient, t.kubeutil, t.radixclient, syncDNSAlias, radixRegistration, radixApplication, logger, &nw)
	//
	// if err != nil {
	// 	return err
	// }
	//
	// err = env.OnSync(meta.NewTime(time.Now().UTC()))
	// if err != nil {
	// 	return err
	// }

	// t.hasSynced(true)
	// eventRecorder.Event(env.GetConfig(), core.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}
