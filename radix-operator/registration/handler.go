package registration

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/application"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Registration is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Registration
	// is synced successfully
	MessageResourceSynced = "Radix Registration synced successfully"
)

type RadixRegistrationHandler struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
}

//NewRegistrationHandler creates a handler which deals with RadixRegistration resources
func NewRegistrationHandler(
	kubeclient kubernetes.Interface,
	radixclient radixclient.Interface) RadixRegistrationHandler {

	handler := RadixRegistrationHandler{
		kubeclient:  kubeclient,
		radixclient: radixclient,
	}

	return handler
}

// Sync Is created on sync of resource
func (t *RadixRegistrationHandler) Sync(namespace, name string, eventRecorder record.EventRecorder) error {
	registration, err := t.radixclient.RadixV1().RadixRegistrations(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		// The Registration resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("Radix registration '%s' in work queue no longer exists", name))
			return nil
		}

		return err
	}

	syncRegistration := registration.DeepCopy()
	logger.Infof("Sync registration %s", syncRegistration.Name)
	err = t.onSync(syncRegistration)
	if err != nil {
		// Put back on queue
		return err
	}

	eventRecorder.Event(syncRegistration, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// TODO: Move to application domain
func (t *RadixRegistrationHandler) onSync(radixRegistration *v1.RadixRegistration) error {
	application, _ := application.NewApplication(t.kubeclient, t.radixclient, radixRegistration)
	return application.OnRegistered()
}
