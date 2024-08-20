package kube

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	k8errs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// ApplyNamespace Creates a new namespace, if not exists already
func (kubeutil *Kube) ApplyNamespace(ctx context.Context, name string, labels map[string]string, ownerRefs []metav1.OwnerReference) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Create namespace: %s", name)

	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			OwnerReferences: ownerRefs,
			Labels:          labels,
		},
	}

	oldNamespace, err := kubeutil.getNamespace(ctx, name)
	if err != nil && k8errs.IsNotFound(err) {
		logger.Debug().Msgf("namespace object %s doesn't exists, create the object", name)
		_, err := kubeutil.kubeClient.CoreV1().Namespaces().Create(ctx, &namespace, metav1.CreateOptions{})
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
		patchedNamespace, err := kubeutil.kubeClient.CoreV1().Namespaces().Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("failed to patch namespace object: %v", err)
		}

		logger.Debug().Msgf("Patched namespace: %s ", patchedNamespace.Name)
		return nil
	}

	logger.Debug().Msgf("No need to patch namespace: %s ", name)
	return nil
}

func (kubeutil *Kube) getNamespace(ctx context.Context, name string) (*corev1.Namespace, error) {
	var namespace *corev1.Namespace
	var err error

	if kubeutil.NamespaceLister != nil {
		namespace, err = kubeutil.NamespaceLister.Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		namespace, err = kubeutil.kubeClient.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return namespace, nil
}

// ListNamespacesWithSelector List namespaces with selector
func (kubeutil *Kube) ListNamespacesWithSelector(ctx context.Context, namespace, labelSelectorString string) ([]*corev1.Namespace, error) {
	if kubeutil.NamespaceLister != nil {
		selector, err := labels.Parse(labelSelectorString)
		if err != nil {
			return nil, err
		}
		return kubeutil.NamespaceLister.List(selector)
	}

	list, err := kubeutil.kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{LabelSelector: labelSelectorString})
	if err != nil {
		return nil, err
	}

	return slice.PointersOf(list.Items).([]*corev1.Namespace), nil

}
