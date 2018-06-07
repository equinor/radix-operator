package kube

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	radixv1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k *Kube) CreateEnvironment(app *radixv1.RadixApplication, envName string) error {
	trueVar := true
	name := fmt.Sprintf("%s-%s", app.Name, envName)
	ns := &v1.Namespace{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"radixApp": app.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "radix.equinor.com/v1",
					Kind:       "RadixApplication",
					Name:       app.Name,
					UID:        app.UID,
					Controller: &trueVar,
				},
			},
		},
		Spec: v1.NamespaceSpec{},
	}

	_, err := k.kubeClient.CoreV1().Namespaces().Create(ns)

	if err != nil {
		log.Errorf("Failed to create namespace %s: %v", name, err)
		return err
	}
	log.Infof("Created namespace %s", name)
	return nil
}
