package kube

import (
	log "github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretExists Checks if secret allready exists
func (k *Kube) SecretExists(namespace, secretName string) bool {
	_, err := k.kubeClient.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		return false
	}
	if err != nil {
		log.Errorf("Failed to get secret %s in namespace %s. %v", secretName, namespace, err)
		return false
	}
	return true
}

func (k *Kube) ApplySecret(namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	secretName := secret.ObjectMeta.Name
	log.Debugf("Applies secret %s in namespace %s", secretName, namespace)

	savedSecret, err := k.kubeClient.CoreV1().Secrets(namespace).Create(secret)
	if errors.IsAlreadyExists(err) {
		log.Debugf("Updating secret %s that already exists in namespace %s.", secretName, namespace)
		savedSecret, err = k.kubeClient.CoreV1().Secrets(namespace).Update(secret)
	}

	if err != nil {
		log.Errorf("Failed to apply secret %s in namespace %s. %v", secretName, namespace, err)
		return nil, err
	}
	log.Debugf("Applied secret %s in namespace %s", secretName, namespace)
	return savedSecret, nil
}
