package kube

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// CreateServiceAccount create a service account
func (kubeutil *Kube) CreateServiceAccount(namespace, name string) (*corev1.ServiceAccount, error) {
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	return kubeutil.ApplyServiceAccount(&serviceAccount)
}

// ApplyServiceAccount Creates or updates service account
func (kubeutil *Kube) ApplyServiceAccount(serviceAccount *corev1.ServiceAccount) (*corev1.ServiceAccount, error) {
	oldServiceAccount, err := kubeutil.GetServiceAccount(serviceAccount.Namespace, serviceAccount.GetName())
	if err != nil && errors.IsNotFound(err) {
		createdServiceAccount, err := kubeutil.kubeClient.CoreV1().ServiceAccounts(serviceAccount.GetNamespace()).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create ServiceAccount object: %w", err)
		}
		log.Debugf("Created ServiceAccount %s in namespace %s", createdServiceAccount.GetName(), createdServiceAccount.GetNamespace())
		return createdServiceAccount, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get ServiceAccount object: %w", err)
	}

	log.Debugf("ServiceAccount object %s already exists in namespace %s, updating the object now", serviceAccount.GetName(), serviceAccount.GetNamespace())
	oldServiceAccountJson, err := json.Marshal(oldServiceAccount)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal old ServiceAccount object: %w", err)
	}

	newServiceAccount := oldServiceAccount.DeepCopy()
	newServiceAccount.OwnerReferences = serviceAccount.OwnerReferences
	newServiceAccount.Labels = serviceAccount.Labels
	newServiceAccount.Annotations = serviceAccount.Annotations
	newServiceAccount.AutomountServiceAccountToken = serviceAccount.AutomountServiceAccountToken

	newServiceAccountJson, err := json.Marshal(newServiceAccount)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal new ServiceAccount object: %w", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldServiceAccountJson, newServiceAccountJson, corev1.ServiceAccount{})
	if err != nil {
		return nil, fmt.Errorf("failed to create two way merge patch ServiceAccount objects: %w", err)
	}

	if !IsEmptyPatch(patchBytes) {
		patchedServiceAccount, err := kubeutil.kubeClient.CoreV1().ServiceAccounts(serviceAccount.GetNamespace()).Patch(context.TODO(), serviceAccount.GetName(), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to patch ServiceAccount object: %w", err)
		}
		log.Debugf("Patched ServiceAccount %s in namespace %s", patchedServiceAccount.GetName(), patchedServiceAccount.GetNamespace())
		return patchedServiceAccount, nil
	} else {
		log.Debugf("No need to patch ServiceAccount %s ", serviceAccount.GetName())
	}

	return oldServiceAccount, nil
}

// DeleteServiceAccount Deletes service account
func (kubeutil *Kube) DeleteServiceAccount(namespace, name string) error {
	_, err := kubeutil.GetServiceAccount(namespace, name)
	if err != nil && errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get service account object: %v", err)
	}
	err = kubeutil.kubeClient.CoreV1().ServiceAccounts(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete ServiceAccount object: %v", err)
	}
	return nil
}

func (kubeutil *Kube) GetServiceAccount(namespace, name string) (*corev1.ServiceAccount, error) {
	var serviceAccount *corev1.ServiceAccount
	var err error

	if kubeutil.ServiceAccountLister != nil {
		serviceAccount, err = kubeutil.ServiceAccountLister.ServiceAccounts(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		serviceAccount, err = kubeutil.kubeClient.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return serviceAccount, nil
}

// List service accounts in namespace
func (kubeutil *Kube) ListServiceAccounts(namespace string) ([]*corev1.ServiceAccount, error) {
	return kubeutil.ListServiceAccountsWithSelector(namespace, "")
}

// List service accounts with selector in namespace
func (kubeutil *Kube) ListServiceAccountsWithSelector(namespace string, labelSelectorString string) ([]*corev1.ServiceAccount, error) {
	var serviceAccounts []*corev1.ServiceAccount

	if kubeutil.ServiceAccountLister != nil {
		selector, err := labels.Parse(labelSelectorString)
		if err != nil {
			return nil, err
		}

		serviceAccounts, err = kubeutil.ServiceAccountLister.ServiceAccounts(namespace).List(selector)
		if err != nil {
			return nil, err
		}
	} else {
		listOptions := metav1.ListOptions{
			LabelSelector: labelSelectorString,
		}

		list, err := kubeutil.kubeClient.CoreV1().ServiceAccounts(namespace).List(context.TODO(), listOptions)
		if err != nil {
			return nil, err
		}

		serviceAccounts = slice.PointersOf(list.Items).([]*corev1.ServiceAccount)
	}

	return serviceAccounts, nil
}
