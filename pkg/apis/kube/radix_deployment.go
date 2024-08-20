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

// GetRadixDeploymentsForApp Get all Radix deployments for an application
func (kubeutil *Kube) GetRadixDeploymentsForApp(ctx context.Context, appName string, labelSelector string) ([]v1.RadixDeployment, error) {
	envNamespaces, err := kubeutil.GetEnvNamespacesForApp(ctx, appName)
	if err != nil {
		return nil, err
	}
	var radixDeployments []v1.RadixDeployment
	for _, namespace := range envNamespaces {
		radixDeploymentList, err := kubeutil.RadixClient().RadixV1().RadixDeployments(namespace.GetName()).List(ctx,
			metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return nil, err
		}
		radixDeployments = append(radixDeployments, radixDeploymentList.Items...)
	}
	return radixDeployments, nil
}
