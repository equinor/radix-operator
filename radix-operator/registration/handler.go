package registration

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/prometheus/common/log"
	"k8s.io/client-go/kubernetes"
)

type RadixRegistrationHandler struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
}

//NewRegistrationHandler creates a handler which deals with RadixRegistration resources
func NewRegistrationHandler(kubeclient kubernetes.Interface, radixclient radixclient.Interface) RadixRegistrationHandler {
	handler := RadixRegistrationHandler{
		kubeclient:  kubeclient,
		radixclient: radixclient,
	}

	return handler
}

// Init handles any handler initialization
func (t *RadixRegistrationHandler) Init() error {
	logger.Info("RadixRegistrationHandler.Init")
	return nil
}

// ObjectCreated is called when an object is created
func (t *RadixRegistrationHandler) ObjectCreated(obj interface{}) error {
	logger.Info("Registration object created event received.")
	radixRegistration, ok := obj.(*v1.RadixRegistration)
	if !ok {
		return fmt.Errorf("Provided object was not a valid Radix Registration; instead was %v", obj)
	}

	t.processRadixRegistration(radixRegistration)
	return nil
}

func (t *RadixRegistrationHandler) processRadixRegistration(radixRegistration *v1.RadixRegistration) {
	application, _ := application.NewApplication(t.kubeclient, t.radixclient, radixRegistration)
	application.OnRegistered()
}

// ObjectDeleted is called when an object is deleted
func (t *RadixRegistrationHandler) ObjectDeleted(key string) error {
	logger.Info("Registration object deleted event received. Do nothing.")
	return nil
}

// ObjectUpdated is called when an object is updated
func (t *RadixRegistrationHandler) ObjectUpdated(objOld, objNew interface{}) error {
	logger.Info("Registration object updated event received.")
	if objOld == nil {
		log.Info("update radix registration - no new changes (objOld == nil)")
		return nil
	}

	radixRegistrationOld, ok := objOld.(*v1.RadixRegistration)
	if !ok {
		return fmt.Errorf("Provided old object was not a valid Radix Registration; instead was %v", objOld)
	}

	radixRegistration, ok := objNew.(*v1.RadixRegistration)
	if !ok {
		return fmt.Errorf("Provided new object was not a valid Radix Registration; instead was %v", objNew)
	}
	application, err := application.NewApplication(t.kubeclient, t.radixclient, radixRegistration)

	if err != nil {
		return err
	}
	application.OnUpdated(radixRegistrationOld)

	return nil
}
