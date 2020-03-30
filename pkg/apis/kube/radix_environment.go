package kube

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetEnvironment Gets environment using lister from cache if present
func (kube *Kube) GetEnvironment(name string) (*v1.RadixEnvironment, error) {
	var environment *v1.RadixEnvironment
	var err error

	if kube.ReLister != nil {
		environment, err = kube.ReLister.Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		environment, err = kube.radixclient.RadixV1().RadixEnvironments().Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return environment, nil
}

// ListEnvironments lists environments from cache if lister is present
func (kube *Kube) ListEnvironments() ([]*v1.RadixEnvironment, error) {
	var environments []*v1.RadixEnvironment
	var err error

	if kube.ReLister != nil {
		environments, err = kube.ReLister.List(labels.NewSelector())
		if err != nil {
			return nil, err
		}
	} else {
		list, err := kube.radixclient.RadixV1().RadixEnvironments().List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		environments = slice.PointersOf(list.Items).([]*v1.RadixEnvironment)
	}

	return environments, nil
}
