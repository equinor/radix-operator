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
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
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

	// Retain Keda label
	// ScaledObjectOwnerAnnotation is named Annotation, but its used as a Label
	if val, ok := oldScaler.ObjectMeta.Labels[v1alpha1.ScaledObjectOwnerAnnotation]; ok {
		newScaler.ObjectMeta.Labels[v1alpha1.ScaledObjectOwnerAnnotation] = val
	}

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

	mergepatch, err := jsonmergepatch.CreateThreeWayJSONMergePatch(oldScalerJSON, newScalerJSON, oldScalerJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to create json merge patch ScaledObject objects: %v: %v", err, mergepatch)
	}

	if IsEmptyPatch(mergepatch) {
		log.Debug().Msgf("No need to patch ScaledObject: %s ", scalerName)
		return oldScaledObject, nil
	}

	patchedScaler, err := kubeutil.kedaClient.KedaV1alpha1().ScaledObjects(namespace).Patch(ctx, scalerName, types.MergePatchType, mergepatch, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to patch ScaledObject object: %v", err)
	}
	log.Debug().Msgf("Patched ScaledObject: %s in namespace %s", patchedScaler.Name, namespace)

	return patchedScaler, nil
}

// ApplyTriggerAuthentication Will create or update ScaledObject in provided namespace
func (kubeutil *Kube) ApplyTriggerAuthentication(ctx context.Context, namespace string, auth v1alpha1.TriggerAuthentication) error {
	name := auth.GetName()
	log.Ctx(ctx).Debug().Msgf("Creating TriggerAuthentication object %s in namespace %s", name, namespace)

	oldAuth, err := kubeutil.kedaClient.KedaV1alpha1().TriggerAuthentications(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		_, err := kubeutil.kedaClient.KedaV1alpha1().TriggerAuthentications(namespace).Create(ctx, &auth, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create TriggerAuthentication object: %v", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get TriggerAuthentication object: %v", err)
	}

	log.Ctx(ctx).Debug().Msgf("TriggerAuthentication object %s already exists in namespace %s, updating the object now", name, namespace)
	newAuth := oldAuth.DeepCopy()
	newAuth.ObjectMeta.Labels = auth.ObjectMeta.Labels
	newAuth.ObjectMeta.Annotations = auth.ObjectMeta.Annotations
	newAuth.ObjectMeta.OwnerReferences = auth.ObjectMeta.OwnerReferences
	newAuth.Spec = auth.Spec

	_, err = kubeutil.PatchTriggerAuthentication(ctx, namespace, oldAuth, newAuth)
	return err
}

// PatchTriggerAuthentication Patches an TriggerAuthentication, if there are changes
func (kubeutil *Kube) PatchTriggerAuthentication(ctx context.Context, namespace string, oldAuth *v1alpha1.TriggerAuthentication, newAuthAuth *v1alpha1.TriggerAuthentication) (*v1alpha1.TriggerAuthentication, error) {
	authName := oldAuth.GetName()
	log.Ctx(ctx).Debug().Msgf("patch an TriggerAuthentication %s in the namespace %s", authName, namespace)
	oldAuthJSON, err := json.Marshal(oldAuth)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal old TriggerAuthentication object: %v", err)
	}

	newAuthJSON, err := json.Marshal(newAuthAuth)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal new TriggerAuthentication object: %v", err)
	}

	mergepatch, err := jsonmergepatch.CreateThreeWayJSONMergePatch(oldAuthJSON, newAuthJSON, oldAuthJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to create json merge patch TriggerAuthentication objects: %v: %v", err, mergepatch)
	}

	if IsEmptyPatch(mergepatch) {
		log.Debug().Msgf("No need to patch TriggerAuthentication: %s ", authName)
		return oldAuth, nil
	}

	patchedAuth, err := kubeutil.kedaClient.KedaV1alpha1().TriggerAuthentications(namespace).Patch(ctx, authName, types.MergePatchType, mergepatch, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to patch TriggerAuthentication object: %v", err)
	}
	log.Debug().Msgf("Patched TriggerAuthentication: %s in namespace %s", patchedAuth.Name, namespace)

	return patchedAuth, nil
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
