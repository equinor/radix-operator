package kube

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// GetScaledObject Gets an ScaledObject by its name
func (kubeutil *Kube) GetScaledObject(ctx context.Context, namespace, name string) (*v1alpha1.ScaledObject, error) {
	return kubeutil.kedaClient.KedaV1alpha1().ScaledObjects(namespace).Get(ctx, name, metav1.GetOptions{})
}

// ApplyScaledObject Will create or update ScaledObject in provided namespace
func (kubeutil *Kube) ApplyScaledObject(ctx context.Context, namespace string, scaledObject *v1alpha1.ScaledObject) error {
	scalerName := scaledObject.GetName()
	log.Ctx(ctx).Debug().Msgf("Creating ScaledObject object %s in namespace %s", scalerName, namespace)

	oldScaler, err := kubeutil.kedaClient.KedaV1alpha1().ScaledObjects(namespace).Get(ctx, scalerName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		_, err := kubeutil.kedaClient.KedaV1alpha1().ScaledObjects(namespace).Create(ctx, scaledObject, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create ScaledObject object: %v", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get ScaledObject object: %v", err)
	}

	log.Ctx(ctx).Debug().Msgf("ScaledObject object %s already exists in namespace %s, updating the object now", scalerName, namespace)
	newScaler := oldScaler.DeepCopy()
	newScaler.ObjectMeta.Labels = scaledObject.ObjectMeta.Labels
	newScaler.ObjectMeta.Annotations = scaledObject.ObjectMeta.Annotations
	newScaler.ObjectMeta.OwnerReferences = scaledObject.ObjectMeta.OwnerReferences
	newScaler.Spec = scaledObject.Spec
	_, err = kubeutil.PatchScaledObject(ctx, namespace, oldScaler, newScaler)
	return err
}

// PatchScaledObject Patches an ScaledObject, if there are changes
func (kubeutil *Kube) PatchScaledObject(ctx context.Context, namespace string, oldScaledObject *v1alpha1.ScaledObject, newScaledObject *v1alpha1.ScaledObject) (*v1alpha1.ScaledObject, error) {
	scalerName := oldScaledObject.GetName()
	log.Ctx(ctx).Debug().Msgf("patch an ScaledObject %s in the namespace %s", scalerName, namespace)
	oldScalerJSON, err := json.Marshal(oldScaledObject)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal old ScaledObject object: %v", err)
	}

	newScalerJSON, err := json.Marshal(newScaledObject)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal new ScaledObject object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldScalerJSON, newScalerJSON, v1alpha1.ScaledObject{})
	if err != nil {
		return nil, fmt.Errorf("failed to create two way merge patch Ingess objects: %v", err)
	}

	if IsEmptyPatch(patchBytes) {
		log.Debug().Msgf("No need to patch ScaledObject: %s ", scalerName)
		return oldScaledObject, nil
	}
	patchedScaler, err := kubeutil.kedaClient.KedaV1alpha1().ScaledObjects(namespace).Patch(ctx, scalerName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to patch ScaledObject object: %v", err)
	}
	log.Debug().Msgf("Patched ScaledObject: %s in namespace %s", patchedScaler.Name, namespace)

	return patchedScaler, nil
}

// ListScaledObject lists ScaledObject
func (kubeutil *Kube) ListScaledObject(ctx context.Context, namespace string) ([]*v1alpha1.ScaledObject, error) {
	return kubeutil.ListScaledObjectWithSelector(ctx, namespace, "")
}

// ListScaledObjectWithSelector lists ScaledObject
func (kubeutil *Kube) ListScaledObjectWithSelector(ctx context.Context, namespace string, labelSelectorString string) ([]*v1alpha1.ScaledObject, error) {
	if kubeutil.ScaledObjectLister == nil {
		list, err := kubeutil.kedaClient.KedaV1alpha1().ScaledObjects(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelectorString})
		if err != nil {
			return nil, err
		}
		return slice.PointersOf(list.Items).([]*v1alpha1.ScaledObject), nil
	}
	selector, err := labels.Parse(labelSelectorString)
	if err != nil {
		return nil, err
	}
	scalers, err := kubeutil.ScaledObjectLister.ScaledObjects(namespace).List(selector)
	if err != nil {
		return nil, err
	}
	return scalers, nil
}

// DeleteScaledObject Deletes ScaledObject
func (kubeutil *Kube) DeleteScaledObject(ctx context.Context, scaledObjects ...*v1alpha1.ScaledObject) error {
	log.Debug().Msgf("delete %d ScaledObject(s)", len(scaledObjects))
	for _, ing := range scaledObjects {
		if err := kubeutil.kedaClient.KedaV1alpha1().ScaledObjects(ing.Namespace).Delete(ctx, ing.Name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}
