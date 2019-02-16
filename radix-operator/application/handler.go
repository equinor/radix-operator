package application

import (
	"fmt"

	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
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
	return nil
}

// ObjectCreated is called when an object is created
func (t *RadixApplicationHandler) ObjectCreated(obj interface{}) error {
	logger.Info("Application object created event received.")
	radixApplication, ok := obj.(*v1.RadixApplication)
	if !ok {
		return fmt.Errorf("Provided object was not a valid Radix Application; instead was %v", obj)
	}

	err := t.processRadixApplication(radixApplication)
	if err != nil {
		return err
	}

	return nil
}

// ObjectDeleted is called when an object is deleted
func (t *RadixApplicationHandler) ObjectDeleted(key string) error {
	logger.Info("Application object deleted event received. Do nothing.")
	return nil
}

// ObjectUpdated is called when an object is updated
func (t *RadixApplicationHandler) ObjectUpdated(objOld, objNew interface{}) error {
	logger.Info("Application object updated event received.")
	radixApplication, ok := objNew.(*v1.RadixApplication)
	if !ok {
		return fmt.Errorf("Provided object was not a valid Radix Application; instead was %v", objNew)
	}

	err := t.processRadixApplication(radixApplication)
	if err != nil {
		return err
	}

	return nil
}

func (t *RadixApplicationHandler) processRadixApplication(radixApplication *v1.RadixApplication) error {
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
