package kube

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// CreateServiceAccount create a service account
func (kubeutil *Kube) CreateServiceAccount(ctx context.Context, namespace, name string) (*corev1.ServiceAccount, error) {
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	return kubeutil.ApplyServiceAccount(ctx, &serviceAccount)
}

// ApplyServiceAccount Creates or updates service account
func (kubeutil *Kube) ApplyServiceAccount(ctx context.Context, serviceAccount *corev1.ServiceAccount) (*corev1.ServiceAccount, error) {
	oldServiceAccount, err := kubeutil.GetServiceAccount(ctx, serviceAccount.Namespace, serviceAccount.GetName())
	logger := log.Ctx(ctx)
	if err != nil && errors.IsNotFound(err) {
		createdServiceAccount, err := kubeutil.kubeClient.CoreV1().ServiceAccounts(serviceAccount.GetNamespace()).Create(ctx, serviceAccount, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create ServiceAccount object: %w", err)
		}
		logger.Debug().Msgf("Created ServiceAccount %s in namespace %s", createdServiceAccount.GetName(), createdServiceAccount.GetNamespace())
		return createdServiceAccount, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get ServiceAccount object: %w", err)
	}

	logger.Debug().Msgf("ServiceAccount object %s already exists in namespace %s, updating the object now", serviceAccount.GetName(), serviceAccount.GetNamespace())
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
		patchedServiceAccount, err := kubeutil.kubeClient.CoreV1().ServiceAccounts(serviceAccount.GetNamespace()).Patch(ctx, serviceAccount.GetName(), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to patch ServiceAccount object: %w", err)
		}
		logger.Debug().Msgf("Patched ServiceAccount %s in namespace %s", patchedServiceAccount.GetName(), patchedServiceAccount.GetNamespace())
		return patchedServiceAccount, nil
	} else {
		logger.Debug().Msgf("No need to patch ServiceAccount %s ", serviceAccount.GetName())
	}

	return oldServiceAccount, nil
}

// DeleteServiceAccount Deletes service account
func (kubeutil *Kube) DeleteServiceAccount(ctx context.Context, namespace, name string) error {
	_, err := kubeutil.GetServiceAccount(ctx, namespace, name)
	if err != nil && errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get service account object: %v", err)
	}
	err = kubeutil.kubeClient.CoreV1().ServiceAccounts(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete ServiceAccount object: %v", err)
	}
	return nil
}

func (kubeutil *Kube) GetServiceAccount(ctx context.Context, namespace, name string) (*corev1.ServiceAccount, error) {
	var serviceAccount *corev1.ServiceAccount
	var err error

	if kubeutil.ServiceAccountLister != nil {
		serviceAccount, err = kubeutil.ServiceAccountLister.ServiceAccounts(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		serviceAccount, err = kubeutil.kubeClient.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return serviceAccount, nil
}

// ListServiceAccounts List service accounts in namespace
func (kubeutil *Kube) ListServiceAccounts(ctx context.Context, namespace string) ([]*corev1.ServiceAccount, error) {
	return kubeutil.ListServiceAccountsWithSelector(ctx, namespace, "")
}

// ListServiceAccountsWithSelector List service accounts with selector in namespace
func (kubeutil *Kube) ListServiceAccountsWithSelector(ctx context.Context, namespace string, labelSelectorString string) ([]*corev1.ServiceAccount, error) {
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

		list, err := kubeutil.kubeClient.CoreV1().ServiceAccounts(namespace).List(ctx, listOptions)
		if err != nil {
			return nil, err
		}

		serviceAccounts = slice.PointersOf(list.Items).([]*corev1.ServiceAccount)
	}

	return serviceAccounts, nil
}
