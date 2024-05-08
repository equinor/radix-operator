package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// SecretExists Checks if secret already exists
func (kubeutil *Kube) SecretExists(ctx context.Context, namespace, secretName string) bool {
	_, err := kubeutil.kubeClient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		return false
	}
	if err != nil {
		// TODO: the error should be returned to and handled by caller
		log.Error().Err(err).Msgf("Failed to get secret %s in namespace %s", secretName, namespace)
		return false
	}
	return true
}

// ListSecretExistsForLabels Gets list of secrets for specific labels
func (kubeutil *Kube) ListSecretExistsForLabels(ctx context.Context, namespace string, labelSelector string) ([]corev1.Secret, error) {
	list, err := kubeutil.kubeClient.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ApplySecret Creates or updates secret to namespace
func (kubeutil *Kube) ApplySecret(ctx context.Context, namespace string, secret *corev1.Secret) (savedSecret *corev1.Secret, err error) {
	secretName := secret.GetName()
	// file deepcode ignore ClearTextLogging: logs name of secret only
	log.Debug().Msgf("Applies secret %s in namespace %s", secretName, namespace)

	oldSecret, err := kubeutil.GetSecret(ctx, namespace, secretName)
	if err != nil && errors.IsNotFound(err) {
		savedSecret, err := kubeutil.kubeClient.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}
		log.Info().Msgf("Created secret: %s in namespace %s", secret.GetName(), namespace)
		return savedSecret, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get Secret object: %w", err)
	}

	oldSecretJSON, err := json.Marshal(oldSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal old secret object: %w", err)
	}

	// Avoid unnecessary patching
	newSecret := oldSecret.DeepCopy()
	newSecret.ObjectMeta.Labels = secret.ObjectMeta.Labels
	newSecret.ObjectMeta.Annotations = secret.ObjectMeta.Annotations
	newSecret.ObjectMeta.OwnerReferences = secret.ObjectMeta.OwnerReferences
	newSecret.Data = secret.Data

	newSecretJSON, err := json.Marshal(newSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal new secret object: %w", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldSecretJSON, newSecretJSON, corev1.Secret{})
	if err != nil {
		return nil, fmt.Errorf("failed to create two way merge patch secret objects: %w", err)
	}

	if !IsEmptyPatch(patchBytes) {
		// Will perform update as patching not properly remove secret data entries
		patchedSecret, err := kubeutil.kubeClient.CoreV1().Secrets(namespace).Update(ctx, newSecret, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to update secret object: %w", err)
		}

		log.Info().Msgf("Updated secret: %s in namespace %s", patchedSecret.GetName(), namespace)
		return patchedSecret, nil

	}

	log.Debug().Msgf("No need to patch secret: %s ", secretName)
	return oldSecret, nil
}

// GetSecret Get secret from cache, if lister exist
func (kubeutil *Kube) GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	var secret *corev1.Secret
	var err error

	if kubeutil.SecretLister != nil {
		secret, err = kubeutil.SecretLister.Secrets(namespace).Get(name)
		secret = secret.DeepCopy() // Need to do a deep copy in case the caller modifies the returned secret
		if err != nil {
			return nil, err
		}
	} else {
		secret, err = kubeutil.kubeClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return secret, nil
}

// ListSecrets secrets in namespace
func (kubeutil *Kube) ListSecrets(ctx context.Context, namespace string) ([]*corev1.Secret, error) {
	return kubeutil.ListSecretsWithSelector(ctx, namespace, "")
}

// ListSecretsWithSelector secrets in namespace
func (kubeutil *Kube) ListSecretsWithSelector(ctx context.Context, namespace string, labelSelectorString string) ([]*corev1.Secret, error) {
	var secrets []*corev1.Secret

	if kubeutil.SecretLister != nil {
		selector, err := labels.Parse(labelSelectorString)
		if err != nil {
			return nil, err
		}

		secrets, err = kubeutil.SecretLister.Secrets(namespace).List(selector)
		if err != nil {
			return nil, err
		}
	} else {
		listOptions := metav1.ListOptions{
			LabelSelector: labelSelectorString,
		}

		list, err := kubeutil.kubeClient.CoreV1().Secrets(namespace).List(ctx, listOptions)
		if err != nil {
			return nil, err
		}

		secrets = slice.PointersOf(list.Items).([]*corev1.Secret)
	}

	return secrets, nil
}

// DeleteSecret Deletes a secret in a namespace
func (kubeutil *Kube) DeleteSecret(ctx context.Context, namespace, secretName string) error {
	err := kubeutil.kubeClient.CoreV1().Secrets(namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	log.Info().Msgf("Deleted secret: %s in namespace %s", secretName, namespace)
	return nil
}

// GetSecretTypeForRadixAzureKeyVault Gets corev1.SecretType by RadixAzureKeyVaultK8sSecretType
func GetSecretTypeForRadixAzureKeyVault(k8sSecretType *radixv1.RadixAzureKeyVaultK8sSecretType) corev1.SecretType {
	if k8sSecretType != nil && *k8sSecretType == radixv1.RadixAzureKeyVaultK8sSecretTypeTls {
		return corev1.SecretTypeTLS
	}
	return corev1.SecretTypeOpaque
}

// GetAzureKeyVaultSecretRefSecretName Gets a secret name for Azure KeyVault RadixSecretRefs
func GetAzureKeyVaultSecretRefSecretName(componentName, radixDeploymentName, azKeyVaultName string, secretType corev1.SecretType) string {
	radixSecretRefSecretType := string(getK8sSecretTypeRadixAzureKeyVaultK8sSecretType(secretType))
	return getSecretRefSecretName(componentName, radixDeploymentName,
		string(radixv1.RadixSecretRefTypeAzureKeyVault), radixSecretRefSecretType, strings.ToLower(azKeyVaultName))
}

func getSecretRefSecretName(componentName, radixDeploymentName, secretRefType, secretType, secretResourceName string) string {
	hash := strings.ToLower(utils.RandStringStrSeed(5, strings.ToLower(fmt.Sprintf("%s-%s-%s-%s", componentName,
		radixDeploymentName, secretRefType, secretResourceName))))
	return strings.ToLower(fmt.Sprintf("%s-%s-%s-%s-%s", componentName, secretRefType, secretType,
		secretResourceName, hash))
}

func getK8sSecretTypeRadixAzureKeyVaultK8sSecretType(k8sSecretType corev1.SecretType) radixv1.RadixAzureKeyVaultK8sSecretType {
	if k8sSecretType == corev1.SecretTypeTLS {
		return radixv1.RadixAzureKeyVaultK8sSecretTypeTls
	}
	return radixv1.RadixAzureKeyVaultK8sSecretTypeOpaque
}
