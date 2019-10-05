package kube

import (
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	labelHelpers "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// ApplyIngress Will create or update ingress in provided namespace
func (kube *Kube) ApplyIngress(namespace string, ingress *v1beta1.Ingress) error {
	ingressName := ingress.GetName()
	log.Debugf("Creating Ingress object %s in namespace %s", ingressName, namespace)

	oldIngress, err := kube.getIngress(namespace, ingressName)
	if err != nil && errors.IsNotFound(err) {
		_, err := kube.kubeClient.ExtensionsV1beta1().Ingresses(namespace).Create(ingress)
		if err != nil {
			return fmt.Errorf("Failed to create Ingress object: %v", err)
		}
		log.Debugf("#########YALLA##########Created Ingress: %s in namespace %s", ingressName, namespace)
		return nil
	}

	log.Debugf("Ingress object %s already exists in namespace %s, updating the object now", ingressName, namespace)

	newIngress := oldIngress.DeepCopy()
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

	if !isEmptyPatch(patchBytes) {
		log.Infof("#########YALLA##########Patch ingress with %s", string(patchBytes))
		patchedIngress, err := kube.kubeClient.ExtensionsV1beta1().Ingresses(namespace).Patch(ingressName, types.StrategicMergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch Ingress object: %v", err)
		}
		log.Infof("#########YALLA##########Patched Ingress: %s in namespace %s", patchedIngress.Name, namespace)
	} else {
		log.Infof("#########YALLA##########No need to patch ingress: %s ", ingressName)
	}

	return nil
}

func (kube *Kube) getIngress(namespace, name string) (*v1beta1.Ingress, error) {
	var ingress *v1beta1.Ingress
	var err error

	if kube.IngressLister != nil {
		ingress, err = kube.IngressLister.Ingresses(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		ingress, err = kube.kubeClient.ExtensionsV1beta1().Ingresses(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return ingress, nil
}

// ListIngresses lists ingresses
func (kube *Kube) ListIngresses(namespace string) ([]*v1beta1.Ingress, error) {
	return kube.ListIngressesWithSelector(namespace, nil)
}

// ListIngressesWithSelector lists ingresses
func (kube *Kube) ListIngressesWithSelector(namespace string, labelSelectorString *string) ([]*v1beta1.Ingress, error) {
	var ingresses []*v1beta1.Ingress
	var err error

	if kube.IngressLister != nil {
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

		ingresses, err = kube.IngressLister.Ingresses(namespace).List(selector)
		if err != nil {
			return nil, err
		}
	} else {
		listOptions := metav1.ListOptions{}
		if labelSelectorString != nil {
			listOptions.LabelSelector = *labelSelectorString
		}

		list, err := kube.kubeClient.ExtensionsV1beta1().Ingresses(namespace).List(listOptions)
		if err != nil {
			return nil, err
		}

		ingresses = slice.PointersOf(list.Items).([]*v1beta1.Ingress)
	}

	return ingresses, nil
}
