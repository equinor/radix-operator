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
	for _, dnsAlias := range app.config.Spec.DNSAlias {
		port, err := app.getPortForDNSAlias(dnsAlias)
		if err != nil {
			return err
		}
		if existingRadixDNSAlias, ok := existingRadixDNSAliasesMap[dnsAlias.Alias]; ok {
			switch {
			case !strings.EqualFold(appName, existingRadixDNSAlias.Spec.AppName):
				errs = append(errs, fmt.Errorf("existing DNS alias %s is used by the application %s", dnsAlias.Alias, existingRadixDNSAlias.Spec.AppName))
			case strings.EqualFold(dnsAlias.Environment, existingRadixDNSAlias.Spec.Environment) &&
				strings.EqualFold(dnsAlias.Component, existingRadixDNSAlias.Spec.Component) &&
				port == existingRadixDNSAlias.Spec.Port:
				// No changes
			default:
				if err = app.updateRadixDNSAlias(existingRadixDNSAlias, dnsAlias, port); err != nil {
					errs = append(errs, err)
				}
			}
			delete(existingRadixDNSAliasesMap, dnsAlias.Alias)
			continue
		}
		if err = app.createRadixDNSAlias(appName, dnsAlias, port); err != nil {
			errs = append(errs, err)
		}
	}
	for _, radixDNSAlias := range existingRadixDNSAliasesMap {
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
			Name:            dnsAlias.Alias,
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
