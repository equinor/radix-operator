package kube

import (
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
