package deployment

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/volumemount"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) syncSecrets(ctx context.Context) error {
	log.Ctx(ctx).Debug().Msg("Apply empty secrets based on radix deployment obj")
	for _, comp := range deploy.radixDeployment.Spec.Components {
		err := deploy.syncComponentSecrets(ctx, &comp)
		if err != nil {
			return fmt.Errorf("failed to sync secrets for component %s: %w", comp.GetName(), err)
		}
	}
	for _, comp := range deploy.radixDeployment.Spec.Jobs {
		err := deploy.syncComponentSecrets(ctx, &comp)
		if err != nil {
			return fmt.Errorf("failed to sync secrets for job %s: %w", comp.GetName(), err)
		}
	}
	return nil
}

func (deploy *Deployment) syncComponentSecrets(ctx context.Context, component radixv1.RadixCommonDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	secretsToManage := make([]string, 0)

	if len(component.GetSecrets()) > 0 {
		secretName := utils.GetComponentSecretName(component.GetName())
		err := deploy.createOrUpdateComponentSecret(ctx, component, secretName)
		if err != nil {
			return err
		}
		secretsToManage = append(secretsToManage, secretName)
	}

	volumeMountSecretsToManage, err := volumemount.CreateOrUpdateVolumeMountSecrets(ctx, deploy.kubeutil, deploy.registration.Name, namespace, component)
	if err != nil {
		return err
	}
	secretsToManage = append(secretsToManage, volumeMountSecretsToManage...)

	err = volumemount.GarbageCollectVolumeMountsSecretsNoLongerInSpecForComponent(ctx, deploy.kubeutil, namespace, component, secretsToManage)
	if err != nil {
		return err
	}

	clientCertificateSecretName := utils.GetComponentClientCertificateSecretName(component.GetName())
	if auth := component.GetAuthentication(); auth != nil && component.IsPublic() && ingress.IsSecretRequiredForClientCertificate(auth.ClientCertificate) {
		if err := deploy.createOrUpdateClientCertificateSecret(ctx, component.GetName(), clientCertificateSecretName); err != nil {
			return err
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
			if err := deploy.kubeutil.DeleteSecret(ctx, deploy.radixDeployment.GetNamespace(), existingSecret.Name); err != nil && !kubeerrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) isEligibleForGarbageCollectSecretsForComponent(existingSecret *corev1.Secret, componentName RadixComponentName) bool {
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
		if err = deploy.kubeutil.DeleteSecret(ctx, deploy.radixDeployment.GetNamespace(), secret.Name); err != nil && !kubeerrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (deploy *Deployment) listSecretsForComponent(ctx context.Context, component radixv1.RadixCommonDeployComponent) ([]*corev1.Secret, error) {
	return listSecrets(ctx, deploy.kubeutil, deploy.radixDeployment.GetNamespace(), getLabelSelectorForComponent(component))
}

func listSecrets(ctx context.Context, kubeUtil *kube.Kube, namespace, labelSelector string) ([]*corev1.Secret, error) {
	secrets, err := kubeUtil.ListSecretsWithSelector(ctx, namespace, labelSelector)

	if err != nil {
		return nil, err
	}

	return secrets, err
}

func (deploy *Deployment) createOrUpdateComponentSecret(ctx context.Context, component radixv1.RadixCommonDeployComponent, secretName string) error {
	ns := deploy.radixDeployment.Namespace
	var currentSecret, desiredSecret *corev1.Secret
	currentSecret, err := deploy.kubeclient.CoreV1().Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if !kubeerrors.IsNotFound(err) {
			return fmt.Errorf("failed to get current secret: %w", err)
		}
		desiredSecret = &corev1.Secret{
			Type: corev1.SecretTypeOpaque,
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: ns,
			},
		}
		currentSecret = nil
	} else {
		desiredSecret = currentSecret.DeepCopy()
	}

	desiredSecret.Labels = map[string]string{
		kube.RadixAppLabel:           deploy.registration.Name,
		kube.RadixComponentLabel:     component.GetName(),
		kube.RadixExternalAliasLabel: "false",
	}

	// Remove orphaned secret keys
	for secretKey := range desiredSecret.Data {
		if !slices.Contains(component.GetSecrets(), secretKey) {
			delete(desiredSecret.Data, secretKey)
		}
	}

	if currentSecret == nil {
		if _, err := deploy.kubeutil.CreateSecret(ctx, ns, desiredSecret); err != nil {
			return fmt.Errorf("failed to create secret: %w", err)
		}
		return nil
	}

	if _, err := deploy.kubeutil.UpdateSecret(ctx, currentSecret, desiredSecret); err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}

	return nil
}

func buildAzureKeyVaultCredentialsSecret(appName, componentName, secretName, azKeyVaultName string) *corev1.Secret {
	secretType := corev1.SecretTypeOpaque
	secret := corev1.Secret{
		Type: secretType,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:                          appName,
				kube.RadixComponentLabel:                    componentName,
				kube.RadixSecretRefTypeLabel:                string(radixv1.RadixSecretRefTypeAzureKeyVault),
				kube.RadixSecretRefNameLabel:                strings.ToLower(azKeyVaultName),
				defaults.SecretUsedBySecretStoreDriverLabel: "true", // used by CSI Azure Key vault secret store driver for secret rotation
			},
		},
	}

	data := make(map[string][]byte)
	defaultValue := []byte(defaults.SecretDefaultData)
	data["clientid"] = defaultValue
	data["clientsecret"] = defaultValue

	secret.Data = data
	return &secret
}

func (deploy *Deployment) createOrUpdateClientCertificateSecret(ctx context.Context, componentName, secretName string) error {
	ns := deploy.radixDeployment.Namespace
	var currentSecret, desiredSecret *corev1.Secret
	currentSecret, err := deploy.kubeclient.CoreV1().Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if !kubeerrors.IsNotFound(err) {
			return fmt.Errorf("failed to get current client certificate secret: %w", err)
		}
		desiredSecret = &corev1.Secret{
			Type: corev1.SecretTypeOpaque,
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: ns,
			},
			Data: map[string][]byte{
				"ca.crt": nil,
			},
		}
		currentSecret = nil
	} else {
		desiredSecret = currentSecret.DeepCopy()
	}

	desiredSecret.Labels = map[string]string{
		kube.RadixAppLabel:       deploy.registration.Name,
		kube.RadixComponentLabel: componentName,
	}

	if currentSecret == nil {
		if _, err := deploy.kubeutil.CreateSecret(ctx, ns, desiredSecret); err != nil {
			return fmt.Errorf("failed to create volume mount secret: %w", err)
		}
		return nil
	}

	if _, err := deploy.kubeutil.UpdateSecret(ctx, currentSecret, desiredSecret); err != nil {
		return fmt.Errorf("failed to update volume mount DNS secret: %w", err)
	}

	return nil
}
