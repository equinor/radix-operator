package kube

import (
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// ApplyLimitRange Applies limit range to namespace
func (k *Kube) ApplyLimitRange(namespace string, limitRange *corev1.LimitRange) error {
	logger = logger.WithFields(log.Fields{"limitRange": limitRange.ObjectMeta.Name})

	logger.Debugf("Apply limit range %s", limitRange.Name)

	oldLimitRange, err := k.getLimitRange(namespace, limitRange.GetName())
	if err != nil && errors.IsNotFound(err) {
		createdLimitRange, err := k.kubeClient.CoreV1().LimitRanges(namespace).Create(limitRange)
		if err != nil {
			return fmt.Errorf("Failed to create LimitRange object: %v", err)
		}

		log.Debugf("Created LimitRange: %s in namespace %s", createdLimitRange.Name, namespace)
		return nil
	} else if err != nil {
		return fmt.Errorf("Failed to get limit range object: %v", err)
	}

	log.Debugf("LimitRange object %s already exists in namespace %s, updating the object now", limitRange.GetName(), namespace)

	newLimitRange := oldLimitRange.DeepCopy()
	newLimitRange.ObjectMeta.OwnerReferences = limitRange.ObjectMeta.OwnerReferences
	newLimitRange.Spec = limitRange.Spec

	oldLimitRangeJSON, err := json.Marshal(oldLimitRange)
	if err != nil {
		return fmt.Errorf("Failed to marshal old limitRange object: %v", err)
	}

	newLimitRangeJSON, err := json.Marshal(newLimitRange)
	if err != nil {
		return fmt.Errorf("Failed to marshal new limitRange object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldLimitRangeJSON, newLimitRangeJSON, corev1.LimitRange{})
	if err != nil {
		return fmt.Errorf("Failed to create two way merge patch limitRange objects: %v", err)
	}

	if !isEmptyPatch(patchBytes) {
		patchedLimitRange, err := k.kubeClient.CoreV1().LimitRanges(namespace).Patch(limitRange.GetName(), types.StrategicMergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch limitRange object: %v", err)
		}
		log.Debugf("Patched limitRange: %s in namespace %s", patchedLimitRange.Name, namespace)
	} else {
		log.Debugf("No need to patch limitRange: %s ", limitRange.GetName())
	}

	return nil
}

// BuildLimitRange Builds a limit range spec
func (k *Kube) BuildLimitRange(namespace, name, appName string,
	defaultResourceCPU, defaultResourceMemory, defaultRequestCPU, defaultRequestMemory resource.Quantity) *corev1.LimitRange {

	defaultResources := make(corev1.ResourceList)
	defaultResources[corev1.ResourceCPU] = defaultResourceCPU
	defaultResources[corev1.ResourceMemory] = defaultResourceMemory

	defaultRequest := make(corev1.ResourceList)
	defaultRequest[corev1.ResourceCPU] = defaultRequestCPU
	defaultRequest[corev1.ResourceMemory] = defaultRequestMemory

	limitRange := &corev1.LimitRange{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "LimitRange",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				RadixAppLabel: appName,
			},
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				corev1.LimitRangeItem{
					Type:           corev1.LimitTypeContainer,
					Default:        defaultResources,
					DefaultRequest: defaultRequest,
				},
			},
		},
	}

	return limitRange
}

func (k *Kube) getLimitRange(namespace, name string) (*corev1.LimitRange, error) {
	var limitRange *corev1.LimitRange
	var err error

	if k.LimitRangeLister != nil {
		limitRange, err = k.LimitRangeLister.LimitRanges(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		limitRange, err = k.kubeClient.CoreV1().LimitRanges(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return limitRange, nil
}
