package registration

import (
	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/brigade"
	"github.com/statoil/radix-operator/pkg/apis/kube"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"k8s.io/client-go/kubernetes"
)

type RadixRegistrationHandler struct {
	kubeclient kubernetes.Interface
	brigade    *brigade.BrigadeGateway
}

//NewRegistrationHandler creates a handler which deals with RadixRegistration resources
func NewRegistrationHandler(kubeclient kubernetes.Interface) RadixRegistrationHandler {
	brigadeGw, _ := brigade.New(kubeclient)
	handler := RadixRegistrationHandler{
		kubeclient: kubeclient,
		brigade:    brigadeGw,
	}

	return handler
}

// Init handles any handler initialization
func (t *RadixRegistrationHandler) Init() error {
	log.Info("RadixRegistrationHandler.Init")
	return nil
}

// ObjectCreated is called when an object is created
func (t *RadixRegistrationHandler) ObjectCreated(obj interface{}) {
	radixRegistration, ok := obj.(*v1.RadixRegistration)
	if !ok {
		log.Errorf("Provided object was not a valid Radix Registration; instead was %v", obj)
		return
	}

	kube, _ := kube.New(t.kubeclient)

	kube.CreateEnvironment(radixRegistration, "app")

	err := t.brigade.EnsureProject(radixRegistration)
	if err != nil {
		log.Errorf("Failed to create Brigade project: %v", err)
	}
}

// ObjectDeleted is called when an object is deleted
func (t *RadixRegistrationHandler) ObjectDeleted(key string) {

}

// ObjectUpdated is called when an object is updated
func (t *RadixRegistrationHandler) ObjectUpdated(objOld, objNew interface{}) {
	radixRegistration, ok := objNew.(*v1.RadixRegistration)
	if !ok {
		log.Errorf("Provided object was not a valid Radix Registration; instead was %v", objNew)
		return
	}

	err := t.brigade.EnsureProject(radixRegistration)
	if err != nil {
		log.Errorf("Failed to update Brigade project: %v", err)
	}
}
