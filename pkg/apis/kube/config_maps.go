package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateConfigMap Create config map
func (kubeutil *Kube) CreateConfigMap(namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	return kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).Create(context.TODO(),
		configMap,
		metav1.CreateOptions{})
}

// GetConfigMap Gets config map by name
func (kubeutil *Kube) GetConfigMap(namespace, name string) (*corev1.ConfigMap, error) {
	var configMap *corev1.ConfigMap
	var err error

	if kubeutil.ConfigMapLister != nil {
		configMap, err = kubeutil.ConfigMapLister.ConfigMaps(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		configMap, err = kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		log.Debugf("Created environment variables ConfigMap  '%s'", configMap.GetName())
	}

	return configMap, nil
}

//UpdateConfigMap Update config-maps
func (kubeutil *Kube) UpdateConfigMap(namespace string, configMaps ...*corev1.ConfigMap) error {
	for _, configMap := range configMaps {
		_, err := kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

//ApplyConfigMap Patch changes of environment-variables to config-map if any
func (kubeutil *Kube) ApplyConfigMap(namespace string, currentConfigMap, desiredConfigMap *corev1.ConfigMap) error {
	currentConfigMapJSON, err := json.Marshal(currentConfigMap)
	if err != nil {
		return fmt.Errorf("Failed to marshal old config-map object: %v", err)
	}

	desiredConfigMapJSON, err := json.Marshal(desiredConfigMap)
	if err != nil {
		return fmt.Errorf("Failed to marshal new config-map object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(currentConfigMapJSON, desiredConfigMapJSON, corev1.ConfigMap{})
	if err != nil {
		return fmt.Errorf("Failed to create two way merge patch config-map objects: %v", err)
	}

	if utils.IsEmptyPatch(patchBytes) {
		log.Debugf("No need to patch config-map: %s ", currentConfigMap.GetName())
		return nil
	}

	log.Debugf("Patch: %s", string(patchBytes))
	patchedConfigMap, err := kubeutil.kubeClient.CoreV1().ConfigMaps(namespace).Patch(context.TODO(), currentConfigMap.GetName(), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("Failed to patch config-map object: %v", err)
	}
	log.Debugf("Patched config-map: %s in namespace %s", patchedConfigMap.Name, namespace)
	return err
}
