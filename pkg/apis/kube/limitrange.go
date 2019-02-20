package kube

import (
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplyLimitRange Applies limit range to namespace
func (k *Kube) ApplyLimitRange(namespace string, limitRange *corev1.LimitRange) error {
	logger = logger.WithFields(log.Fields{"limitRange": limitRange.ObjectMeta.Name})

	logger.Infof("Apply limit range %s", limitRange.Name)

	_, err := k.kubeClient.CoreV1().LimitRanges(namespace).Create(limitRange)
	if errors.IsAlreadyExists(err) {
		_, err = k.kubeClient.CoreV1().LimitRanges(namespace).Update(limitRange)
		logger.Infof("Limit range %s already exists. Updating", limitRange.Name)
	}

	if err != nil {
		logger.Errorf("Failed to save limit range in [%s]: %v", namespace, err)
		return err
	}

	logger.Infof("Created roleBinding %s in %s", limitRange.Name, namespace)
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
