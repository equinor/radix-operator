package kube

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

// ApplyServiceAccount Creates or updates service account
func (kube *Kube) ApplyServiceAccount(serviceAccount corev1.ServiceAccount) (*corev1.ServiceAccount, error) {
	oldServiceAccount, err := kube.getServiceAccount(serviceAccount.Namespace, serviceAccount.GetName())
	if err != nil && errors.IsNotFound(err) {
		_, err := kube.kubeClient.CoreV1().ServiceAccounts(serviceAccount.Namespace).Create(&serviceAccount)
		if err != nil {
			return nil, fmt.Errorf("Failed to create ServiceAccount object: %v", err)
		}
		time.Sleep(time.Millisecond * 200) //Wait while a secret Can be returned with the ServiceAccount
		createdServiceAccount, err := kube.getServiceAccount(serviceAccount.Namespace, serviceAccount.GetName())
		if createdServiceAccount == nil {
			return nil, fmt.Errorf("Cannot get created ServiceAccount: %s in namespace %s", serviceAccount.GetName(), serviceAccount.Namespace)
		}
		log.Debugf("Created ServiceAccount: %s in namespace %s", createdServiceAccount.Name, serviceAccount.Namespace)
		return createdServiceAccount, nil
	} else if err != nil {
		return nil, fmt.Errorf("Failed to get service account object: %v", err)

	}

	log.Debugf("ServiceAccount object %s already exists in namespace %s", serviceAccount.GetName(), serviceAccount.Namespace)
	return oldServiceAccount, nil
}

// DeleteServiceAccount Deletes service account
func (kube *Kube) DeleteServiceAccount(namespace, name string) error {
	_, err := kube.getServiceAccount(namespace, name)
	if err != nil && errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("Failed to get service account object: %v", err)
	}
	err = kube.kubeClient.CoreV1().ServiceAccounts(namespace).Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("Failed to delete ServiceAccount object: %v", err)
	}
	return nil
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
