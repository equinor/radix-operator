package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	k8errs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const waitTimeout = 15 * time.Second

// ApplyNamespace Creates a new namespace, if not exists already
func (kubeutil *Kube) ApplyNamespace(name string, labels map[string]string, ownerRefs []metav1.OwnerReference) error {
	log.Debug().Msgf("Create namespace: %s", name)

	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			OwnerReferences: ownerRefs,
			Labels:          labels,
		},
	}

	oldNamespace, err := kubeutil.getNamespace(name)
	if err != nil && k8errs.IsNotFound(err) {
		log.Debug().Msgf("namespace object %s doesn't exists, create the object", name)
		_, err := kubeutil.kubeClient.CoreV1().Namespaces().Create(context.TODO(), &namespace, metav1.CreateOptions{})
		return err
	}

	newNamespace := oldNamespace.DeepCopy()
	newNamespace.ObjectMeta.OwnerReferences = ownerRefs
	newNamespace.ObjectMeta.Labels = labels

	oldNamespaceJSON, err := json.Marshal(oldNamespace)
	if err != nil {
		return fmt.Errorf("failed to marshal old namespace object: %v", err)
	}

	newNamespaceJSON, err := json.Marshal(newNamespace)
	if err != nil {
		return fmt.Errorf("failed to marshal new namespace object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldNamespaceJSON, newNamespaceJSON, corev1.Namespace{})
	if err != nil {
		return fmt.Errorf("failed to create two way merge patch namespace objects: %v", err)
	}

	if !IsEmptyPatch(patchBytes) {
		patchedNamespace, err := kubeutil.kubeClient.CoreV1().Namespaces().Patch(context.TODO(), name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("failed to patch namespace object: %v", err)
		}

		log.Debug().Msgf("Patched namespace: %s ", patchedNamespace.Name)
		return nil
	}

	log.Debug().Msgf("No need to patch namespace: %s ", name)
	return nil
}

func (kubeutil *Kube) getNamespace(name string) (*corev1.Namespace, error) {
	var namespace *corev1.Namespace
	var err error

	if kubeutil.NamespaceLister != nil {
		namespace, err = kubeutil.NamespaceLister.Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		namespace, err = kubeutil.kubeClient.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return namespace, nil
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
	log.Info().Msgf("Waiting for namespace %s", namespace)
	err := waitForNamespace(watcher.client, namespace)
	if err != nil {
		return err
	}

	log.Info().Msgf("namespace %s exists and is active", namespace)
	return nil

}

func waitForNamespace(client kubernetes.Interface, namespace string) error {
	timoutContext, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()

	return wait.PollUntilContextCancel(timoutContext, time.Second, true, func(ctx context.Context) (done bool, err error) {
		ns, err := client.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
		if err != nil {
			if k8errs.IsNotFound(err) || k8errs.IsForbidden(err) {
				return false, nil // the environment namespace or the rolebinding for the cluster-role radix-pipeline-env are not yet created
			}
			return false, err
		}
		if ns != nil && ns.Status.Phase == corev1.NamespaceActive {
			return true, nil
		}
		return false, nil
	})
}
