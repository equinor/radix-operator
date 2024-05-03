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
func (kubeutil *Kube) GetRadixDeployment(ctx context.Context, namespace, name string) (*v1.RadixDeployment, error) {
	var deployment *v1.RadixDeployment
	var err error

	if kubeutil.RdLister != nil {
		deployment, err = kubeutil.RdLister.RadixDeployments(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		deployment, err = kubeutil.radixclient.RadixV1().RadixDeployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return deployment, nil
}

// ListRadixDeployments Gets deployments using lister if present
func (kubeutil *Kube) ListRadixDeployments(ctx context.Context, namespace string) ([]*v1.RadixDeployment, error) {

	if kubeutil.RdLister != nil {
		rds, err := kubeutil.RdLister.RadixDeployments(namespace).List(labels.NewSelector())
		if err != nil {
			return nil, fmt.Errorf("failed to get all RadixDeployments. Error was %v", err)
		}
		return rds, nil
	}

	rds, err := kubeutil.radixclient.RadixV1().RadixDeployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get all RadixDeployments. Error was %v", err)
	}

	return slice.PointersOf(rds.Items).([]*v1.RadixDeployment), nil

}

// GetActiveDeployment Get active RadixDeployment for the namespace
func (kubeutil *Kube) GetActiveDeployment(ctx context.Context, namespace string) (*v1.RadixDeployment, error) {
	radixDeployments, err := kubeutil.radixclient.RadixV1().RadixDeployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, rd := range radixDeployments.Items {
		if rd.Status.ActiveTo.IsZero() {
			return &rd, err
		}
	}
	return nil, nil
}
