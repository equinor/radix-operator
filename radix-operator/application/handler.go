package application

import (
	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/kube"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"k8s.io/client-go/kubernetes"
)

type RadixAppHandler struct {
	kubeclient kubernetes.Interface
}

func NewApplicationHandler(kubeclient kubernetes.Interface) RadixAppHandler {
	handler := RadixAppHandler{
		kubeclient: kubeclient,
	}

	return handler
}

// Init handles any handler initialization
func (t *RadixAppHandler) Init() error {
	log.Info("RadixAppHandler.Init")
	return nil
}

// ObjectCreated is called when an object is created
func (t *RadixAppHandler) ObjectCreated(obj interface{}) {
	radixApp, ok := obj.(*v1.RadixApplication)
	if !ok {
		log.Errorf("Provided object was not a valid Radix Application; instead was %v", obj)
		return
	}

	kube, _ := kube.New(t.kubeclient)

	// for _, e := range radixApp.Spec.Environment {
	// 	err := kube.CreateEnvironment(radixApp, e.Name)
	// 	if err != nil {
	// 		log.Errorf("Failed to create environment: %v", err)
	// 	}
	// }

	kube.CreateRoleBindings(radixApp)
}

// ObjectDeleted is called when an object is deleted
func (t *RadixAppHandler) ObjectDeleted(key string) {

}

// ObjectUpdated is called when an object is updated
func (t *RadixAppHandler) ObjectUpdated(objOld, objNew interface{}) {

}
