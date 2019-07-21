package kube

import (
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

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

	if errors.IsAlreadyExists(err) {
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

		patchedNamespace, err := kube.kubeClient.CoreV1().Namespaces().Patch(name, types.StrategicMergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch namespace object: %v", err)
		}

		log.Debugf("Patched namespace: %s ", patchedNamespace.Name)
		return nil
	}

	return err
}
