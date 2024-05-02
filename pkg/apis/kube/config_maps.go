package kube

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// CreateConfigMap Create config map
func (kubeutil *Kube) CreateConfigMap(namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	return kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).Create(context.TODO(),
		configMap,
		metav1.CreateOptions{})
}

// GetConfigMap Gets config map by name
func (kubeutil *Kube) GetConfigMap(namespace, name string) (*corev1.ConfigMap, error) {
	return kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
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

// UpdateConfigMap Update config-maps
func (kubeutil *Kube) UpdateConfigMap(namespace string, configMaps ...*corev1.ConfigMap) error {
	for _, configMap := range configMaps {
		_, err := kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteConfigMap Deletes config-maps
func (kubeutil *Kube) DeleteConfigMap(namespace string, name string) error {
	return kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).Delete(context.TODO(),
		name,
		metav1.DeleteOptions{})
}

// ApplyConfigMap Patch changes of environment-variables to config-map if any
func (kubeutil *Kube) ApplyConfigMap(namespace string, currentConfigMap, desiredConfigMap *corev1.ConfigMap) error {
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
		log.Debug().Msgf("No need to patch config-map: %s ", currentConfigMap.GetName())
		return nil
	}

	log.Debug().Msgf("Patch: %s", string(patchBytes))
	patchedConfigMap, err := kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).Patch(context.TODO(), currentConfigMap.GetName(), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch config-map object: %v", err)
	}
	log.Debug().Msgf("Patched config-map: %s in namespace %s", patchedConfigMap.Name, namespace)
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
