package applicationconfig

import (
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils/errors"
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app *ApplicationConfig) createOrUpdateDNSAliases() error {
	appName := app.registration.Name
	radixDNSAliasesMap, err := app.getRadixDNSAliasesMap(appName)
	if err != nil {
		return err
	}
	var errs []error
	for _, dnsAlias := range app.config.Spec.DNSAlias {
		if radixDNSAlias, ok := radixDNSAliasesMap[dnsAlias.Domain]; ok {
			switch {
			case !strings.EqualFold(appName, radixDNSAlias.Spec.AppName):
				errs = append(errs, fmt.Errorf("existing DNS alias domain %s is used by the application %s", dnsAlias.Domain, radixDNSAlias.Spec.AppName))
			case strings.EqualFold(dnsAlias.Environment, radixDNSAlias.Spec.Environment) && strings.EqualFold(dnsAlias.Component, radixDNSAlias.Spec.Component):
				// No changes
			default:
				if err = app.updateRadixDNSAlias(radixDNSAlias, dnsAlias); err != nil {
					errs = append(errs, err)
				}
			}
			delete(radixDNSAliasesMap, dnsAlias.Domain)
			continue
		}
		if err = app.createRadixDNSAlias(appName, dnsAlias); err != nil {
			errs = append(errs, err)
		}
	}
	for _, radixDNSAlias := range radixDNSAliasesMap {
		if err = app.kubeutil.DeleteRadixDNSAlias(radixDNSAlias); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Concat(errs)
}

func (app *ApplicationConfig) updateRadixDNSAlias(radixDNSAlias *radixv1.RadixDNSAlias, dnsAlias radixv1.DNSAlias) error {
	updatedRadixDNSAlias := radixDNSAlias.DeepCopy()
	updatedRadixDNSAlias.Spec.Environment = dnsAlias.Environment
	updatedRadixDNSAlias.Spec.Component = dnsAlias.Component
	return app.kubeutil.UpdateRadixDNSAlias(updatedRadixDNSAlias)
}

func (app *ApplicationConfig) createRadixDNSAlias(appName string, dnsAlias radixv1.DNSAlias) error {
	radixDNSAlias := radixv1.RadixDNSAlias{
		ObjectMeta: metav1.ObjectMeta{
			Name:   dnsAlias.Domain,
			Labels: labels.Merge(labels.ForApplicationName(appName), labels.ForComponentName(dnsAlias.Component)),
		},
		Spec: radixv1.RadixDNSAliasSpec{
			AppName:     appName,
			Environment: dnsAlias.Environment,
			Component:   dnsAlias.Component,
		},
	}
	return app.kubeutil.CreateRadixDNSAlias(&radixDNSAlias)
}

func (app *ApplicationConfig) getRadixDNSAliasesMap(appName string) (map[string]*radixv1.RadixDNSAlias, error) {
	dnsAliases, err := app.kubeutil.ListRadixDNSAliasWithSelector(labels.ForApplicationName(appName).String())
	if err != nil {
		return nil, err
	}
	return slice.Reduce(dnsAliases, make(map[string]*radixv1.RadixDNSAlias), func(acc map[string]*radixv1.RadixDNSAlias, dnsAlias *radixv1.RadixDNSAlias) map[string]*radixv1.RadixDNSAlias {
		acc[dnsAlias.Name] = dnsAlias
		return acc
	}), err
}
