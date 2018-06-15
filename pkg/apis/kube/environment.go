package kube

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	radixv1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//CreateEnvironment creates a namespace with RadixRegistration as owner
func (k *Kube) CreateEnvironment(registration *radixv1.RadixRegistration, envName string) error {
	trueVar := true
	name := fmt.Sprintf("%s-%s", registration.Name, envName)
	ns := &v1.Namespace{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"radixApp": registration.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "radix.equinor.com/v1",
					Kind:       "RadixRegistration",
					Name:       registration.Name,
					UID:        registration.UID,
					Controller: &trueVar,
				},
			},
		},
		Spec: v1.NamespaceSpec{},
	}

	_, err := k.kubeClient.CoreV1().Namespaces().Create(ns)
	if errors.IsAlreadyExists(err) {
		log.Infof("Namespace %s already exists", name)
		return nil
	}

	if err != nil {
		log.Errorf("Failed to create namespace %s: %v", name, err)
		return err
	}
	log.Infof("Created namespace %s", name)
	return nil
}
