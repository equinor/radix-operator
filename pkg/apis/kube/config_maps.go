package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// GetConfigMap Gets config map by name
func (kubeutil *Kube) GetConfigMap(ctx context.Context, namespace, name string) (*corev1.ConfigMap, error) {
	return kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
}

// ListConfigMaps Lists config maps in namespace
func (kubeutil *Kube) ListConfigMaps(ctx context.Context, namespace string) ([]*corev1.ConfigMap, error) {
	return kubeutil.ListConfigMapsWithSelector(ctx, namespace, "")
}

// ListEnvVarsConfigMaps Lists config maps which contain env vars
func (kubeutil *Kube) ListEnvVarsConfigMaps(ctx context.Context, namespace string) ([]*corev1.ConfigMap, error) {
	return kubeutil.ListConfigMapsWithSelector(ctx, namespace, getEnvVarsConfigMapSelector().String())
}

// ListEnvVarsMetadataConfigMaps Lists config maps which contain metadata of env vars
func (kubeutil *Kube) ListEnvVarsMetadataConfigMaps(ctx context.Context, namespace string) ([]*corev1.ConfigMap, error) {
	return kubeutil.ListConfigMapsWithSelector(ctx, namespace, getEnvVarsMetadataConfigMapSelector().String())
}

// CreateConfigMap Create config map
func (kubeutil *Kube) CreateConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	created, err := kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{})
	log.Ctx(ctx).Info().Msgf("Created config map %s/%s", created.Namespace, created.Name)
	return created, err
}

// UpdateConfigMap updates the `modified` configmap.
// If `original` is set, the two configmaps are compared, and the secret is only updated if they are not equal.
func (kubeutil *Kube) UpdateConfigMap(ctx context.Context, original, modified *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	if original != nil && reflect.DeepEqual(original, modified) {
		log.Ctx(ctx).Debug().Msgf("No need to update configmap %s/%s", modified.Namespace, modified.Name)
		return modified, nil
	}

	updated, err := kubeutil.kubeClient.CoreV1().ConfigMaps(modified.Namespace).Update(ctx, modified, metav1.UpdateOptions{})
	log.Ctx(ctx).Info().Msgf("Updated configmap %s/%s", updated.Namespace, updated.Name)
	return updated, err
}

// DeleteConfigMap Deletes config-maps
func (kubeutil *Kube) DeleteConfigMap(ctx context.Context, namespace string, name string) error {
	return kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).Delete(ctx,
		name,
		metav1.DeleteOptions{})
}

// ApplyConfigMap Patch changes of environment-variables to config-map if any
func (kubeutil *Kube) ApplyConfigMap(ctx context.Context, namespace string, currentConfigMap, desiredConfigMap *corev1.ConfigMap) error {
	currentConfigMapJSON, err := json.Marshal(currentConfigMap)
	if err != nil {
		return fmt.Errorf("failed to marshal old config-map object: %v", err)
	}

	desiredConfigMapJSON, err := json.Marshal(desiredConfigMap)
	if err != nil {
		return fmt.Errorf("failed to marshal new config-map object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(currentConfigMapJSON, desiredConfigMapJSON, corev1.ConfigMap{})
	if err != nil {
		return fmt.Errorf("failed to create two way merge patch config-map objects: %v", err)
	}

	if IsEmptyPatch(patchBytes) {
		log.Ctx(ctx).Debug().Msgf("No need to patch config-map: %s ", currentConfigMap.GetName())
		return nil
	}

	log.Ctx(ctx).Debug().Msgf("Patch: %s", string(patchBytes))
	patchedConfigMap, err := kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).Patch(ctx, currentConfigMap.GetName(), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch config-map object: %v", err)
	}
	log.Ctx(ctx).Debug().Msgf("Patched config-map: %s in namespace %s", patchedConfigMap.Name, namespace)
	return err
}

// ListConfigMapsWithSelector Get a list of ConfigMaps by Label requirements
func (kubeutil *Kube) ListConfigMapsWithSelector(ctx context.Context, namespace string, labelSelectorString string) ([]*corev1.ConfigMap, error) {
	listOptions := metav1.ListOptions{LabelSelector: labelSelectorString}
	list, err := kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}
	return slice.PointersOf(list.Items).([]*corev1.ConfigMap), nil
}

func getEnvVarsConfigMapSelector() labels.Selector {
	requirement, _ := labels.NewRequirement(RadixConfigMapTypeLabel, selection.Equals, []string{string(EnvVarsConfigMap)})
	return labels.NewSelector().Add(*requirement)
}

func getEnvVarsMetadataConfigMapSelector() labels.Selector {
	requirement, _ := labels.NewRequirement(RadixConfigMapTypeLabel, selection.Equals, []string{string(EnvVarsMetadataConfigMap)})
	return labels.NewSelector().Add(*requirement)
}
