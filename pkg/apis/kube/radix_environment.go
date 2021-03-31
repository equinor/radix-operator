package kube

import (
	"context"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetEnvironment Gets environment using lister from cache if present
func (kubeutil *Kube) GetEnvironment(name string) (*v1.RadixEnvironment, error) {
	var environment *v1.RadixEnvironment
	var err error

	if kubeutil.ReLister != nil {
		environment, err = kubeutil.ReLister.Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		environment, err = kubeutil.radixclient.RadixV1().RadixEnvironments().Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return environment, nil
}

// ListEnvironments lists environments from cache if lister is present
func (kubeutil *Kube) ListEnvironments() ([]*v1.RadixEnvironment, error) {
	var environments []*v1.RadixEnvironment
	var err error

	if kubeutil.ReLister != nil {
		environments, err = kubeutil.ReLister.List(labels.NewSelector())
		if err != nil {
			return nil, err
		}
	} else {
		list, err := kubeutil.radixclient.RadixV1().RadixEnvironments().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		environments = slice.PointersOf(list.Items).([]*v1.RadixEnvironment)
	}

	return environments, nil
}
