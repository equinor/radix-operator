package applicationconfig

import (
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils/errors"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app *ApplicationConfig) createOrUpdateDNSAliases() error {
	appName := app.registration.Name
	existingRadixDNSAliasesMap, err := kube.GetRadixDNSAliasMapWithSelector(app.radixclient, labels.ForApplicationName(app.config.Name).String())
	if err != nil {
		return err
	}
	var errs []error
	var radixDNSAliasesToCreate []*radixv1.RadixDNSAlias
	var radixDNSAliasesToUpdate []*radixv1.RadixDNSAlias
	for _, dnsAlias := range app.config.Spec.DNSAlias {
		port, err := app.getPortForDNSAlias(dnsAlias)
		if err != nil {
			return err
		}
		if existingRadixDNSAlias, ok := existingRadixDNSAliasesMap[dnsAlias.Alias]; ok {
			switch {
			case !strings.EqualFold(appName, existingRadixDNSAlias.Spec.AppName):
				errs = append(errs, fmt.Errorf("existing DNS alias %s is used by the application %s", dnsAlias.Alias, existingRadixDNSAlias.Spec.AppName))
				delete(existingRadixDNSAliasesMap, dnsAlias.Alias)
				continue
			case strings.EqualFold(dnsAlias.Environment, existingRadixDNSAlias.Spec.Environment) &&
				strings.EqualFold(dnsAlias.Component, existingRadixDNSAlias.Spec.Component):
				if port != existingRadixDNSAlias.Spec.Port {
					updatingRadixDNSAlias := existingRadixDNSAlias.DeepCopy()
					updatingRadixDNSAlias.Spec.Port = port
					radixDNSAliasesToUpdate = append(radixDNSAliasesToUpdate, updatingRadixDNSAlias)
				}
				delete(existingRadixDNSAliasesMap, dnsAlias.Alias)
				continue
			}
		}
		radixDNSAliasesToCreate = append(radixDNSAliasesToCreate, app.buildRadixDNSAlias(appName, dnsAlias, port))
	}
	if len(errs) > 0 {
		return errors.Concat(errs)
	}
	for _, radixDNSAlias := range existingRadixDNSAliasesMap {
		if err = app.kubeutil.DeleteRadixDNSAliases(radixDNSAlias); err != nil {
			errs = append(errs, err)
		}
	}
	for _, radixDNSAlias := range radixDNSAliasesToUpdate {
		if err = app.kubeutil.UpdateRadixDNSAlias(radixDNSAlias); err != nil {
			errs = append(errs, err)
		}
	}
	for _, radixDNSAlias := range radixDNSAliasesToCreate {
		if err = app.kubeutil.CreateRadixDNSAlias(radixDNSAlias); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Concat(errs)
}

func getComponentPublicPort(component *radixv1.RadixComponent) *radixv1.ComponentPort {
	if port, ok := slice.FindFirst(component.GetPorts(), func(p radixv1.ComponentPort) bool { return p.Name == component.PublicPort }); ok {
		return &port
	}
	return nil
}

func (app *ApplicationConfig) buildRadixDNSAlias(appName string, dnsAlias radixv1.DNSAlias, port int32) *radixv1.RadixDNSAlias {
	return &radixv1.RadixDNSAlias{
		ObjectMeta: metav1.ObjectMeta{
			Name:            dnsAlias.Alias,
			Labels:          labels.Merge(labels.ForApplicationName(appName), labels.ForComponentName(dnsAlias.Component), labels.ForEnvironmentName(dnsAlias.Environment)),
			OwnerReferences: []metav1.OwnerReference{getOwnerReferenceOfRadixRegistration(app.registration)},
			Finalizers:      []string{kube.RadixDNSAliasFinalizer},
		},
		Spec: radixv1.RadixDNSAliasSpec{
			AppName:     appName,
			Environment: dnsAlias.Environment,
			Component:   dnsAlias.Component,
			Port:        port,
		},
	}
}
