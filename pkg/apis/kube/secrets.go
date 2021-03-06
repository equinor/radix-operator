package kube

import (
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	labelHelpers "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// SecretExists Checks if secret allready exists
func (k *Kube) SecretExists(namespace, secretName string) bool {
	_, err := k.kubeClient.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
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
func (k *Kube) ApplySecret(namespace string, secret *corev1.Secret) (savedSecret *corev1.Secret, err error) {
	secretName := secret.GetName()
	log.Debugf("Applies secret %s in namespace %s", secretName, namespace)

	oldSecret, err := k.GetSecret(namespace, secretName)
	if err != nil && errors.IsNotFound(err) {
		savedSecret, err := k.kubeClient.CoreV1().Secrets(namespace).Create(secret)
		return savedSecret, err
	} else if err != nil {
		return nil, fmt.Errorf("Failed to get Secret object: %v", err)
	}

	oldSectetJSON, err := json.Marshal(oldSecret)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal old secret object: %v", err)
	}

	// Avvoid uneccessary patching
	newSecret := oldSecret.DeepCopy()
	newSecret.ObjectMeta.Labels = secret.ObjectMeta.Labels
	newSecret.ObjectMeta.Annotations = secret.ObjectMeta.Annotations
	newSecret.ObjectMeta.OwnerReferences = secret.ObjectMeta.OwnerReferences
	newSecret.Data = secret.Data

	print(newSecret.Data)

	newSecretJSON, err := json.Marshal(newSecret)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal new secret object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldSectetJSON, newSecretJSON, corev1.Secret{})
	if err != nil {
		return nil, fmt.Errorf("Failed to create two way merge patch secret objects: %v", err)
	}

	if !isEmptyPatch(patchBytes) {
		// Will perform update as patching not properly remove secret data entries
		patchedSecret, err := k.kubeClient.CoreV1().Secrets(namespace).Update(newSecret)
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
func (k *Kube) GetSecret(namespace, name string) (*corev1.Secret, error) {
	var secret *corev1.Secret
	var err error

	if k.SecretLister != nil {
		secret, err = k.SecretLister.Secrets(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		secret, err = k.kubeClient.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return secret, nil
}

// ListSecrets secrets in namespace
func (k *Kube) ListSecrets(namespace string) ([]*v1.Secret, error) {
	return k.ListSecretsWithSelector(namespace, nil)
}

// ListSecretsWithSelector secrets in namespace
func (k *Kube) ListSecretsWithSelector(namespace string, labelSelectorString *string) ([]*v1.Secret, error) {
	var secrets []*v1.Secret
	var err error

	if k.SecretLister != nil {
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

		secrets, err = k.SecretLister.Secrets(namespace).List(selector)
		if err != nil {
			return nil, err
		}
	} else {
		listOptions := metav1.ListOptions{}
		if labelSelectorString != nil {
			listOptions.LabelSelector = *labelSelectorString
		}

		list, err := k.kubeClient.CoreV1().Secrets(namespace).List(listOptions)
		if err != nil {
			return nil, err
		}

		secrets = slice.PointersOf(list.Items).([]*v1.Secret)
	}

	return secrets, nil
}

// DeleteSecret Deletes a secret in a namespace
func (k *Kube) DeleteSecret(namespace, secretName string) error {
	err := k.kubeClient.CoreV1().Secrets(namespace).Delete(secretName, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}
