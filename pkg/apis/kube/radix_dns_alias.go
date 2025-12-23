package kube

import (
	"context"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetRadixDNSAlias Gets RadixDNSAlias using lister if present
func (kubeutil *Kube) GetRadixDNSAlias(ctx context.Context, name string) (*radixv1.RadixDNSAlias, error) {
	var alias *radixv1.RadixDNSAlias
	var err error
	if kubeutil.RadixDNSAliasLister != nil {
		if alias, err = kubeutil.RadixDNSAliasLister.Get(name); err != nil {
			return nil, err
		}
		return alias, nil
	}
	if alias, err = kubeutil.radixclient.RadixV1().RadixDNSAliases().Get(ctx, name, metav1.GetOptions{}); err != nil {
		return nil, err
	}
	return alias, nil
}

// ListRadixDNSAlias List RadixDNSAliases using lister if present
func (kubeutil *Kube) ListRadixDNSAlias(ctx context.Context) ([]*radixv1.RadixDNSAlias, error) {
	return kubeutil.ListRadixDNSAliasWithSelector(ctx, "")
}

// ListRadixDNSAliasWithSelector List RadixDNSAliases with selector
func (kubeutil *Kube) ListRadixDNSAliasWithSelector(ctx context.Context, labelSelectorString string) ([]*radixv1.RadixDNSAlias, error) {
	if kubeutil.RadixDNSAliasLister != nil {
		selector, err := labels.Parse(labelSelectorString)
		if err != nil {
			return nil, err
		}
		aliases, err := kubeutil.RadixDNSAliasLister.List(selector)
		if err != nil {
			return nil, fmt.Errorf("failed to get all RadixDNSAliases: %w", err)
		}
		return aliases, nil
	}

	aliasList, err := kubeutil.GetRadixDNSAliasWithSelector(ctx, labelSelectorString)
	if err != nil {
		return nil, fmt.Errorf("failed to get all RadixDNSAliases: %w", err)
	}
	return slice.PointersOf(aliasList.Items).([]*radixv1.RadixDNSAlias), nil
}

// GetRadixDNSAliasWithSelector Get RadixDNSAliases with selector
func (kubeutil *Kube) GetRadixDNSAliasWithSelector(ctx context.Context, labelSelectorString string) (*radixv1.RadixDNSAliasList, error) {
	return kubeutil.radixclient.RadixV1().RadixDNSAliases().List(ctx, metav1.ListOptions{LabelSelector: labelSelectorString})
}

// GetRadixDNSAliasMap Gets a map of all RadixDNSAliases
func GetRadixDNSAliasMap(ctx context.Context, radixClient radixclient.Interface) (map[string]*radixv1.RadixDNSAlias, error) {
	radixDNSAliases, err := radixClient.RadixV1().RadixDNSAliases().List(ctx, metav1.ListOptions{})
	if err != nil {
		return make(map[string]*radixv1.RadixDNSAlias), err
	}
	return slice.Reduce(radixDNSAliases.Items, make(map[string]*radixv1.RadixDNSAlias, len(radixDNSAliases.Items)), func(acc map[string]*radixv1.RadixDNSAlias, dnsAlias radixv1.RadixDNSAlias) map[string]*radixv1.RadixDNSAlias {
		acc[dnsAlias.Name] = &dnsAlias
		return acc
	}), nil
}
