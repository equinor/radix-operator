package deployment

import (
	"fmt"
	"strconv"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const secretDefaultData = "xx"

func (deploy *Deployment) createOrUpdateSecrets(registration *radixv1.RadixRegistration, deployment *radixv1.RadixDeployment) error {
	envName := deployment.Spec.Environment
	namespace := utils.GetEnvironmentNamespace(registration.Name, envName)

	log.Debugf("Apply empty secrets based on radix deployment obj")
	for _, comp := range deployment.Spec.Components {
		err := deploy.createOrUpdateSecretsForComponent(registration, deployment, &comp, namespace)
		if err != nil {
			return err
		}
	}
	for _, comp := range deployment.Spec.Jobs {
		err := deploy.createOrUpdateSecretsForComponent(registration, deployment, &comp, namespace)
		if err != nil {
			return err
		}
	}
	return nil
}

func (deploy *Deployment) createOrUpdateSecretsForComponent(registration *radixv1.RadixRegistration, deployment *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent, ns string) error {
	secretsToManage := make([]string, 0)

	if len(*component.GetSecrets()) > 0 {
		secretName := utils.GetComponentSecretName(component.GetName())
		if !deploy.kubeutil.SecretExists(ns, secretName) {
			err := deploy.createOrUpdateSecret(ns, registration.Name, component.GetName(), secretName, false)
			if err != nil {
				return err
			}
		} else {
			err := deploy.removeOrphanedSecrets(ns, registration.Name, component.GetName(), secretName, *component.GetSecrets())
			if err != nil {
				return err
			}
		}

		secretsToManage = append(secretsToManage, secretName)
	}

	dnsExternalAlias := component.GetDNSExternalAlias()
	if dnsExternalAlias != nil && len(*dnsExternalAlias) > 0 {
		err := deploy.garbageCollectSecretsNoLongerInSpecForComponentAndExternalAlias(component)
		if err != nil {
			return err
		}

		// Create secrets to hold TLS certificates
		for _, externalAlias := range *dnsExternalAlias {
			secretsToManage = append(secretsToManage, externalAlias)

			if deploy.kubeutil.SecretExists(ns, externalAlias) {
				continue
			}

			err := deploy.createOrUpdateSecret(ns, registration.Name, component.GetName(), externalAlias, true)
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

	if len(*component.GetVolumeMounts()) > 0 {
		for _, volumeMount := range *component.GetVolumeMounts() {
			if volumeMount.Type == radixv1.MountTypeBlob {
				secretName, accountKey, accountName := deploy.getBlobFuseCredsSecrets(ns, component.GetName(), volumeMount.Name)
				secretsToManage = append(secretsToManage, secretName)
				err := deploy.createOrUpdateVolumeMountsSecrets(ns, component.GetName(), secretName, accountName, accountKey)
				if err != nil {
					return err
				}
			}
		}
	} else {
		err := deploy.garbageCollectVolumeMountsSecretsNoLongerInSpecForComponent(component)
		if err != nil {
			return err
		}
	}

	err := deploy.grantAppAdminAccessToRuntimeSecrets(deployment.Namespace, registration, component, secretsToManage)
	if err != nil {
		return fmt.Errorf("Failed to grant app admin access to own secrets. %v", err)
	}

	if len(secretsToManage) == 0 {
		err := deploy.garbageCollectSecretsNoLongerInSpecForComponent(component)
		if err != nil {
			return err
		}
	}
	return nil
}

func (deploy *Deployment) getBlobFuseCredsSecrets(ns, componentName, volumeMountName string) (string, []byte, []byte) {
	secretName := defaults.GetBlobFuseCredsSecretName(componentName, volumeMountName)
	accountKey := []byte(secretDefaultData)
	accountName := []byte(secretDefaultData)
	if deploy.kubeutil.SecretExists(ns, secretName) {
		oldSecret, _ := deploy.kubeutil.GetSecret(ns, secretName)
		accountKey = oldSecret.Data[defaults.BlobFuseCredsAccountKeyPart]
		accountName = oldSecret.Data[defaults.BlobFuseCredsAccountNamePart]
	}
	return secretName, accountKey, accountName
}

func (deploy *Deployment) garbageCollectSecretsNoLongerInSpec() error {
	secrets, err := deploy.kubeutil.ListSecrets(deploy.radixDeployment.GetNamespace())
	if err != nil {
		return err
	}

	for _, exisitingSecret := range secrets {
		if exisitingSecret.ObjectMeta.Labels[kube.RadixExternalAliasLabel] != "" {
			// Not handled here
			continue
		}

		componentName, ok := NewRadixComponentNameFromLabels(exisitingSecret)
		if !ok {
			continue
		}

		// Secrets should be kept if switching a component to a job or vice versa
		if !componentName.ExistInDeploymentSpec(deploy.radixDeployment) {
			err = deploy.kubeclient.CoreV1().Secrets(deploy.radixDeployment.GetNamespace()).Delete(exisitingSecret.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectSecretsNoLongerInSpecForComponent(component radixv1.RadixCommonDeployComponent) error {
	secrets, err := deploy.listSecretsForComponent(component)
	if err != nil {
		return err
	}

	for _, secret := range secrets {
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

func (deploy *Deployment) garbageCollectAllSecretsForComponentAndExternalAlias(component radixv1.RadixCommonDeployComponent) error {
	return deploy.garbageCollectSecretsForComponentAndExternalAlias(component, true)
}

func (deploy *Deployment) garbageCollectSecretsNoLongerInSpecForComponentAndExternalAlias(component radixv1.RadixCommonDeployComponent) error {
	return deploy.garbageCollectSecretsForComponentAndExternalAlias(component, false)
}

func (deploy *Deployment) garbageCollectSecretsForComponentAndExternalAlias(component radixv1.RadixCommonDeployComponent, all bool) error {
	secrets, err := deploy.listSecretsForComponentExternalAlias(component)
	if err != nil {
		return err
	}

	for _, secret := range secrets {
		garbageCollectSecret := true

		dnsExternalAlias := component.GetDNSExternalAlias()
		if !all && dnsExternalAlias != nil {
			externalAliasForSecret := secret.Name
			for _, externalAlias := range *dnsExternalAlias {
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

func (deploy *Deployment) listSecretsForComponent(component radixv1.RadixCommonDeployComponent) ([]*v1.Secret, error) {
	return deploy.listSecrets(getLabelSelectorForComponent(component))
}

func (deploy *Deployment) listSecretsForComponentExternalAlias(component radixv1.RadixCommonDeployComponent) ([]*v1.Secret, error) {
	return deploy.listSecrets(getLabelSelectorForExternalAlias(component))
}

func (deploy *Deployment) listSecretsForForBlobVolumeMount(component radixv1.RadixCommonDeployComponent) ([]*v1.Secret, error) {
	return deploy.listSecrets(getLabelSelectorForBlobVolumeMountSecret(component))
}

func (deploy *Deployment) listSecrets(labelSelector string) ([]*v1.Secret, error) {
	secrets, err := deploy.kubeutil.ListSecretsWithSelector(deploy.radixDeployment.GetNamespace(), &labelSelector)

	if err != nil {
		return nil, err
	}

	return secrets, err
}

func (deploy *Deployment) createOrUpdateSecret(ns, app, component, secretName string, isExternalAlias bool) error {
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
		defaultValue := []byte(secretDefaultData)

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
	secret, err := deploy.kubeutil.GetSecret(ns, secretName)
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
		_, err = deploy.kubeutil.ApplySecret(ns, secret)
		if err != nil {
			return err
		}
	}

	return nil
}
