package registration

import (
	"fmt"

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
func (t *RadixRegistrationHandler) ObjectCreated(obj interface{}) error {
	radixRegistration, ok := obj.(*v1.RadixRegistration)
	if !ok {
		return fmt.Errorf("Provided object was not a valid Radix Registration; instead was %v", obj)
	}

	kube, _ := kube.New(t.kubeclient)

	kube.CreateEnvironment(radixRegistration, "app")

	brigadeProject, err := t.brigade.EnsureProject(radixRegistration)
	if err != nil {
		log.Errorf("Failed to create Brigade project: %v", err)
		return fmt.Errorf("Failed to create Brigade project: %v", err)
	}

	// TODO
	err = kube.ApplyRbacRadixRegistration(radixRegistration, brigadeProject)
	if err != nil {
		log.Errorf("Failed to set access on RadixRegistration: %v", err)
	}

	return nil
}

// ObjectDeleted is called when an object is deleted
func (t *RadixRegistrationHandler) ObjectDeleted(key string) error {
	return nil
}

// ObjectUpdated is called when an object is updated
func (t *RadixRegistrationHandler) ObjectUpdated(objOld, objNew interface{}) error {
	radixRegistration, ok := objNew.(*v1.RadixRegistration)
	if !ok {
		return fmt.Errorf("Provided object was not a valid Radix Registration; instead was %v", objNew)
	}

	_, err := t.brigade.EnsureProject(radixRegistration)
	if err != nil {
		return fmt.Errorf("Failed to update Brigade project: %v", err)
	}
	return nil
}
