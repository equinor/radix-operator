package kube

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
)

// ApplyDeployment Create or update deployment in provided namespace
func (kube *Kube) ApplyDeployment(namespace string, deployment *v1beta1.Deployment) error {
	log.Debugf("Creating Deployment object %s in namespace %s", deployment.Name, namespace)
	createdDeployment, err := kube.kubeClient.ExtensionsV1beta1().Deployments(namespace).Create(deployment)
	if errors.IsAlreadyExists(err) {
		log.Debugf("Deployment object %s already exists in namespace %s, updating the object now", deployment.Name, namespace)
		updatedDeployment, err := kube.kubeClient.ExtensionsV1beta1().Deployments(namespace).Update(deployment)
		if err != nil {
			return fmt.Errorf("Failed to update Deployment object: %v", err)
		}
		log.Debugf("Updated Deployment: %s in namespace %s", updatedDeployment.Name, namespace)
		return nil
	}

	if err != nil {
		return fmt.Errorf("Failed to create Deployment object: %v", err)
	}

	log.Debugf("Created Deployment: %s in namespace %s", createdDeployment.Name, namespace)
	return nil
}
