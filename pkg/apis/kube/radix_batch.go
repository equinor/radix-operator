package kube

import (
	"context"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetRadixBatch Gets batches using lister if present
func (kubeutil *Kube) GetRadixBatch(namespace, name string) (*v1.RadixBatch, error) {
	var batch *v1.RadixBatch
	var err error

	if kubeutil.RbLister != nil {
		batch, err = kubeutil.RbLister.RadixBatches(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		batch, err = kubeutil.radixclient.RadixV1().RadixBatches(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return batch, nil
}

// ListRadixBatches Gets batches using lister if present
func (kubeutil *Kube) ListRadixBatches(ctx context.Context, namespace string) ([]*v1.RadixBatch, error) {

	if kubeutil.RbLister != nil {
		rbs, err := kubeutil.RbLister.RadixBatches(namespace).List(labels.NewSelector())
		if err != nil {
			return nil, fmt.Errorf("failed to get all RadixBatches. Error was %v", err)
		}
		return rbs, nil
	}

	rbs, err := kubeutil.radixclient.RadixV1().RadixBatches(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get all RadixBatches. Error was %v", err)
	}

	return slice.PointersOf(rbs.Items).([]*v1.RadixBatch), nil

}

// DeleteRadixBatch Deletes a batch
func (kubeutil *Kube) DeleteRadixBatch(ctx context.Context, namespace, name string) error {
	return kubeutil.radixclient.RadixV1().RadixBatches(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}
