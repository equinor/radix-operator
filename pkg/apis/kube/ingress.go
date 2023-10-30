package kube

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// GetIngress Gets an ingress by its name
func (kubeutil *Kube) GetIngress(namespace, name string) (*networkingv1.Ingress, error) {
	return kubeutil.kubeClient.NetworkingV1().Ingresses(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

// ApplyIngress Will create or update ingress in provided namespace
func (kubeutil *Kube) ApplyIngress(namespace string, ingress *networkingv1.Ingress) error {
	ingressName := ingress.GetName()
	log.Debugf("Creating Ingress object %s in namespace %s", ingressName, namespace)

	oldIngress, err := kubeutil.kubeClient.NetworkingV1().Ingresses(namespace).Get(context.Background(), ingressName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		_, err := kubeutil.kubeClient.NetworkingV1().Ingresses(namespace).Create(context.Background(), ingress, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create Ingress object: %v", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get Ingress object: %v", err)
	}

	log.Debugf("Ingress object %s already exists in namespace %s, updating the object now", ingressName, namespace)
	newIngress := oldIngress.DeepCopy()
	newIngress.ObjectMeta.Labels = ingress.ObjectMeta.Labels
	newIngress.ObjectMeta.Annotations = ingress.ObjectMeta.Annotations
	newIngress.ObjectMeta.OwnerReferences = ingress.ObjectMeta.OwnerReferences
	newIngress.Spec = ingress.Spec
	return kubeutil.PatchIngress(namespace, oldIngress, newIngress)
}

// PatchIngress Patches an ingress, if there are changes
func (kubeutil *Kube) PatchIngress(namespace string, oldIngress *networkingv1.Ingress, newIngress *networkingv1.Ingress) error {
	ingressName := oldIngress.GetName()
	oldIngressJSON, err := json.Marshal(oldIngress)
	if err != nil {
		return fmt.Errorf("failed to marshal old Ingress object: %v", err)
	}

	newIngressJSON, err := json.Marshal(newIngress)
	if err != nil {
		return fmt.Errorf("failed to marshal new Ingress object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldIngressJSON, newIngressJSON, networkingv1.Ingress{})
	if err != nil {
		return fmt.Errorf("failed to create two way merge patch Ingess objects: %v", err)
	}

	if !IsEmptyPatch(patchBytes) {
		patchedIngress, err := kubeutil.kubeClient.NetworkingV1().Ingresses(namespace).Patch(context.Background(), ingressName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("failed to patch Ingress object: %v", err)
		}
		log.Debugf("Patched Ingress: %s in namespace %s", patchedIngress.Name, namespace)
	} else {
		log.Debugf("No need to patch ingress: %s ", ingressName)
	}

	return nil
}

// ListIngresses lists ingresses
func (kubeutil *Kube) ListIngresses(namespace string) ([]*networkingv1.Ingress, error) {
	return kubeutil.ListIngressesWithSelector(namespace, "")
}

// ListIngressesWithSelector lists ingresses
func (kubeutil *Kube) ListIngressesWithSelector(namespace string, labelSelectorString string) ([]*networkingv1.Ingress, error) {
	var ingresses []*networkingv1.Ingress

	if kubeutil.IngressLister != nil {
		selector, err := labels.Parse(labelSelectorString)
		if err != nil {
			return nil, err
		}

		ingresses, err = kubeutil.IngressLister.Ingresses(namespace).List(selector)
		if err != nil {
			return nil, err
		}
	} else {
		listOptions := metav1.ListOptions{
			LabelSelector: labelSelectorString,
		}

		list, err := kubeutil.kubeClient.NetworkingV1().Ingresses(namespace).List(context.Background(), listOptions)
		if err != nil {
			return nil, err
		}

		ingresses = slice.PointersOf(list.Items).([]*networkingv1.Ingress)
	}

	return ingresses, nil
}
