package application

import (
	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/brigade"
	"github.com/statoil/radix-operator/pkg/apis/kube"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type RadixAppHandler struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
}

func NewApplicationHandler(kubeclient kubernetes.Interface, radixclient radixclient.Interface) RadixAppHandler {
	handler := RadixAppHandler{
		kubeclient:  kubeclient,
		radixclient: radixclient,
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

	registration, err := t.radixclient.RadixV1().RadixRegistrations("default").Get(radixApp.Name, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Could not find Radix Registration for %s", radixApp.Name)
		return
	}

	kube, _ := kube.New(t.kubeclient)

	for _, e := range radixApp.Spec.Environments {
		err := kube.CreateEnvironment(registration, e.Name)
		if err != nil {
			log.Errorf("Failed to create environment: %v", err)
		}
	}

	kube.CreateRoleBindings(radixApp)

	brigade, err := brigade.New(t.kubeclient)
	if err != nil {
		log.Errorf("Failed to create Brigade gateway: %v", err)
		return
	}

	brigade.AddAppConfigToProject(radixApp)
}

// ObjectDeleted is called when an object is deleted
func (t *RadixAppHandler) ObjectDeleted(key string) {

}

// ObjectUpdated is called when an object is updated
func (t *RadixAppHandler) ObjectUpdated(objOld, objNew interface{}) {

}
