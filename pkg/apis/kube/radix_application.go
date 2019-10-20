package kube

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetApplication Gets application using lister if present
func (kube *Kube) GetApplication(namespace, name string) (*v1.RadixApplication, error) {
	var application *v1.RadixApplication
	var err error

	if kube.RaLister != nil {
		application, err = kube.RaLister.RadixApplications(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		application, err = kube.radixclient.RadixV1().RadixApplications(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return application, nil
}
