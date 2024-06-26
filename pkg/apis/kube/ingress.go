package kube

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	"github.com/rs/zerolog/log"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// GetIngress Gets an ingress by its name
func (kubeutil *Kube) GetIngress(ctx context.Context, namespace, name string) (*networkingv1.Ingress, error) {
	return kubeutil.kubeClient.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
}

// ApplyIngress Will create or update ingress in provided namespace
func (kubeutil *Kube) ApplyIngress(ctx context.Context, namespace string, ingress *networkingv1.Ingress) error {
	ingressName := ingress.GetName()
	log.Ctx(ctx).Debug().Msgf("Creating Ingress object %s in namespace %s", ingressName, namespace)

	oldIngress, err := kubeutil.kubeClient.NetworkingV1().Ingresses(namespace).Get(ctx, ingressName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		_, err := kubeutil.kubeClient.NetworkingV1().Ingresses(namespace).Create(ctx, ingress, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create Ingress object: %v", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get Ingress object: %v", err)
	}

	log.Ctx(ctx).Debug().Msgf("Ingress object %s already exists in namespace %s, updating the object now", ingressName, namespace)
	newIngress := oldIngress.DeepCopy()
	newIngress.ObjectMeta.Labels = ingress.ObjectMeta.Labels
	newIngress.ObjectMeta.Annotations = ingress.ObjectMeta.Annotations
	newIngress.ObjectMeta.OwnerReferences = ingress.ObjectMeta.OwnerReferences
	newIngress.Spec = ingress.Spec
	_, err = kubeutil.PatchIngress(ctx, namespace, oldIngress, newIngress)
	return err
}

// PatchIngress Patches an ingress, if there are changes
func (kubeutil *Kube) PatchIngress(ctx context.Context, namespace string, oldIngress *networkingv1.Ingress, newIngress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	ingressName := oldIngress.GetName()
	log.Ctx(ctx).Debug().Msgf("patch an ingress %s in the namespace %s", ingressName, namespace)
	oldIngressJSON, err := json.Marshal(oldIngress)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal old Ingress object: %v", err)
	}

	newIngressJSON, err := json.Marshal(newIngress)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal new Ingress object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldIngressJSON, newIngressJSON, networkingv1.Ingress{})
	if err != nil {
		return nil, fmt.Errorf("failed to create two way merge patch Ingess objects: %v", err)
	}

	if IsEmptyPatch(patchBytes) {
		log.Ctx(ctx).Debug().Msgf("No need to patch ingress: %s ", ingressName)
		return oldIngress, nil
	}
	patchedIngress, err := kubeutil.kubeClient.NetworkingV1().Ingresses(namespace).Patch(ctx, ingressName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to patch Ingress object: %v", err)
	}
	log.Ctx(ctx).Debug().Msgf("Patched Ingress: %s in namespace %s", patchedIngress.Name, namespace)

	return patchedIngress, nil
}

// ListIngresses lists ingresses
func (kubeutil *Kube) ListIngresses(ctx context.Context, namespace string) ([]*networkingv1.Ingress, error) {
	return kubeutil.ListIngressesWithSelector(ctx, namespace, "")
}

// ListIngressesWithSelector lists ingresses
func (kubeutil *Kube) ListIngressesWithSelector(ctx context.Context, namespace string, labelSelectorString string) ([]*networkingv1.Ingress, error) {
	if kubeutil.IngressLister == nil {
		list, err := kubeutil.kubeClient.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelectorString})
		if err != nil {
			return nil, err
		}
		return slice.PointersOf(list.Items).([]*networkingv1.Ingress), nil
	}
	selector, err := labels.Parse(labelSelectorString)
	if err != nil {
		return nil, err
	}
	ingresses, err := kubeutil.IngressLister.Ingresses(namespace).List(selector)
	if err != nil {
		return nil, err
	}
	return ingresses, nil
}

// DeleteIngresses Deletes ingresses
func (kubeutil *Kube) DeleteIngresses(ctx context.Context, ingresses ...networkingv1.Ingress) error {
	log.Ctx(ctx).Debug().Msgf("delete %d Ingress(es)", len(ingresses))
	for _, ing := range ingresses {
		if err := kubeutil.KubeClient().NetworkingV1().Ingresses(ing.Namespace).Delete(ctx, ing.Name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}
