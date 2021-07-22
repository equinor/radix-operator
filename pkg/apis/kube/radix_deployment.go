package kube

import (
	"context"
	"fmt"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetRadixDeployment Gets deployment using lister if present
func (kubeutil *Kube) GetRadixDeployment(namespace, name string) (*v1.RadixDeployment, error) {
	var deployment *v1.RadixDeployment
	var err error

	if kubeutil.RdLister != nil {
		deployment, err = kubeutil.RdLister.RadixDeployments(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		deployment, err = kubeutil.radixclient.RadixV1().RadixDeployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return deployment, nil
}

// ListRadixDeployments Gets deployments using lister if present
func (kubeutil *Kube) ListRadixDeployments(namespace string) ([]*v1.RadixDeployment, error) {
	var allRDs []*v1.RadixDeployment
	var err error

	if kubeutil.RdLister != nil {
		allRDs, err = kubeutil.RdLister.RadixDeployments(namespace).List(labels.NewSelector())
		if err != nil {
			err = fmt.Errorf("Failed to get all RadixDeployments. Error was %v", err)
		}
	} else {
		rds, err := kubeutil.radixclient.RadixV1().RadixDeployments(namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			err = fmt.Errorf("Failed to get all RadixDeployments. Error was %v", err)
		}

		allRDs = slice.PointersOf(rds.Items).([]*v1.RadixDeployment)
	}

	return allRDs, err
}

//GetActiveDeployment Get active RadixDeployment for the namespace
func (kubeutil *Kube) GetActiveDeployment(namespace string) (*v1.RadixDeployment, error) {
	radixDeployments, err := kubeutil.ListRadixDeployments(namespace)
	if err != nil {
		return nil, err
	}
	for _, rd := range radixDeployments {
		if rd.Status.ActiveTo.IsZero() {
			return rd, err
		}
	}
	return nil, nil
}
