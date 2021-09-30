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
func (kubeutil *Kube) GetRadixAlert(namespace, name string) (*v1.RadixAlert, error) {
	var alert *v1.RadixAlert
	var err error

	if kubeutil.RadixAlertLister != nil {
		alert, err = kubeutil.RadixAlertLister.RadixAlerts(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		alert, err = kubeutil.radixclient.RadixV1().RadixAlerts(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return alert, nil
}

// ListRadixDeployments Gets deployments using lister if present
func (kubeutil *Kube) ListRadixAlert(namespace string) ([]*v1.RadixDeployment, error) {
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
