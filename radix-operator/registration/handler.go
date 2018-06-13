package registration

import (
	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/kube"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"k8s.io/client-go/kubernetes"
)

type RadixRegistrationHandler struct {
	kubeclient kubernetes.Interface
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
}

// ObjectDeleted is called when an object is deleted
func (t *RadixRegistrationHandler) ObjectDeleted(key string) {

}

// ObjectUpdated is called when an object is updated
func (t *RadixRegistrationHandler) ObjectUpdated(objOld, objNew interface{}) {

}
