package application

import (
	"fmt"

	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Application Config is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Application Config
	// is synced successfully
	MessageResourceSynced = "Radix Application synced successfully"
)

// RadixApplicationHandler Instance variables
type RadixApplicationHandler struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
	kubeutil    *kube.Kube
	hasSynced   common.HasSynced
}

// NewApplicationHandler Constructor
func NewApplicationHandler(
	kubeclient kubernetes.Interface,
	radixclient radixclient.Interface,
	hasSynced common.HasSynced) RadixApplicationHandler {
	kube, _ := kube.New(kubeclient)

	handler := RadixApplicationHandler{
		kubeclient:  kubeclient,
		radixclient: radixclient,
		kubeutil:    kube,
		hasSynced:   hasSynced,
	}

	return handler
}

// Sync Is created on sync of resource
func (t *RadixApplicationHandler) Sync(namespace, name string, eventRecorder record.EventRecorder) error {
	radixApplication, err := t.radixclient.RadixV1().RadixApplications(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		// The Application resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("Radix application '%s' in work queue no longer exists", name))
			return nil
		}

		return err
	}

	radixRegistration, err := t.radixclient.RadixV1().RadixRegistrations().Get(radixApplication.Name, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Failed to get RR for app %s. Error: %v", radixApplication.Name, err)
		return err
	}

	syncApplication := radixApplication.DeepCopy()
	logger.Infof("Sync application %s", syncApplication.Name)
	applicationConfig, err := application.NewApplicationConfig(t.kubeclient, t.radixclient, radixRegistration, radixApplication)
	if err != nil {
		// Put back on queue
		return err
	}
	err = applicationConfig.OnSync()
	if err != nil {
		// Put back on queue
		return err
	}

	t.hasSynced(true)
	eventRecorder.Event(syncApplication, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}
