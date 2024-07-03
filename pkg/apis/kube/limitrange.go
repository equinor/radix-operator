package kube

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// ApplyLimitRange Applies limit range to namespace
func (kubeutil *Kube) ApplyLimitRange(ctx context.Context, namespace string, limitRange *corev1.LimitRange) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Apply limit range %s", limitRange.Name)

	oldLimitRange, err := kubeutil.getLimitRange(ctx, namespace, limitRange.GetName())
	if err != nil && errors.IsNotFound(err) {
		createdLimitRange, err := kubeutil.kubeClient.CoreV1().LimitRanges(namespace).Create(ctx, limitRange, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create LimitRange object: %v", err)
		}

		logger.Debug().Msgf("Created LimitRange: %s in namespace %s", createdLimitRange.Name, namespace)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get limit range object: %v", err)
	}

	logger.Debug().Msgf("LimitRange object %s already exists in namespace %s, updating the object now", limitRange.GetName(), namespace)

	newLimitRange := oldLimitRange.DeepCopy()
	newLimitRange.ObjectMeta.OwnerReferences = limitRange.ObjectMeta.OwnerReferences
	newLimitRange.Spec = limitRange.Spec

	oldLimitRangeJSON, err := json.Marshal(oldLimitRange)
	if err != nil {
		return fmt.Errorf("failed to marshal old limitRange object: %v", err)
	}

	newLimitRangeJSON, err := json.Marshal(newLimitRange)
	if err != nil {
		return fmt.Errorf("failed to marshal new limitRange object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldLimitRangeJSON, newLimitRangeJSON, corev1.LimitRange{})
	if err != nil {
		return fmt.Errorf("failed to create two way merge patch limitRange objects: %v", err)
	}

	if !IsEmptyPatch(patchBytes) {
		patchedLimitRange, err := kubeutil.kubeClient.CoreV1().LimitRanges(namespace).Patch(ctx, limitRange.GetName(), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("failed to patch limitRange object: %v", err)
		}
		logger.Debug().Msgf("Patched limitRange: %s in namespace %s", patchedLimitRange.Name, namespace)
	} else {
		logger.Debug().Msgf("No need to patch limitRange: %s ", limitRange.GetName())
	}

	return nil
}

// BuildLimitRange Builds a limit range spec
func (kubeutil *Kube) BuildLimitRange(namespace, name, appName string, defaultResourceMemory, defaultRequestCPU, defaultRequestMemory *resource.Quantity) *corev1.LimitRange {

	defaultResources := make(corev1.ResourceList)
	defaultResources[corev1.ResourceMemory] = *defaultResourceMemory

	defaultRequest := make(corev1.ResourceList)
	defaultRequest[corev1.ResourceCPU] = *defaultRequestCPU
	defaultRequest[corev1.ResourceMemory] = *defaultRequestMemory

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
				{
					Type:           corev1.LimitTypeContainer,
					Default:        defaultResources,
					DefaultRequest: defaultRequest,
				},
			},
		},
	}

	return limitRange
}

func (kubeutil *Kube) getLimitRange(ctx context.Context, namespace, name string) (*corev1.LimitRange, error) {
	var limitRange *corev1.LimitRange
	var err error

	if kubeutil.LimitRangeLister != nil {
		limitRange, err = kubeutil.LimitRangeLister.LimitRanges(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		limitRange, err = kubeutil.kubeClient.CoreV1().LimitRanges(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return limitRange, nil
}
