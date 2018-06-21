package application

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/kube"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type RadixAppHandler struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
	kubeutil    *kube.Kube
}

func NewApplicationHandler(kubeclient kubernetes.Interface, radixclient radixclient.Interface) RadixAppHandler {
	kube, _ := kube.New(kubeclient)

	handler := RadixAppHandler{
		kubeclient:  kubeclient,
		radixclient: radixclient,
		kubeutil:    kube,
	}

	return handler
}

// Init handles any handler initialization
func (t *RadixAppHandler) Init() error {
	log.Info("RadixAppHandler.Init")
	return nil
}

// ObjectCreated is called when an object is created
func (t *RadixAppHandler) ObjectCreated(obj interface{}) error {
	radixApp, registration, err := t.ensureCorrectlyRegisteredApp(obj)
	if err != nil {
		return fmt.Errorf("Failed to create application: %v", err)
	}

	for _, e := range radixApp.Spec.Environments {
		err := t.kubeutil.CreateEnvironment(registration, e.Name)
		if err != nil {
			return fmt.Errorf("Failed to create environment: %v", err)
		}

		err = t.kubeutil.CreateSecrets(registration, e.Name)
		if err != nil {
			return fmt.Errorf("Failed to provision secrets: %v", err)
		}
	}

	t.kubeutil.CreateRoleBindings(radixApp)
	return nil
}

// ObjectDeleted is called when an object is deleted
func (t *RadixAppHandler) ObjectDeleted(key string) error {
	return nil
}

// ObjectUpdated is called when an object is updated
func (t *RadixAppHandler) ObjectUpdated(objOld, objNew interface{}) error {
	radixApp, registration, err := t.ensureCorrectlyRegisteredApp(objNew)
	if err != nil {
		return fmt.Errorf("Failed to update application: %v", err)
	}
	radixAppSelector := fmt.Sprintf("radixApp=%s", radixApp.Name)

	existingEnvironments, err := t.kubeclient.CoreV1().Namespaces().List(metav1.ListOptions{
		LabelSelector: radixAppSelector,
	})
	if err != nil {
		return fmt.Errorf("Failed to retrieve existing environments: %v", err)
	}

	err = t.removeDeletedEnvironments(existingEnvironments, radixApp)
	if err != nil {
		return fmt.Errorf("Failed to clean up deleted environments: %v", err)
	}
	for _, e := range radixApp.Spec.Environments {
		err := t.kubeutil.CreateEnvironment(registration, e.Name)
		if err != nil {
			return fmt.Errorf("Failed to create environment: %v", err)
		}
	}

	return nil
}

func (t *RadixAppHandler) removeDeletedEnvironments(existingNamespaces *corev1.NamespaceList, radixApp *v1.RadixApplication) error {
	for _, ns := range existingNamespaces.Items {
		exists := false
		for _, env := range radixApp.Spec.Environments {
			if ns.Name == fmt.Sprintf("%s-app", radixApp.Name) {
				exists = true
				continue
			}

			if ns.Name == fmt.Sprintf("%s-%s", radixApp.Name, env.Name) {
				exists = true
			}
		}
		if !exists {
			err := t.kubeclient.CoreV1().Namespaces().Delete(ns.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *RadixAppHandler) ensureCorrectlyRegisteredApp(obj interface{}) (*v1.RadixApplication, *v1.RadixRegistration, error) {
	radixApp, ok := obj.(*v1.RadixApplication)
	if !ok {
		return nil, nil, fmt.Errorf("Provided object was not a valid Radix Application; instead was %v", obj)
	}

	registration, err := t.radixclient.RadixV1().RadixRegistrations("default").Get(radixApp.Name, metav1.GetOptions{})
	if err != nil {
		return radixApp, nil, fmt.Errorf("Could not find Radix Registration for %s", radixApp.Name)
	}

	return radixApp, registration, nil
}
