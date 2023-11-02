package applicationconfig

import (
	"context"
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
	radixDNSAliasesMap, err := app.getRadixDNSAliasesMapForAppName(appName)
	if err != nil {
		return err
	}
	var errs []error
	for _, dnsAlias := range app.config.Spec.DNSAlias {
		port, err := app.getPortForDNSAlias(dnsAlias)
		if err != nil {
			return err
		}
		if radixDNSAlias, ok := radixDNSAliasesMap[dnsAlias.Domain]; ok {
			switch {
			case !strings.EqualFold(appName, radixDNSAlias.Spec.AppName):
				errs = append(errs, fmt.Errorf("existing DNS alias domain %s is used by the application %s", dnsAlias.Domain, radixDNSAlias.Spec.AppName))
			case strings.EqualFold(dnsAlias.Environment, radixDNSAlias.Spec.Environment) &&
				strings.EqualFold(dnsAlias.Component, radixDNSAlias.Spec.Component) &&
				port == radixDNSAlias.Spec.Port:
				// No changes
			default:
				if err = app.updateRadixDNSAlias(radixDNSAlias, dnsAlias, port); err != nil {
					errs = append(errs, err)
				}
			}
			delete(radixDNSAliasesMap, dnsAlias.Domain)
			continue
		}
		if err = app.createRadixDNSAlias(appName, dnsAlias, port); err != nil {
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

func getComponentPublicPort(component *radixv1.RadixComponent) *radixv1.ComponentPort {
	if port, ok := slice.FindFirst(component.GetPorts(), func(p radixv1.ComponentPort) bool { return p.Name == component.PublicPort }); ok {
		return &port
	}
	return nil
}

func (app *ApplicationConfig) updateRadixDNSAlias(radixDNSAlias *radixv1.RadixDNSAlias, dnsAlias radixv1.DNSAlias, port int32) error {
	updatedRadixDNSAlias := radixDNSAlias.DeepCopy()
	updatedRadixDNSAlias.Spec.Environment = dnsAlias.Environment
	updatedRadixDNSAlias.Spec.Component = dnsAlias.Component
	updatedRadixDNSAlias.Spec.Port = port
	return app.kubeutil.UpdateRadixDNSAlias(updatedRadixDNSAlias)
}

func (app *ApplicationConfig) createRadixDNSAlias(appName string, dnsAlias radixv1.DNSAlias, port int32) error {
	radixDNSAlias := radixv1.RadixDNSAlias{
		ObjectMeta: metav1.ObjectMeta{
			Name:            dnsAlias.Domain,
			Labels:          labels.Merge(labels.ForApplicationName(appName), labels.ForComponentName(dnsAlias.Component)),
			OwnerReferences: []metav1.OwnerReference{getOwnerReferenceOfApplication(app.config)},
		},
		Spec: radixv1.RadixDNSAliasSpec{
			AppName:     appName,
			Environment: dnsAlias.Environment,
			Component:   dnsAlias.Component,
			Port:        port,
		},
	}
	return app.kubeutil.CreateRadixDNSAlias(&radixDNSAlias)
}

func (app *ApplicationConfig) getRadixDNSAliasesMapForAppName(appName string) (map[string]*radixv1.RadixDNSAlias, error) {
	radixDNSAliasList, err := app.kubeutil.ListRadixDNSAliasWithSelector(labels.ForApplicationName(appName).String())
	if err != nil {
		return nil, err
	}
	return getRadixDNSAliasMap(radixDNSAliasList), err
}

func (app *ApplicationConfig) getAllRadixDNSAliasesMap() (map[string]*radixv1.RadixDNSAlias, error) {
	radixDNSAliasList, err := app.kubeutil.RadixClient().RadixV1().RadixDNSAliases().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return getRadixDNSAliasMap(slice.PointersOf(radixDNSAliasList.Items).([]*radixv1.RadixDNSAlias)), nil
}

func getRadixDNSAliasMap(dnsAliases []*radixv1.RadixDNSAlias) map[string]*radixv1.RadixDNSAlias {
	return slice.Reduce(dnsAliases, make(map[string]*radixv1.RadixDNSAlias), func(acc map[string]*radixv1.RadixDNSAlias, dnsAlias *radixv1.RadixDNSAlias) map[string]*radixv1.RadixDNSAlias {
		acc[dnsAlias.Name] = dnsAlias
		return acc
	})
}
