package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
func (kubeutil *Kube) ListConfigMaps(namespace string) ([]*corev1.ConfigMap, error) {
	return kubeutil.listConfigMapsByLabels(namespace, make(map[string]string))
}

// ListEnvVarsConfigMaps Lists config maps which contain env vars
func (kubeutil *Kube) ListEnvVarsConfigMaps(namespace string) ([]*corev1.ConfigMap, error) {
	labelsMap := map[string]string{
		RadixConfigMapTypeLabel: string(EnvVarsConfigMap),
	}
	return kubeutil.listConfigMapsByLabels(namespace, labelsMap)
}

// ListEnvVarsMetadataConfigMaps Lists config maps which contain metadata of env vars
func (kubeutil *Kube) ListEnvVarsMetadataConfigMaps(namespace string) ([]*corev1.ConfigMap, error) {
	labelsMap := map[string]string{
		RadixConfigMapTypeLabel: string(EnvVarsMetadataConfigMap),
	}
	return kubeutil.listConfigMapsByLabels(namespace, labelsMap)
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
		log.Debugf("No need to patch config-map: %s ", currentConfigMap.GetName())
		return nil
	}

	log.Debugf("Patch: %s", string(patchBytes))
	patchedConfigMap, err := kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).Patch(context.TODO(), currentConfigMap.GetName(), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch config-map object: %v", err)
	}
	log.Debugf("Patched config-map: %s in namespace %s", patchedConfigMap.Name, namespace)
	return err
}

func (kubeutil *Kube) listConfigMapsByLabels(namespace string, labelsMap map[string]string) ([]*corev1.ConfigMap, error) {
	list, err := kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).List(
		context.TODO(), metav1.ListOptions{
			LabelSelector: labels.Set(metav1.LabelSelector{
				MatchLabels: labelsMap,
			}.MatchLabels).String(),
		},
	)
	if err != nil {
		return nil, err
	}
	cms := slice.PointersOf(list.Items).([]*corev1.ConfigMap)
	return cms, nil
}
