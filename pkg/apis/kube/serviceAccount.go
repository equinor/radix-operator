package kube

import (
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplyServiceAccount Creates or updates service account
func (kube *Kube) ApplyServiceAccount(serviceAccountName, namespace string) (*corev1.ServiceAccount, error) {
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}

	sa, err := kube.kubeClient.CoreV1().ServiceAccounts(namespace).Create(&serviceAccount)
	if errors.IsAlreadyExists(err) {
		log.Debugf("Pipeline service account already exist")
		sa, err = kube.kubeClient.CoreV1().ServiceAccounts(namespace).Get(serviceAccount.ObjectMeta.Name, metav1.GetOptions{})
		return sa, nil
	}

	if err != nil {
		return nil, err
	}
	return sa, nil
}
