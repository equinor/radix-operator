package kube

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8errs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
)

const waitTimeout = 10 * time.Second

// ApplyNamespace Creates a new namespace, if not exists allready
func (kube *Kube) ApplyNamespace(name string, labels map[string]string, ownerRefs []metav1.OwnerReference) error {
	log.Debugf("Create namespace: %s", name)

	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			OwnerReferences: ownerRefs,
			Labels:          labels,
		},
	}
	_, err := kube.kubeClient.CoreV1().Namespaces().Create(&namespace)

	if k8errs.IsAlreadyExists(err) {
		log.Debugf("Namespace object %s already exists, updating the object now", name)
		oldNamespace, err := kube.kubeClient.CoreV1().Namespaces().Get(name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("Failed to get old Ingress object: %v", err)
		}

		newNamespace := oldNamespace.DeepCopy()
		newNamespace.ObjectMeta.OwnerReferences = ownerRefs
		newNamespace.ObjectMeta.Labels = labels

		oldNamespaceJSON, err := json.Marshal(oldNamespace)
		if err != nil {
			return fmt.Errorf("Failed to marshal old namespace object: %v", err)
		}

		newNamespaceJSON, err := json.Marshal(newNamespace)
		if err != nil {
			return fmt.Errorf("Failed to marshal new namespace object: %v", err)
		}

		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldNamespaceJSON, newNamespaceJSON, corev1.Namespace{})
		if err != nil {
			return fmt.Errorf("Failed to create two way merge patch namespace objects: %v", err)
		}

		if !isEmptyPatch(patchBytes) {
			patchedNamespace, err := kube.kubeClient.CoreV1().Namespaces().Patch(name, types.StrategicMergePatchType, patchBytes)
			if err != nil {
				return fmt.Errorf("Failed to patch namespace object: %v", err)
			}

			log.Debugf("Patched namespace: %s ", patchedNamespace.Name)
		} else {
			log.Debugf("No need to patch namespace: %s ", name)
		}

		return nil
	}

	return err
}

// NamespaceWatcher Watcher to wait for namespace to be created
type NamespaceWatcher interface {
	WaitFor(namespace string) error
}

// NamespaceWatcherImpl Implementation of watcher
type NamespaceWatcherImpl struct {
	client kubernetes.Interface
}

// NewNamespaceWatcherImpl Constructor
func NewNamespaceWatcherImpl(client kubernetes.Interface) NamespaceWatcherImpl {
	return NamespaceWatcherImpl{
		client,
	}
}

// WaitFor Waits for namespace to appear
func (watcher NamespaceWatcherImpl) WaitFor(namespace string) error {
	log.Infof("Waiting for namespace %s", namespace)
	err := waitForNamespace(watcher.client, namespace)
	if err != nil {
		return err
	}

	log.Infof("Namespace %s exists and is active", namespace)
	return nil

}

func waitForNamespace(client kubernetes.Interface, namespace string) error {
	checkDone := make(chan bool, 1)
	errorCh := make(chan error, 1)
	timer := time.NewTimer(waitTimeout)
	defer timer.Stop()

	for {
		go func() {
			ns, err := client.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
			if !k8errs.IsNotFound(err) {
				errorCh <- err
			}

			if ns != nil && ns.Status.Phase == corev1.NamespaceActive {
				errorCh <- nil
			}

			time.Sleep(time.Second)
			checkDone <- true
		}()

		select {
		case <-checkDone:
			log.Debugf("Namespace %s still doesn't exists", namespace)
		case err := <-errorCh:
			return err
		case <-timer.C:
			return errors.New("Timed out waiting for namespace")
		}
	}
}
