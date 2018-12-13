package kube

import (
	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"github.com/statoil/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO : This should be moved closer to Application domain/package
func (kube *Kube) ApplyPipelineServiceAccount(radixRegistration *v1.RadixRegistration) (*corev1.ServiceAccount, error) {
	namespace := utils.GetAppNamespace(radixRegistration.Name)
	return kube.ApplyServiceAccount("radix-pipeline", namespace)
}

func (kube *Kube) ApplyServiceAccount(serviceAccountName, namespace string) (*corev1.ServiceAccount, error) {
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}

	sa, err := kube.kubeClient.CoreV1().ServiceAccounts(namespace).Create(&serviceAccount)
	if errors.IsAlreadyExists(err) {
		log.Infof("Pipeline service account already exist")
		sa, err = kube.kubeClient.CoreV1().ServiceAccounts(namespace).Get(serviceAccount.ObjectMeta.Name, metav1.GetOptions{})
		return sa, nil
	}

	if err != nil {
		return nil, err
	}
	return sa, nil
}
