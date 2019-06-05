package deployment

import (
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
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

	log.Debugf("Apply empty secrets based on radix deployment obj")
	for _, component := range deployment.Spec.Components {
		secretsToManage := make([]string, 0)

		if len(component.Secrets) > 0 {
			secretName := utils.GetComponentSecretName(component.Name)
			if deploy.kubeutil.SecretExists(ns, secretName) {
				continue
			}

			err := deploy.createSecret(ns, registration.Name, component.Name, secretName)
			if err != nil {
				return err
			}

			secretsToManage = append(secretsToManage, secretName)
		}

		if len(component.DNSExternalAlias) > 0 {
			// Create secrets to hold TLS certificates
			for n := range component.DNSExternalAlias {
				secretName := fmt.Sprintf(externalAliasTLSCertificateFormat, component.Name, (n + 1))
				if deploy.kubeutil.SecretExists(ns, secretName) {
					continue
				}

				err := deploy.createSecret(ns, registration.Name, component.Name, secretName)
				if err != nil {
					return err
				}

				secretsToManage = append(secretsToManage, secretName)
			}
		}

		err = deploy.grantAppAdminAccessToRuntimeSecrets(deployment.Namespace, registration, &component, secretsToManage)
		if err != nil {
			return fmt.Errorf("Failed to grant app admin access to own secrets. %v", err)
		}
	}
	return nil
}

func (deploy *Deployment) garbageCollectSecretsNoLongerInSpec() error {
	secrets, err := deploy.kubeclient.CoreV1().Secrets(deploy.radixDeployment.GetNamespace()).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, exisitingComponent := range secrets.Items {
		garbageCollect := true
		exisitingComponentName, exists := exisitingComponent.ObjectMeta.Labels[kube.RadixComponentLabel]

		if !exists {
			continue
		}

		for _, component := range deploy.radixDeployment.Spec.Components {
			if strings.EqualFold(component.Name, exisitingComponentName) {
				garbageCollect = false
				break
			}
		}

		if garbageCollect {
			err = deploy.kubeclient.CoreV1().Secrets(deploy.radixDeployment.GetNamespace()).Delete(exisitingComponent.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
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

	log.Debugf("Created container registry credentials secret: %s in namespace %s", saveDockerSecret.Name, ns)
	return nil
}

func (deploy *Deployment) createSecret(ns, app, component, secretName string) error {
	secret := v1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:       app,
				kube.RadixComponentLabel: component,
			},
		},
	}
	_, err := deploy.kubeutil.ApplySecret(ns, &secret)
	if err != nil {
		return err
	}

	return nil
}
