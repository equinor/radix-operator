package kube

import (
	"context"
	"encoding/json"
	"fmt"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	labelHelpers "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
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

// ApplySecret Creates or updates secret to namespace
func (kubeutil *Kube) ApplySecret(namespace string, secret *corev1.Secret) (savedSecret *corev1.Secret, err error) {
	secretName := secret.GetName()
	log.Debugf("Applies secret %s in namespace %s", secretName, namespace)

	oldSecret, err := kubeutil.GetSecret(namespace, secretName)
	if err != nil && errors.IsNotFound(err) {
		savedSecret, err := kubeutil.kubeClient.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		return savedSecret, err
	} else if err != nil {
		return nil, fmt.Errorf("Failed to get Secret object: %v", err)
	}

	oldSectetJSON, err := json.Marshal(oldSecret)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal old secret object: %v", err)
	}

	// Avoid uneccessary patching
	newSecret := oldSecret.DeepCopy()
	newSecret.ObjectMeta.Labels = secret.ObjectMeta.Labels
	newSecret.ObjectMeta.Annotations = secret.ObjectMeta.Annotations
	newSecret.ObjectMeta.OwnerReferences = secret.ObjectMeta.OwnerReferences
	newSecret.Data = secret.Data

	newSecretJSON, err := json.Marshal(newSecret)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal new secret object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldSectetJSON, newSecretJSON, corev1.Secret{})
	if err != nil {
		return nil, fmt.Errorf("Failed to create two way merge patch secret objects: %v", err)
	}

	if !IsEmptyPatch(patchBytes) {
		// Will perform update as patching not properly remove secret data entries
		patchedSecret, err := kubeutil.kubeClient.CoreV1().Secrets(namespace).Update(context.TODO(), newSecret, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("Failed to update secret object: %v", err)
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
func (kubeutil *Kube) ListSecrets(namespace string) ([]*v1.Secret, error) {
	return kubeutil.ListSecretsWithSelector(namespace, nil)
}

// ListSecretsWithSelector secrets in namespace
func (kubeutil *Kube) ListSecretsWithSelector(namespace string, labelSelectorString *string) ([]*v1.Secret, error) {
	var secrets []*v1.Secret
	var err error

	if kubeutil.SecretLister != nil {
		var selector labels.Selector
		if labelSelectorString != nil {
			labelSelector, err := labelHelpers.ParseToLabelSelector(*labelSelectorString)
			if err != nil {
				return nil, err
			}

			selector, err = labelHelpers.LabelSelectorAsSelector(labelSelector)
			if err != nil {
				return nil, err
			}

		} else {
			selector = labels.NewSelector()
		}

		secrets, err = kubeutil.SecretLister.Secrets(namespace).List(selector)
		if err != nil {
			return nil, err
		}
	} else {
		listOptions := metav1.ListOptions{}
		if labelSelectorString != nil {
			listOptions.LabelSelector = *labelSelectorString
		}

		list, err := kubeutil.kubeClient.CoreV1().Secrets(namespace).List(context.TODO(), listOptions)
		if err != nil {
			return nil, err
		}

		secrets = slice.PointersOf(list.Items).([]*v1.Secret)
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

// DeleteChangedSecretProviderClass Deletes a role in a namespace
func (kubeutil *Kube) DeleteChangedSecretProviderClass(namespace string, componentName string, radixKeyVault radixv1.RadixKeyVault) error {
	scName := utils.GetComponentKeyVaultSecretProviderClassName(componentName, radixKeyVault.Name)
	class := secretsstorev1.SecretProviderClass{}
	//_, err := kubeutil.kubeClient.CoreV1().Namespaces(namespace).Delete( GetKV)
	if err != nil && errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("Failed to get role object: %v", err)
	}
	err = kubeutil.kubeClient.RbacV1().Roles(namespace).Delete(context.TODO(), sc, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("Failed to delete role object: %v", err)
	}
	return nil
}
