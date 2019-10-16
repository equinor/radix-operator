package kube

import (
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// ApplyIngress Will create or update ingress in provided namespace
func (kube *Kube) ApplyIngress(namespace string, ingress *v1beta1.Ingress) error {
	ingressName := ingress.GetName()
	log.Debugf("Creating Ingress object %s in namespace %s", ingressName, namespace)

	_, err := kube.kubeClient.ExtensionsV1beta1().Ingresses(namespace).Create(ingress)
	if errors.IsAlreadyExists(err) {
		log.Debugf("Ingress object %s already exists in namespace %s, updating the object now", ingressName, namespace)
		oldIngress, err := kube.kubeClient.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("Failed to get old Ingress object: %v", err)
		}
		newIngress := oldIngress.DeepCopy()
		newIngress.ObjectMeta.Labels = ingress.ObjectMeta.Labels
		newIngress.ObjectMeta.Annotations = ingress.ObjectMeta.Annotations
		newIngress.ObjectMeta.OwnerReferences = ingress.ObjectMeta.OwnerReferences
		newIngress.Spec = ingress.Spec

		oldIngressJSON, err := json.Marshal(oldIngress)
		if err != nil {
			return fmt.Errorf("Failed to marshal old Ingress object: %v", err)
		}

		newIngressJSON, err := json.Marshal(newIngress)
		if err != nil {
			return fmt.Errorf("Failed to marshal new Ingress object: %v", err)
		}

		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldIngressJSON, newIngressJSON, v1beta1.Ingress{})
		if err != nil {
			return fmt.Errorf("Failed to create two way merge patch Ingess objects: %v", err)
		}

		patchedIngress, err := kube.kubeClient.ExtensionsV1beta1().Ingresses(namespace).Patch(ingressName, types.StrategicMergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch Ingress object: %v", err)
		}
		log.Debugf("Patched Ingress: %s in namespace %s", patchedIngress.Name, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("Failed to create Ingress object: %v", err)
	}
	log.Debugf("Created Ingress: %s in namespace %s", ingressName, namespace)
	return nil
}
