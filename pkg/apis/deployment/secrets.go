package deployment

import (
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) createSecrets(registration *radixv1.RadixRegistration, deployment *radixv1.RadixDeployment) error {
	envName := deployment.Spec.Environment
	ns := utils.GetEnvironmentNamespace(registration.Name, envName)

	err := deploy.createDockerSecret(registration, ns)
	if err != nil {
		return err
	}

	log.Infof("Apply empty secrets based on radix deployment obj")
	for _, component := range deployment.Spec.Components {
		if len(component.Secrets) > 0 {
			secretName := utils.GetComponentSecretName(component.Name)
			if deploy.kubeutil.SecretExists(ns, secretName) {
				continue
			}
			secret := v1.Secret{
				Type: "Opaque",
				ObjectMeta: metav1.ObjectMeta{
					Name: secretName,
				},
			}
			_, err = deploy.kubeutil.ApplySecret(ns, &secret)
			if err != nil {
				return err
			}

			err = deploy.grantAppAdminAccessToRuntimeSecrets(deployment.Namespace, registration, &component)
			if err != nil {
				return fmt.Errorf("Failed to grant app admin access to own secrets. %v", err)
			}
		}
	}
	return nil
}

func (deploy *Deployment) createDockerSecret(registration *radixv1.RadixRegistration, ns string) error {
	dockerSecret, err := deploy.kubeclient.CoreV1().Secrets("default").Get("radix-docker", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Could not find container registry credentials: %v", err)
	}
	dockerSecret.ResourceVersion = ""
	dockerSecret.Namespace = ns
	dockerSecret.UID = ""
	saveDockerSecret, err := deploy.kubeutil.ApplySecret(ns, dockerSecret)
	if err != nil {
		return fmt.Errorf("Failed to create container registry credentials secret in %s: %v", ns, err)
	}

	log.Infof("Created container registry credentials secret: %s in namespace %s", saveDockerSecret.Name, ns)
	return nil
}
