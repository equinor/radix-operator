package kube

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ApplyServiceAccount Creates or updates service account
func (kube *Kube) ApplyServiceAccount(serviceAccountName, namespace string) (*corev1.ServiceAccount, error) {
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}

	oldServiceAccount, err := kube.getServiceAccount(namespace, serviceAccount.GetName())
	if err != nil && errors.IsNotFound(err) {
		createdServiceAccount, err := kube.kubeClient.CoreV1().ServiceAccounts(namespace).Create(&serviceAccount)
		if err != nil {
			return nil, fmt.Errorf("Failed to create ServiceAccount object: %v", err)
		}

		log.Debugf("Created ServiceAccount: %s in namespace %s", createdServiceAccount.Name, namespace)
		return createdServiceAccount, nil
	}

	log.Debugf("ServiceAccount object %s already exists in namespace %s", serviceAccount.GetName(), namespace)
	return oldServiceAccount, nil
}

// ListServiceAccounts List service account
func (kube *Kube) ListServiceAccounts(namespace string) ([]*corev1.ServiceAccount, error) {
	var serviceAccounts []*corev1.ServiceAccount
	var err error

	if kube.ServiceAccountLister != nil {
		serviceAccounts, err = kube.ServiceAccountLister.ServiceAccounts(namespace).List(labels.NewSelector())
		if err != nil {
			return nil, err
		}
	} else {
		list, err := kube.kubeClient.CoreV1().ServiceAccounts(namespace).List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		serviceAccounts = slice.PointersOf(list.Items).([]*corev1.ServiceAccount)
	}

	return serviceAccounts, nil
}

func (kube *Kube) getServiceAccount(namespace, name string) (*corev1.ServiceAccount, error) {
	var serviceAccount *corev1.ServiceAccount
	var err error

	if kube.ServiceAccountLister != nil {
		serviceAccount, err = kube.ServiceAccountLister.ServiceAccounts(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		serviceAccount, err = kube.kubeClient.CoreV1().ServiceAccounts(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return serviceAccount, nil
}
