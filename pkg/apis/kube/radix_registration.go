package kube

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetRegistration Gets registration using lister if present
func (kube *Kube) GetRegistration(name string) (*v1.RadixRegistration, error) {
	var registration *v1.RadixRegistration
	var err error

	if kube.RrLister != nil {
		registration, err = kube.RrLister.Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		registration, err = kube.radixclient.RadixV1().RadixRegistrations().Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return registration, nil
}

// ListRegistrations lists registrations
func (kube *Kube) ListRegistrations() ([]*v1.RadixRegistration, error) {
	var registrations []*v1.RadixRegistration
	var err error

	if kube.RrLister != nil {
		registrations, err = kube.RrLister.List(labels.NewSelector())
		if err != nil {
			return nil, err
		}
	} else {
		list, err := kube.radixclient.RadixV1().RadixRegistrations().List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		registrations = slice.PointersOf(list.Items).([]*v1.RadixRegistration)
	}

	return registrations, nil
}
