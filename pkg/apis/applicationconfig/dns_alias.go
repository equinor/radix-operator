package applicationconfig

import (
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app *ApplicationConfig) syncDNSAliases() error {
	aliasesToCreate, aliasesToUpdate, aliasesToDelete, errs := app.getDNSAliasesToSync()
	// first - delete
	for _, dnsAlias := range aliasesToDelete {
		if err := app.kubeutil.DeleteRadixDNSAliases(dnsAlias); err != nil {
			errs = append(errs, err)
		}
	}
	// then - update
	for _, dnsAlias := range aliasesToUpdate {
		if err := app.kubeutil.UpdateRadixDNSAlias(dnsAlias); err != nil {
			errs = append(errs, err)
		}
	}
	// then - create
	for _, dnsAlias := range aliasesToCreate {
		if err := app.kubeutil.CreateRadixDNSAlias(dnsAlias); err != nil {
			errs = append(errs, err)
		}
	}
	return stderrors.Join(errs...)
}

func (app *ApplicationConfig) getDNSAliasesToSync() ([]*radixv1.RadixDNSAlias, []*radixv1.RadixDNSAlias, []*radixv1.RadixDNSAlias, []error) {
	existingAliases, err := kube.GetRadixDNSAliasMap(app.radixclient)
	if err != nil {
		return nil, nil, nil, []error{err}
	}

	var aliasesToCreate, aliasesToUpdate, aliasesToDelete []*radixv1.RadixDNSAlias
	processedAliases := make(map[string]any)
	appName := app.registration.Name
	var errs []error
	for _, dnsAlias := range app.config.Spec.DNSAlias {
		port, err := app.getPortForDNSAlias(dnsAlias)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to get a port for DNS alias %s: %v", dnsAlias.Alias, err))
			processedAliases[dnsAlias.Alias] = true
			continue
		}
		if existingAlias, exists := existingAliases[dnsAlias.Alias]; exists {
			if !strings.EqualFold(appName, existingAlias.Spec.AppName) {
				errs = append(errs, fmt.Errorf("existing DNS alias %s is used by another application", dnsAlias.Alias))
				processedAliases[dnsAlias.Alias] = true
				continue
			}
			if strings.EqualFold(dnsAlias.Environment, existingAlias.Spec.Environment) && strings.EqualFold(dnsAlias.Component, existingAlias.Spec.Component) {
				if port != existingAlias.Spec.Port {
					updatingRadixDNSAlias := existingAlias.DeepCopy()
					updatingRadixDNSAlias.Spec.Port = port
					aliasesToUpdate = append(aliasesToUpdate, updatingRadixDNSAlias)
				}
				processedAliases[dnsAlias.Alias] = true
				continue
			}
		}
		aliasesToCreate = append(aliasesToCreate, app.buildRadixDNSAlias(appName, dnsAlias, port)) // new alias or an alias with changed environment or component
	}
	for aliasName, dnsAlias := range existingAliases {
		if _, ok := processedAliases[aliasName]; !ok && strings.EqualFold(dnsAlias.Spec.AppName, appName) {
			aliasesToDelete = append(aliasesToDelete, dnsAlias)
		}
	}
	return aliasesToCreate, aliasesToUpdate, aliasesToDelete, errs
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

func (app *ApplicationConfig) getPortForDNSAlias(dnsAlias radixv1.DNSAlias) (int32, error) {
	component, componentFound := slice.FindFirst(app.config.Spec.Components, func(c radixv1.RadixComponent) bool {
		return c.Name == dnsAlias.Component
	})
	if !componentFound {
		return 0, fmt.Errorf("component %s does not exist in the application %s", dnsAlias.Component, app.config.GetName())
	}
	if !component.GetEnabledForEnvironment(dnsAlias.Environment) {
		return 0, fmt.Errorf("component %s is not enabled for the environment %s in the application %s", dnsAlias.Component, dnsAlias.Environment, app.config.GetName())
	}
	componentPublicPort := getComponentPublicPort(&component)
	if componentPublicPort == nil {
		return 0, fmt.Errorf("component %s does not have public port in the application %s", dnsAlias.Component, app.config.GetName())
	}
	return componentPublicPort.Port, nil
}
