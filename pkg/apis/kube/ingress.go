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

// ApplyIngress Will create or update ingress in provided namespace
func (kubeutil *Kube) ApplyIngress(namespace string, ingress *networkingv1.Ingress) error {
	ingressName := ingress.GetName()
	log.Debugf("Creating Ingress object %s in namespace %s", ingressName, namespace)

	oldIngress, err := kubeutil.getIngress(namespace, ingressName)
	if err != nil && errors.IsNotFound(err) {
		_, err := kubeutil.kubeClient.NetworkingV1().Ingresses(namespace).Create(context.TODO(), ingress, metav1.CreateOptions{})
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
		patchedIngress, err := kubeutil.kubeClient.NetworkingV1().Ingresses(namespace).Patch(context.TODO(), ingressName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("failed to patch Ingress object: %v", err)
		}
		log.Debugf("Patched Ingress: %s in namespace %s", patchedIngress.Name, namespace)
	} else {
		log.Debugf("No need to patch ingress: %s ", ingressName)
	}

	return nil
}

func (kubeutil *Kube) getIngress(namespace, name string) (*networkingv1.Ingress, error) {
	var ingress *networkingv1.Ingress
	var err error

	if kubeutil.IngressLister != nil {
		ingress, err = kubeutil.IngressLister.Ingresses(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		ingress, err = kubeutil.kubeClient.NetworkingV1().Ingresses(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return ingress, nil
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

		list, err := kubeutil.kubeClient.NetworkingV1().Ingresses(namespace).List(context.TODO(), listOptions)
		if err != nil {
			return nil, err
		}

		ingresses = slice.PointersOf(list.Items).([]*networkingv1.Ingress)
	}

	return ingresses, nil
}
