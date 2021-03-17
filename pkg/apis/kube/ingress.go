package kube

import (
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	labelHelpers "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// ApplyIngress Will create or update ingress in provided namespace
func (kubeutil *Kube) ApplyIngress(namespace string, ingress *networkingv1beta1.Ingress) error {
	ingressName := ingress.GetName()
	log.Debugf("Creating Ingress object %s in namespace %s", ingressName, namespace)

	oldIngress, err := kubeutil.getIngress(namespace, ingressName)
	if err != nil && errors.IsNotFound(err) {
		_, err := kubeutil.kubeClient.NetworkingV1beta1().Ingresses(namespace).Create(ingress)
		if err != nil {
			return fmt.Errorf("Failed to create Ingress object: %v", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("Failed to get Ingress object: %v", err)
	}

	log.Debugf("Ingress object %s already exists in namespace %s, updating the object now", ingressName, namespace)
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

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldIngressJSON, newIngressJSON, networkingv1beta1.Ingress{})
	if err != nil {
		return fmt.Errorf("Failed to create two way merge patch Ingess objects: %v", err)
	}

	if !isEmptyPatch(patchBytes) {
		patchedIngress, err := kubeutil.kubeClient.NetworkingV1beta1().Ingresses(namespace).Patch(ingressName, types.StrategicMergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch Ingress object: %v", err)
		}
		log.Debugf("Patched Ingress: %s in namespace %s", patchedIngress.Name, namespace)
	} else {
		log.Debugf("No need to patch ingress: %s ", ingressName)
	}

	return nil
}

func (kubeutil *Kube) getIngress(namespace, name string) (*networkingv1beta1.Ingress, error) {
	var ingress *networkingv1beta1.Ingress
	var err error

	if kubeutil.IngressLister != nil {
		ingress, err = kubeutil.IngressLister.Ingresses(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		ingress, err = kubeutil.kubeClient.NetworkingV1beta1().Ingresses(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return ingress, nil
}

// ListIngresses lists ingresses
func (kubeutil *Kube) ListIngresses(namespace string) ([]*networkingv1beta1.Ingress, error) {
	return kubeutil.ListIngressesWithSelector(namespace, nil)
}

// ListIngressesWithSelector lists ingresses
func (kubeutil *Kube) ListIngressesWithSelector(namespace string, labelSelectorString *string) ([]*networkingv1beta1.Ingress, error) {
	var ingresses []*networkingv1beta1.Ingress
	var err error

	if kubeutil.IngressLister != nil {
		var selector labels.Selector
		if labelSelectorString != nil {
			labelSelector, err := labelHelpers.ParseToLabelSelector(*labelSelectorString)
			if err != nil {
				return nil, err
			}

			selector, err = labelHelpers.LabelSelectorAsSelector(labelSelector)
			if err != nil {
				return nil, err
			}

		} else {
			selector = labels.NewSelector()
		}

		ingresses, err = kubeutil.IngressLister.Ingresses(namespace).List(selector)
		if err != nil {
			return nil, err
		}
	} else {
		listOptions := metav1.ListOptions{}
		if labelSelectorString != nil {
			listOptions.LabelSelector = *labelSelectorString
		}

		list, err := kubeutil.kubeClient.NetworkingV1beta1().Ingresses(namespace).List(listOptions)
		if err != nil {
			return nil, err
		}

		ingresses = slice.PointersOf(list.Items).([]*networkingv1beta1.Ingress)
	}

	return ingresses, nil
}
