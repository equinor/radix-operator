package kube

import (
	"context"
	"fmt"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetRadixDeployment Gets radix alert using lister if present
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

// ListRadixAlert Gets radix alerts using lister if present
func (kubeutil *Kube) ListRadixAlert(namespace string) ([]*v1.RadixAlert, error) {
	if kubeutil.RadixAlertLister != nil {
		alerts, err := kubeutil.RadixAlertLister.RadixAlerts(namespace).List(labels.NewSelector())
		if err != nil {
			return nil, fmt.Errorf("failed to get all RadixAlerts. Error was %v", err)
		}
		return alerts, nil
	}

	rds, err := kubeutil.radixclient.RadixV1().RadixAlerts(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get all RadixAlerts. Error was %v", err)
	}

	return slice.PointersOf(rds.Items).([]*v1.RadixAlert), nil
}
