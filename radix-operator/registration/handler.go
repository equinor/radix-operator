package registration

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/application"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	listers "github.com/equinor/radix-operator/pkg/client/listers/radix/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Registration is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Registration
	// is synced successfully
	MessageResourceSynced = "Radix Registration synced successfully"
)

type RadixRegistrationHandler struct {
	kubeclient         kubernetes.Interface
	radixclient        radixclient.Interface
	registrationLister listers.RadixRegistrationLister
}

//NewRegistrationHandler creates a handler which deals with RadixRegistration resources
func NewRegistrationHandler(
	kubeclient kubernetes.Interface,
	radixclient radixclient.Interface,
	registrationLister listers.RadixRegistrationLister) RadixRegistrationHandler {
	handler := RadixRegistrationHandler{
		kubeclient:         kubeclient,
		radixclient:        radixclient,
		registrationLister: registrationLister,
	}

	return handler
}

// Sync Is created on sync of resource
func (t *RadixRegistrationHandler) Sync(namespace, name string, eventRecorder record.EventRecorder) error {
	registration, err := t.registrationLister.RadixRegistrations(namespace).Get(name)
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
	klog.Infof("Sync registration %s", syncRegistration.Name)
	t.onSync(syncRegistration)
	eventRecorder.Event(syncRegistration, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// TODO: Move to application domain
func (t *RadixRegistrationHandler) onSync(radixRegistration *v1.RadixRegistration) {
	application, _ := application.NewApplication(t.kubeclient, t.radixclient, radixRegistration)
	application.OnRegistered()
}

// ObjectCreated is called when an object is created
// TODO: remove
func (t *RadixRegistrationHandler) ObjectCreated(obj interface{}) error {
	return nil
}

// ObjectDeleted is called when an object is deleted
// TODO: remove
func (t *RadixRegistrationHandler) ObjectDeleted(key string) error {
	return nil
}

// ObjectUpdated is called when an object is updated
// TODO: remove
func (t *RadixRegistrationHandler) ObjectUpdated(objOld, objNew interface{}) error {

	return nil
}
