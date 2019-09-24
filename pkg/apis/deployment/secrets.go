package deployment

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const tlsSecretDefaultData = "xx"

func (deploy *Deployment) createSecrets(registration *radixv1.RadixRegistration, deployment *radixv1.RadixDeployment) error {
	envName := deployment.Spec.Environment
	ns := utils.GetEnvironmentNamespace(registration.Name, envName)

	err = deploy.createDockerSecret(registration, ns)
	if err != nil {
		return err
	}

	log.Debugf("Apply empty secrets based on radix deployment obj")
	for _, component := range deployment.Spec.Components {
		secretsToManage := make([]string, 0)

		if len(component.Secrets) > 0 {
			secretName := utils.GetComponentSecretName(component.Name)
			if !deploy.kubeutil.SecretExists(ns, secretName) {
				err := deploy.createSecret(ns, registration.Name, component.Name, secretName, false)
				if err != nil {
					return err
				}
			} else {
				err := deploy.removeOrphanedSecrets(ns, registration.Name, component.Name, secretName, component.Secrets)
				if err != nil {
					return err
				}
			}

			secretsToManage = append(secretsToManage, secretName)
		}

		if len(component.DNSExternalAlias) > 0 {
			err := deploy.garbageCollectSecretsNoLongerInSpecForComponentAndExternalAlias(component)
			if err != nil {
				return err
			}

			// Create secrets to hold TLS certificates
			for _, externalAlias := range component.DNSExternalAlias {
				secretsToManage = append(secretsToManage, externalAlias)

				if deploy.kubeutil.SecretExists(ns, externalAlias) {
					continue
				}

				err := deploy.createSecret(ns, registration.Name, component.Name, externalAlias, true)
				if err != nil {
					return err
				}
			}
		} else {
			err := deploy.garbageCollectAllSecretsForComponentAndExternalAlias(component)
			if err != nil {
				return err
			}
		}

		err = deploy.grantAppAdminAccessToRuntimeSecrets(deployment.Namespace, registration, &component, secretsToManage)
		if err != nil {
			return fmt.Errorf("Failed to grant app admin access to own secrets. %v", err)
		}

		if len(secretsToManage) == 0 {
			err := deploy.garbageCollectSecretsNoLongerInSpecForComponent(component)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (deploy *Deployment) garbageCollectSecretsNoLongerInSpec() error {
	secrets, err := deploy.kubeclient.CoreV1().Secrets(deploy.radixDeployment.GetNamespace()).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, exisitingSecret := range secrets.Items {
		if exisitingSecret.ObjectMeta.Labels[kube.RadixExternalAliasLabel] != "" {
			// Not handled here
			continue
		}

		garbageCollect := true
		exisitingComponentName, exists := exisitingSecret.ObjectMeta.Labels[kube.RadixComponentLabel]

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
			err = deploy.kubeclient.CoreV1().Secrets(deploy.radixDeployment.GetNamespace()).Delete(exisitingSecret.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectSecretsNoLongerInSpecForComponent(component radixv1.RadixDeployComponent) error {
	secrets, err := deploy.listSecretsForComponent(component)
	if err != nil {
		return err
	}

	for _, secret := range secrets.Items {
		if secret.ObjectMeta.Labels[kube.RadixExternalAliasLabel] != "" {
			// Not handled here
			continue
		}

		err = deploy.kubeclient.CoreV1().Secrets(deploy.radixDeployment.GetNamespace()).Delete(secret.Name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectAllSecretsForComponentAndExternalAlias(component radixv1.RadixDeployComponent) error {
	return deploy.garbageCollectSecretsForComponentAndExternalAlias(component, true)
}

func (deploy *Deployment) garbageCollectSecretsNoLongerInSpecForComponentAndExternalAlias(component radixv1.RadixDeployComponent) error {
	return deploy.garbageCollectSecretsForComponentAndExternalAlias(component, false)
}

func (deploy *Deployment) garbageCollectSecretsForComponentAndExternalAlias(component radixv1.RadixDeployComponent, all bool) error {
	secrets, err := deploy.listSecretsForComponentExternalAlias(component)
	if err != nil {
		return err
	}

	for _, secret := range secrets.Items {
		garbageCollectSecret := true

		if !all {
			externalAliasForSecret := secret.Name
			for _, externalAlias := range component.DNSExternalAlias {
				if externalAlias == externalAliasForSecret {
					garbageCollectSecret = false
				}
			}
		}

		if garbageCollectSecret {
			err = deploy.kubeclient.CoreV1().Secrets(deploy.radixDeployment.GetNamespace()).Delete(secret.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) listSecretsForComponent(component radixv1.RadixDeployComponent) (*v1.SecretList, error) {
	return deploy.listSecrets(getLabelSelectorForComponent(component))
}

func (deploy *Deployment) listSecretsForComponentExternalAlias(component radixv1.RadixDeployComponent) (*v1.SecretList, error) {
	return deploy.listSecrets(getLabelSelectorForExternalAlias(component))
}

func (deploy *Deployment) listSecrets(labelSelector string) (*v1.SecretList, error) {
	secrets, err := deploy.kubeclient.CoreV1().Secrets(deploy.radixDeployment.GetNamespace()).List(metav1.ListOptions{
		LabelSelector: labelSelector,
	})

	if err != nil {
		return nil, err
	}

	return secrets, err
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

func (deploy *Deployment) createSecret(ns, app, component, secretName string, isExternalAlias bool) error {
	secretType := v1.SecretType("Opaque")
	if isExternalAlias {
		secretType = v1.SecretType("kubernetes.io/tls")
	}

	secret := v1.Secret{
		Type: secretType,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:           app,
				kube.RadixComponentLabel:     component,
				kube.RadixExternalAliasLabel: strconv.FormatBool(isExternalAlias),
			},
		},
	}

	if isExternalAlias {
		defaultValue := []byte(tlsSecretDefaultData)

		// Will need to set fake data in order to apply the secret. The user then need to set data to real values
		data := make(map[string][]byte)
		data["tls.crt"] = defaultValue
		data["tls.key"] = defaultValue

		secret.Data = data
	}

	_, err := deploy.kubeutil.ApplySecret(ns, &secret)
	if err != nil {
		return err
	}

	return nil
}

func (deploy *Deployment) removeOrphanedSecrets(ns, app, component, secretName string, secrets []string) error {
	secret, err := deploy.kubeclient.CoreV1().Secrets(ns).Get(secretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	orphanRemoved := false
	for secretName := range secret.Data {
		if !slice.ContainsString(secrets, secretName) {
			delete(secret.Data, secretName)
			orphanRemoved = true
		}
	}

	if orphanRemoved {
		_, err = deploy.kubeclient.CoreV1().Secrets(ns).Update(secret)
		if err != nil {
			return err
		}
	}

	return nil
}
