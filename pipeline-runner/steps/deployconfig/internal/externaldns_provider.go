package internal

import (
	"fmt"
	"slices"

	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// ExternalDNSFeatureProvider handles external DNS configuration
type ExternalDNSFeatureProvider struct{}

func (d *ExternalDNSFeatureProvider) IsEnabledForEnvironment(envName string, ra radixv1.RadixApplication, activeRd radixv1.RadixDeployment) bool {
	if slices.ContainsFunc(ra.Spec.DNSExternalAlias, func(alias radixv1.ExternalAlias) bool { return alias.Environment == envName }) {
		return true
	}

	if slices.ContainsFunc(activeRd.Spec.Components, func(comp radixv1.RadixDeployComponent) bool { return len(comp.GetExternalDNS()) > 0 }) {
		return true
	}

	return false
}

func (d *ExternalDNSFeatureProvider) Mutate(target, source radixv1.RadixDeployment) error {
	for i, targetComp := range target.Spec.Components {
		sourceComp, found := slice.FindFirst(source.Spec.Components, func(c radixv1.RadixDeployComponent) bool { return c.Name == targetComp.Name })
		if !found {
			return fmt.Errorf("component %s not found in active deployment", targetComp.Name)
		}
		target.Spec.Components[i].ExternalDNS = sourceComp.GetExternalDNS()
	}

	return nil
}
