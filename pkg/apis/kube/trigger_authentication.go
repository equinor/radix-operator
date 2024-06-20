package kube

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
)

// ApplyTriggerAuthentication Will create or update TriggerAuthentication in provided namespace
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
		log.Ctx(ctx).Debug().Msgf("No need to patch TriggerAuthentication: %s ", authName)
		return oldAuth, nil
	}

	patchedAuth, err := kubeutil.kedaClient.KedaV1alpha1().TriggerAuthentications(namespace).Patch(ctx, authName, types.MergePatchType, mergepatch, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to patch TriggerAuthentication object: %v", err)
	}
	log.Ctx(ctx).Debug().Msgf("Patched TriggerAuthentication: %s in namespace %s", patchedAuth.Name, namespace)

	return patchedAuth, nil
}

// ListTriggerAuthentications lists TriggerAuths
func (kubeutil *Kube) ListTriggerAuthentications(ctx context.Context, namespace string) ([]*v1alpha1.TriggerAuthentication, error) {
	return kubeutil.ListTriggerAuthenticationsWithSelector(ctx, namespace, "")
}

// ListTriggerAuthenticationsWithSelector lists TriggerAuths
func (kubeutil *Kube) ListTriggerAuthenticationsWithSelector(ctx context.Context, namespace string, labelSelectorString string) ([]*v1alpha1.TriggerAuthentication, error) {
	if kubeutil.TriggerAuthLister == nil {
		list, err := kubeutil.kedaClient.KedaV1alpha1().TriggerAuthentications(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelectorString})
		if err != nil {
			return nil, err
		}
		return slice.PointersOf(list.Items).([]*v1alpha1.TriggerAuthentication), nil
	}
	selector, err := labels.Parse(labelSelectorString)
	if err != nil {
		return nil, err
	}
	auths, err := kubeutil.TriggerAuthLister.TriggerAuthentications(namespace).List(selector)
	if err != nil {
		return nil, err
	}
	return auths, nil
}

// DeleteTriggerAuthentication Deletes TriggerAuthentications
func (kubeutil *Kube) DeleteTriggerAuthentication(ctx context.Context, triggerAuth ...*v1alpha1.TriggerAuthentication) error {
	log.Ctx(ctx).Debug().Msgf("delete %d TriggerAuthentication(s)", len(triggerAuth))
	for _, ing := range triggerAuth {
		if err := kubeutil.kedaClient.KedaV1alpha1().TriggerAuthentications(ing.Namespace).Delete(ctx, ing.Name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}
