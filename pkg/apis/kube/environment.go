package kube

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	radixv1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"github.com/statoil/radix-operator/pkg/apis/utils"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateSecrets should provision required secrets in the specified environment
// TODO : This should be moved closer to Deployment domain/package
func (k *Kube) CreateSecrets(registration *radixv1.RadixRegistration, deploy *radixv1.RadixDeployment) error {
	envName := deploy.Spec.Environment
	logger = logger.WithFields(log.Fields{"registrationName": registration.ObjectMeta.Name, "registrationNamespace": registration.ObjectMeta.Namespace})
	ns := utils.GetEnvironmentNamespace(registration.Name, envName)

	err := k.createDockerSecret(registration, ns)
	if err != nil {
		return err
	}

	logger.Infof("Apply empty secrets based on radix deployment obj")
	for _, component := range deploy.Spec.Components {
		if len(component.Secrets) > 0 {
			secretName := utils.GetComponentSecretName(component.Name)
			if k.isSecretExists(ns, secretName) {
				continue
			}
			secret := v1.Secret{
				Type: "Opaque",
				ObjectMeta: metav1.ObjectMeta{
					Name: secretName,
				},
			}
			_, err = k.ApplySecret(ns, &secret)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// CreateEnvironment creates a namespace with RadixRegistration as owner
// TODO : This should be moved closer to Deployment domain/package
func (k *Kube) CreateEnvironment(registration *radixv1.RadixRegistration, envName string) error {
	logger = logger.WithFields(log.Fields{"registrationName": registration.ObjectMeta.Name, "registrationNamespace": registration.ObjectMeta.Namespace})

	name := utils.GetEnvironmentNamespace(registration.Name, envName)

	labels := map[string]string{
		"radixApp":  registration.Name, // For backwards compatibility. Remove when cluster is migrated
		"radix-app": registration.Name,
		"radix-env": envName,
	}

	ownerRef := GetOwnerReferenceOfRegistration(registration)
	err := k.ApplyNamespace(name, labels, ownerRef)

	if err != nil {
		logger.Errorf("Failed to create namespace %s: %v", name, err)
		return err
	}

	logger.Infof("Created namespace %s", name)
	return nil
}

func (k *Kube) createDockerSecret(registration *radixv1.RadixRegistration, ns string) error {
	dockerSecret, err := k.kubeClient.CoreV1().Secrets("default").Get("radix-docker", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Could not find container registry credentials: %v", err)
	}
	dockerSecret.ResourceVersion = ""
	dockerSecret.Namespace = ns
	dockerSecret.UID = ""
	saveDockerSecret, err := k.ApplySecret(ns, dockerSecret)
	if err != nil {
		return fmt.Errorf("Failed to create container registry credentials secret in %s: %v", ns, err)
	}
	logger.Infof("Created container registry credentials secret: %s in namespace %s", saveDockerSecret.Name, ns)
	return nil
}
