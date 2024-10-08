package deployment

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	secretDefaultData                  = "xx"
	secretUsedBySecretStoreDriverLabel = "secrets-store.csi.k8s.io/used"
)

func tlsSecretDefaultData() map[string][]byte {
	return map[string][]byte{
		v1.TLSCertKey:       nil,
		v1.TLSPrivateKeyKey: nil,
	}
}

func (deploy *Deployment) createOrUpdateSecrets(ctx context.Context) error {
	log.Ctx(ctx).Debug().Msg("Apply empty secrets based on radix deployment obj")
	for _, comp := range deploy.radixDeployment.Spec.Components {
		err := deploy.createOrUpdateSecretsForComponent(ctx, &comp)
		if err != nil {
			return err
		}
	}
	for _, comp := range deploy.radixDeployment.Spec.Jobs {
		err := deploy.createOrUpdateSecretsForComponent(ctx, &comp)
		if err != nil {
			return err
		}
	}
	return nil
}

func (deploy *Deployment) createOrUpdateSecretsForComponent(ctx context.Context, component radixv1.RadixCommonDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	secretsToManage := make([]string, 0)

	if len(component.GetSecrets()) > 0 {
		secretName := utils.GetComponentSecretName(component.GetName())
		if !deploy.kubeutil.SecretExists(ctx, namespace, secretName) {
			err := deploy.createOrUpdateComponentSecret(ctx, namespace, deploy.registration.Name, component.GetName(), secretName)
			if err != nil {
				return err
			}
		} else {
			err := deploy.removeOrphanedSecrets(ctx, namespace, secretName, component.GetSecrets())
			if err != nil {
				return err
			}
		}

		secretsToManage = append(secretsToManage, secretName)
	}

	volumeMountSecretsToManage, err := deploy.createOrUpdateVolumeMountSecrets(ctx, namespace, component.GetName(), component.GetVolumeMounts())
	if err != nil {
		return err
	}
	secretsToManage = append(secretsToManage, volumeMountSecretsToManage...)

	err = deploy.garbageCollectVolumeMountsSecretsNoLongerInSpecForComponent(ctx, component, secretsToManage)
	if err != nil {
		return err
	}

	clientCertificateSecretName := utils.GetComponentClientCertificateSecretName(component.GetName())
	if auth := component.GetAuthentication(); auth != nil && component.IsPublic() && ingress.IsSecretRequiredForClientCertificate(auth.ClientCertificate) {
		if !deploy.kubeutil.SecretExists(ctx, namespace, clientCertificateSecretName) {
			if err := deploy.createClientCertificateSecret(ctx, namespace, deploy.registration.Name, component.GetName(), clientCertificateSecretName); err != nil {
				return err
			}
		}
		secretsToManage = append(secretsToManage, clientCertificateSecretName)
	} else if deploy.kubeutil.SecretExists(ctx, namespace, clientCertificateSecretName) {
		err := deploy.kubeutil.DeleteSecret(ctx, namespace, clientCertificateSecretName)
		if err != nil {
			return err
		}
	}

	secretRefsSecretNames, err := deploy.createSecretRefs(ctx, namespace, component)
	if err != nil {
		return err
	}
	secretsToManage = append(secretsToManage, secretRefsSecretNames...)

	err = deploy.grantAccessToComponentRuntimeSecrets(ctx, component, secretsToManage)
	if err != nil {
		return fmt.Errorf("failed to grant access to secrets. %v", err)
	}

	if len(secretsToManage) == 0 {
		err := deploy.garbageCollectSecretsNoLongerInSpecForComponent(ctx, component)
		if err != nil {
			return err
		}
	}
	return nil
}

func (deploy *Deployment) createOrUpdateVolumeMountSecrets(ctx context.Context, namespace, componentName string, volumeMounts []radixv1.RadixVolumeMount) ([]string, error) {
	var volumeMountSecretsToManage []string
	for _, volumeMount := range volumeMounts {
		switch GetCsiAzureVolumeMountType(&volumeMount) {
		case radixv1.MountTypeBlob:
			{
				secretName, accountKey, accountName := deploy.getBlobFuseCredsSecrets(ctx, namespace, componentName, volumeMount.Name)
				volumeMountSecretsToManage = append(volumeMountSecretsToManage, secretName)
				err := deploy.createOrUpdateVolumeMountsSecrets(ctx, namespace, componentName, secretName, accountName, accountKey)
				if err != nil {
					return nil, err
				}
			}
		case radixv1.MountTypeBlobFuse2FuseCsiAzure, radixv1.MountTypeBlobFuse2Fuse2CsiAzure, radixv1.MountTypeBlobFuse2NfsCsiAzure, radixv1.MountTypeAzureFileCsiAzure:
			{
				secretName, accountKey, accountName := deploy.getCsiAzureVolumeMountCredsSecrets(ctx, namespace, componentName, volumeMount.Name)
				volumeMountSecretsToManage = append(volumeMountSecretsToManage, secretName)
				err := deploy.createOrUpdateCsiAzureVolumeMountsSecrets(ctx, namespace, componentName, &volumeMount, secretName, accountName, accountKey)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return volumeMountSecretsToManage, nil
}

func (deploy *Deployment) getBlobFuseCredsSecrets(ctx context.Context, ns, componentName, volumeMountName string) (string, []byte, []byte) {
	secretName := defaults.GetBlobFuseCredsSecretName(componentName, volumeMountName)
	accountKey := []byte(secretDefaultData)
	accountName := []byte(secretDefaultData)
	if deploy.kubeutil.SecretExists(ctx, ns, secretName) {
		oldSecret, _ := deploy.kubeutil.GetSecret(ctx, ns, secretName)
		accountKey = oldSecret.Data[defaults.BlobFuseCredsAccountKeyPart]
		accountName = oldSecret.Data[defaults.BlobFuseCredsAccountNamePart]
	}
	return secretName, accountKey, accountName
}

func (deploy *Deployment) getCsiAzureVolumeMountCredsSecrets(ctx context.Context, namespace, componentName, volumeMountName string) (string, []byte, []byte) {
	secretName := defaults.GetCsiAzureVolumeMountCredsSecretName(componentName, volumeMountName)
	accountKey := []byte(secretDefaultData)
	accountName := []byte(secretDefaultData)
	if deploy.kubeutil.SecretExists(ctx, namespace, secretName) {
		oldSecret, _ := deploy.kubeutil.GetSecret(ctx, namespace, secretName)
		accountKey = oldSecret.Data[defaults.CsiAzureCredsAccountKeyPart]
		accountName = oldSecret.Data[defaults.CsiAzureCredsAccountNamePart]
	}
	return secretName, accountKey, accountName
}

func (deploy *Deployment) garbageCollectSecretsNoLongerInSpec(ctx context.Context) error {
	secrets, err := deploy.kubeutil.ListSecrets(ctx, deploy.radixDeployment.GetNamespace())
	if err != nil {
		return err
	}

	for _, existingSecret := range secrets {
		componentName, ok := RadixComponentNameFromComponentLabel(existingSecret)
		if !ok {
			continue
		}

		if deploy.isEligibleForGarbageCollectSecretsForComponent(existingSecret, componentName) {
			err := deploy.deleteSecret(ctx, existingSecret)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) isEligibleForGarbageCollectSecretsForComponent(existingSecret *v1.Secret, componentName RadixComponentName) bool {
	// Garbage collect if secret is labelled radix-job-type=job-scheduler and not defined in RD jobs
	if jobType, ok := NewRadixJobTypeFromObjectLabels(existingSecret); ok && jobType.IsJobScheduler() {
		return !componentName.ExistInDeploymentSpecJobList(deploy.radixDeployment)
	}
	// Garbage collect secret if not defined in RD components or jobs
	return !componentName.ExistInDeploymentSpec(deploy.radixDeployment)
}

func (deploy *Deployment) garbageCollectSecretsNoLongerInSpecForComponent(ctx context.Context, component radixv1.RadixCommonDeployComponent) error {
	secrets, err := deploy.listSecretsForComponent(ctx, component)
	if err != nil {
		return err
	}

	for _, secret := range secrets {
		// External alias not handled here
		if secret.ObjectMeta.Labels[kube.RadixExternalAliasLabel] == "true" {
			continue
		}

		// Secrets for jobs not handled here
		if _, ok := NewRadixJobTypeFromObjectLabels(secret); ok {
			continue
		}

		log.Ctx(ctx).Debug().Msgf("Delete secret %s no longer in spec for component %s", secret.Name, component.GetName())
		err = deploy.deleteSecret(ctx, secret)
		if err != nil {
			return err
		}
	}

	return nil
}

func (deploy *Deployment) listSecretsForComponent(ctx context.Context, component radixv1.RadixCommonDeployComponent) ([]*v1.Secret, error) {
	return deploy.listSecrets(ctx, getLabelSelectorForComponent(component))
}

func (deploy *Deployment) listSecretsForVolumeMounts(ctx context.Context, component radixv1.RadixCommonDeployComponent) ([]*v1.Secret, error) {
	blobVolumeMountSecret := getLabelSelectorForBlobVolumeMountSecret(component)
	secrets, err := deploy.listSecrets(ctx, blobVolumeMountSecret)
	if err != nil {
		return nil, err
	}
	csiAzureVolumeMountSecret := getLabelSelectorForCsiAzureVolumeMountSecret(component)
	csiSecrets, err := deploy.listSecrets(ctx, csiAzureVolumeMountSecret)
	if err != nil {
		return nil, err
	}
	secrets = append(secrets, csiSecrets...)
	return secrets, err
}

func (deploy *Deployment) listSecrets(ctx context.Context, labelSelector string) ([]*v1.Secret, error) {
	secrets, err := deploy.kubeutil.ListSecretsWithSelector(ctx, deploy.radixDeployment.GetNamespace(), labelSelector)

	if err != nil {
		return nil, err
	}

	return secrets, err
}

func (deploy *Deployment) createOrUpdateComponentSecret(ctx context.Context, ns, app, component, secretName string) error {

	secret := v1.Secret{
		Type: v1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:           app,
				kube.RadixComponentLabel:     component,
				kube.RadixExternalAliasLabel: "false",
			},
		},
	}

	existingSecret, err := deploy.kubeclient.CoreV1().Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
	if err == nil {
		secret.Data = existingSecret.Data
	} else if !errors.IsNotFound(err) {
		return err
	}

	_, err = deploy.kubeutil.ApplySecret(ctx, ns, &secret) //nolint:staticcheck // must be updated to use UpdateSecret or CreateSecret
	if err != nil {
		return err
	}

	return nil
}

func buildAzureKeyVaultCredentialsSecret(appName, componentName, secretName, azKeyVaultName string) *v1.Secret {
	secretType := v1.SecretTypeOpaque
	secret := v1.Secret{
		Type: secretType,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:                 appName,
				kube.RadixComponentLabel:           componentName,
				kube.RadixSecretRefTypeLabel:       string(radixv1.RadixSecretRefTypeAzureKeyVault),
				kube.RadixSecretRefNameLabel:       strings.ToLower(azKeyVaultName),
				secretUsedBySecretStoreDriverLabel: "true", // used by CSI Azure Key vault secret store driver for secret rotation
			},
		},
	}

	data := make(map[string][]byte)
	defaultValue := []byte(secretDefaultData)
	data["clientid"] = defaultValue
	data["clientsecret"] = defaultValue

	secret.Data = data
	return &secret
}

func (deploy *Deployment) createClientCertificateSecret(ctx context.Context, ns, app, component, secretName string) error {
	secret := v1.Secret{
		Type: v1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:       app,
				kube.RadixComponentLabel: component,
			},
		},
	}

	defaultValue := []byte(secretDefaultData)

	// Will need to set fake data in order to apply the secret. The user then need to set data to real values
	data := make(map[string][]byte)
	data["ca.crt"] = defaultValue
	secret.Data = data

	_, err := deploy.kubeutil.ApplySecret(ctx, ns, &secret) //nolint:staticcheck // must be updated to use UpdateSecret or CreateSecret
	return err
}

func (deploy *Deployment) removeOrphanedSecrets(ctx context.Context, ns, secretName string, secrets []string) error {
	secret, err := deploy.kubeutil.GetSecret(ctx, ns, secretName)
	if err != nil {
		return err
	}

	orphanRemoved := false
	for secretName := range secret.Data {

		if !slice.Any(secrets, func(s string) bool { return s == secretName }) {
			delete(secret.Data, secretName)
			orphanRemoved = true
		}
	}

	if orphanRemoved {
		_, err = deploy.kubeutil.ApplySecret(ctx, ns, secret) //nolint:staticcheck // must be updated to use UpdateSecret or CreateSecret
		if err != nil {
			return err
		}
	}

	return nil
}

// GarbageCollectSecrets delete secrets, excluding with names in the excludeSecretNames
func (deploy *Deployment) GarbageCollectSecrets(ctx context.Context, secrets []*v1.Secret, excludeSecretNames []string) error {
	for _, secret := range secrets {
		if slice.Any(excludeSecretNames, func(s string) bool { return s == secret.Name }) {
			continue
		}
		err := deploy.deleteSecret(ctx, secret)
		if err != nil {
			return err
		}
	}
	return nil
}

func (deploy *Deployment) deleteSecret(ctx context.Context, secret *v1.Secret) error {
	log.Ctx(ctx).Debug().Msgf("Delete secret %s", secret.Name)
	err := deploy.kubeclient.CoreV1().Secrets(deploy.radixDeployment.GetNamespace()).Delete(ctx, secret.Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	log.Ctx(ctx).Info().Msgf("Deleted secret: %s in namespace %s", secret.GetName(), deploy.radixDeployment.GetNamespace())
	return nil
}
