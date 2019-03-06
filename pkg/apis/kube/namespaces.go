package kube

import (
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (kube *Kube) ApplyNamespace(name string, labels map[string]string, ownerRefs []metav1.OwnerReference) error {
	log.Debugf("Create namespace: %s", name)

	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			OwnerReferences: ownerRefs,
			Labels:          labels,
		},
	}
	_, err := kube.kubeClient.CoreV1().Namespaces().Create(&namespace)

	if errors.IsAlreadyExists(err) {
		log.Debugf("Namespace already exist %s", name)
		return nil
	}

	return err
}
