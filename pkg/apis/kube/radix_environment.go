package kube

import (
	"context"
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetEnvironment Gets environment using lister from cache if present
func (kubeutil *Kube) GetEnvironment(ctx context.Context, name string) (*radixv1.RadixEnvironment, error) {
	var environment *radixv1.RadixEnvironment
	var err error

	if kubeutil.ReLister != nil {
		environment, err = kubeutil.ReLister.Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		environment, err = kubeutil.radixclient.RadixV1().RadixEnvironments().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return environment, nil
}

// ListEnvironmentsWithSelector Gets lists environments with label selector
func (kubeutil *Kube) ListEnvironmentsWithSelector(ctx context.Context, labelSelectorString string) ([]*radixv1.RadixEnvironment, error) {
	listOptions := metav1.ListOptions{LabelSelector: labelSelectorString}
	list, err := kubeutil.radixclient.RadixV1().RadixEnvironments().List(ctx, listOptions)
	if err != nil {
		return nil, err
	}
	return slice.PointersOf(list.Items).([]*radixv1.RadixEnvironment), nil
}

// ListEnvironments lists environments from cache if lister is present
func (kubeutil *Kube) ListEnvironments(ctx context.Context) ([]*radixv1.RadixEnvironment, error) {
	var environments []*radixv1.RadixEnvironment
	var err error

	if kubeutil.ReLister != nil {
		environments, err = kubeutil.ReLister.List(labels.NewSelector())
		if err != nil {
			return nil, err
		}
	} else {
		list, err := kubeutil.radixclient.RadixV1().RadixEnvironments().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		environments = slice.PointersOf(list.Items).([]*radixv1.RadixEnvironment)
	}

	return environments, nil
}

// UpdateRadixEnvironment Updates changes of RadixEnvironment if any
func (kubeutil *Kube) UpdateRadixEnvironment(ctx context.Context, radixEnvironment *radixv1.RadixEnvironment) (*radixv1.RadixEnvironment, error) {
	log.Ctx(ctx).Debug().Msgf("Update RadixEnvironment %s in the application %s", radixEnvironment.Name, radixEnvironment.Spec.AppName)
	updated, err := kubeutil.RadixClient().RadixV1().RadixEnvironments().Update(ctx, radixEnvironment, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update RadixEnvironment object: %v", err)
	}
	log.Ctx(ctx).Debug().Msgf("Updated RadixEnvironment: %s in the application %s", radixEnvironment.Name, radixEnvironment.Spec.AppName)
	return updated, nil
}
