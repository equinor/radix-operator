package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

type SecretType string

const (
	SecretTypeOpaque SecretType = "Opaque"
	SecretTypeTls    SecretType = "kubernetes.io/tls"
)

// SecretExists Checks if secret already exists
func (kubeutil *Kube) SecretExists(namespace, secretName string) bool {
	_, err := kubeutil.kubeClient.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		return false
	}
	if err != nil {
		log.Errorf("Failed to get secret %s in namespace %s. %v", secretName, namespace, err)
		return false
	}
	return true
}

// ListSecretExistsForLabels Gets list of secrets for specific labels
func (kubeutil *Kube) ListSecretExistsForLabels(namespace string, labelSelector string) ([]corev1.Secret, error) {
	list, err := kubeutil.kubeClient.CoreV1().Secrets(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		log.Errorf("failed to get secret in namespace %s. %v", namespace, err)
		return nil, err
	}
	return list.Items, nil
}

// ApplySecret Creates or updates secret to namespace
func (kubeutil *Kube) ApplySecret(namespace string, secret *corev1.Secret) (savedSecret *corev1.Secret, err error) {
	secretName := secret.GetName()
	log.Debugf("Applies secret %s in namespace %s", secretName, namespace)

	oldSecret, err := kubeutil.GetSecret(namespace, secretName)
	if err != nil && errors.IsNotFound(err) {
		savedSecret, err := kubeutil.kubeClient.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		return savedSecret, err
	} else if err != nil {
		return nil, fmt.Errorf("failed to get Secret object: %v", err)
	}

	oldSecretJSON, err := json.Marshal(oldSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal old secret object: %v", err)
	}

	// Avoid unnecessary patching
	newSecret := oldSecret.DeepCopy()
	newSecret.ObjectMeta.Labels = secret.ObjectMeta.Labels
	newSecret.ObjectMeta.Annotations = secret.ObjectMeta.Annotations
	newSecret.ObjectMeta.OwnerReferences = secret.ObjectMeta.OwnerReferences
	newSecret.Data = secret.Data

	newSecretJSON, err := json.Marshal(newSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal new secret object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldSecretJSON, newSecretJSON, corev1.Secret{})
	if err != nil {
		return nil, fmt.Errorf("failed to create two way merge patch secret objects: %v", err)
	}

	if !IsEmptyPatch(patchBytes) {
		// Will perform update as patching not properly remove secret data entries
		patchedSecret, err := kubeutil.kubeClient.CoreV1().Secrets(namespace).Update(context.TODO(), newSecret, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to update secret object: %v", err)
		}

		log.Debugf("Updated secret: %s ", patchedSecret.Name)
		return patchedSecret, nil

	}

	log.Debugf("No need to patch secret: %s ", secretName)
	return oldSecret, nil
}

// GetSecret Get secret from cache, if lister exist
func (kubeutil *Kube) GetSecret(namespace, name string) (*corev1.Secret, error) {
	var secret *corev1.Secret
	var err error

	if kubeutil.SecretLister != nil {
		secret, err = kubeutil.SecretLister.Secrets(namespace).Get(name)
		secret = secret.DeepCopy() // Need to do a deep copy in case the caller modifies the returned secret
		if err != nil {
			return nil, err
		}
	} else {
		secret, err = kubeutil.kubeClient.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return secret, nil
}

// ListSecrets secrets in namespace
func (kubeutil *Kube) ListSecrets(namespace string) ([]*corev1.Secret, error) {
	return kubeutil.ListSecretsWithSelector(namespace, "")
}

// ListSecretsWithSelector secrets in namespace
func (kubeutil *Kube) ListSecretsWithSelector(namespace string, labelSelectorString string) ([]*corev1.Secret, error) {
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

		list, err := kubeutil.kubeClient.CoreV1().Secrets(namespace).List(context.TODO(), listOptions)
		if err != nil {
			return nil, err
		}

		secrets = slice.PointersOf(list.Items).([]*corev1.Secret)
	}

	return secrets, nil
}

// DeleteSecret Deletes a secret in a namespace
func (kubeutil *Kube) DeleteSecret(namespace, secretName string) error {
	err := kubeutil.kubeClient.CoreV1().Secrets(namespace).Delete(context.TODO(), secretName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

// GetSecretTypeForRadixAzureKeyVault Gets SecretType by RadixAzureKeyVaultK8sSecretType
func GetSecretTypeForRadixAzureKeyVault(k8sSecretType *radixv1.RadixAzureKeyVaultK8sSecretType) SecretType {
	if k8sSecretType != nil && *k8sSecretType == radixv1.RadixAzureKeyVaultK8sSecretTypeTls {
		return SecretTypeTls
	}
	return SecretTypeOpaque
}

// GetAzureKeyVaultSecretRefSecretName Gets a secret name for Azure KeyVault RadixSecretRefs
func GetAzureKeyVaultSecretRefSecretName(componentName, radixDeploymentName, azKeyVaultName string, secretType SecretType) string {
	radixSecretRefSecretType := string(getK8sSecretTypeRadixAzureKeyVaultK8sSecretType(secretType))
	return getSecretRefSecretName(componentName, radixDeploymentName, string(radixv1.RadixSecretRefTypeAzureKeyVault), radixSecretRefSecretType, azKeyVaultName)
}

func getSecretRefSecretName(componentName, radixDeploymentName, secretRefType, secretType, secretResourceName string) string {
	hash := strings.ToLower(utils.RandStringStrSeed(5, fmt.Sprintf("%s-%s-%s-%s", componentName, radixDeploymentName, secretRefType, secretResourceName)))
	return fmt.Sprintf("%s-%s-%s-%s-%s", componentName, secretRefType, secretType, secretResourceName, hash)
}

func getK8sSecretTypeRadixAzureKeyVaultK8sSecretType(k8sSecretType SecretType) radixv1.RadixAzureKeyVaultK8sSecretType {
	if k8sSecretType == SecretTypeTls {
		return radixv1.RadixAzureKeyVaultK8sSecretTypeTls
	}
	return radixv1.RadixAzureKeyVaultK8sSecretTypeOpaque
}
