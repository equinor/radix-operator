package kube

import (
	"context"
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// CreateRadixDNSAlias Creates RadixDNSAlias
func (kubeutil *Kube) CreateRadixDNSAlias(radixDNSAlias *radixv1.RadixDNSAlias) error {
	_, err := kubeutil.radixclient.RadixV1().RadixDNSAliases().Create(context.Background(), radixDNSAlias, metav1.CreateOptions{})
	return err
}

// GetRadixDNSAlias Gets RadixDNSAlias using lister if present
func (kubeutil *Kube) GetRadixDNSAlias(name string) (*radixv1.RadixDNSAlias, error) {
	var alias *radixv1.RadixDNSAlias
	var err error
	if kubeutil.RadixDNSAliasLister != nil {
		if alias, err = kubeutil.RadixDNSAliasLister.Get(name); err != nil {
			return nil, err
		}
		return alias, nil
	}
	if alias, err = kubeutil.radixclient.RadixV1().RadixDNSAliases().Get(context.TODO(), name, metav1.GetOptions{}); err != nil {
		return nil, err
	}
	return alias, nil
}

// ListRadixDNSAlias List RadixDNSAliases using lister if present
func (kubeutil *Kube) ListRadixDNSAlias() ([]*radixv1.RadixDNSAlias, error) {
	return kubeutil.ListRadixDNSAliasWithSelector("")
}

// ListRadixDNSAliasWithSelector List radix DNS aliases with selector
func (kubeutil *Kube) ListRadixDNSAliasWithSelector(labelSelectorString string) ([]*radixv1.RadixDNSAlias, error) {
	if kubeutil.RadixDNSAliasLister != nil {
		selector, err := labels.Parse(labelSelectorString)
		if err != nil {
			return nil, err
		}
		aliases, err := kubeutil.RadixDNSAliasLister.List(selector)
		if err != nil {
			return nil, fmt.Errorf("failed to get all RadixDNSAliases. Error was %v", err)
		}
		return aliases, nil
	}

	aliasList, err := kubeutil.radixclient.RadixV1().RadixDNSAliases().List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelectorString})
	if err != nil {
		return nil, fmt.Errorf("failed to get all RadixDNSAliases. Error was %v", err)
	}
	return slice.PointersOf(aliasList.Items).([]*radixv1.RadixDNSAlias), nil
}

// UpdateRadixDNSAlias Update RadixDNSAlias
func (kubeutil *Kube) UpdateRadixDNSAlias(radixDNSAlias *radixv1.RadixDNSAlias) error {
	_, err := kubeutil.radixclient.RadixV1().RadixDNSAliases().Update(context.Background(), radixDNSAlias, metav1.UpdateOptions{})
	return err
}

// DeleteRadixDNSAlias Delete RadixDNSAliases
func (kubeutil *Kube) DeleteRadixDNSAlias(radixDNSAlias *radixv1.RadixDNSAlias) error {
	return kubeutil.radixclient.RadixV1().RadixDNSAliases().Delete(context.Background(), radixDNSAlias.GetName(), metav1.DeleteOptions{})
}
