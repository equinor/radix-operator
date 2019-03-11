package kube

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
)

// ApplyIngress Will create or update ingress in provided namespace
func (kube *Kube) ApplyIngress(namespace string, ingress *v1beta1.Ingress) error {
	ingressName := ingress.GetName()
	log.Debugf("Creating Ingress object %s in namespace %s", ingressName, namespace)

	_, err := kube.kubeClient.ExtensionsV1beta1().Ingresses(namespace).Create(ingress)
	if errors.IsAlreadyExists(err) {
		log.Debugf("Ingress object %s already exists in namespace %s, updating the object now", ingressName, namespace)
		_, err := kube.kubeClient.ExtensionsV1beta1().Ingresses(namespace).Update(ingress)
		if err != nil {
			return fmt.Errorf("Failed to update Ingress object: %v", err)
		}
		log.Debugf("Updated Ingress: %s in namespace %s", ingressName, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("Failed to create Ingress object: %v", err)
	}
	log.Debugf("Created Ingress: %s in namespace %s", ingressName, namespace)
	return nil
}
