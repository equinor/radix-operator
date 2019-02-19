package application

import (
	"fmt"

	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
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
}

// NewApplicationHandler Constructor
func NewApplicationHandler(kubeclient kubernetes.Interface, radixclient radixclient.Interface) RadixApplicationHandler {
	kube, _ := kube.New(kubeclient)

	handler := RadixApplicationHandler{
		kubeclient:  kubeclient,
		radixclient: radixclient,
		kubeutil:    kube,
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

	syncApplication := radixApplication.DeepCopy()
	klog.Infof("Sync application %s", syncApplication.Name)
	t.onSync(syncApplication)
	eventRecorder.Event(syncApplication, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// TODO: Move to application config domain
func (t *RadixApplicationHandler) onSync(radixApplication *v1.RadixApplication) error {
	radixRegistration, err := t.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Get(radixApplication.Name, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Failed to get RR for app %s. Error: %v", radixApplication.Name, err)
		return err
	}

	applicationConfig, err := application.NewApplicationConfig(t.kubeclient, t.radixclient, radixRegistration, radixApplication)
	if err != nil {
		return err
	}

	applicationConfig.OnConfigApplied()
	return nil
}
