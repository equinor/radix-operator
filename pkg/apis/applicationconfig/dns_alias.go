package applicationconfig

import (
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app *ApplicationConfig) syncDNSAliases() error {
	existingAliases, err := kube.GetRadixDNSAliasMap(app.radixclient)
	if err != nil {
		return err
	}
	aliasesToCreate, aliasesToUpdate, aliasesToDelete, err := app.getDNSAliasesToSync(existingAliases)
	if err != nil {
		return err
	}
	var errs []error
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

func (app *ApplicationConfig) getDNSAliasesToSync(existingAliases map[string]*radixv1.RadixDNSAlias) ([]*radixv1.RadixDNSAlias, []*radixv1.RadixDNSAlias, []*radixv1.RadixDNSAlias, error) {
	var aliasesToCreate, aliasesToUpdate, aliasesToDelete []*radixv1.RadixDNSAlias
	processedAliases := make(map[string]any)
	appName := app.registration.Name
	var errs []error
	for _, dnsAlias := range app.config.Spec.DNSAlias {
		if existingAlias, exists := existingAliases[dnsAlias.Alias]; exists {
			if existingAlias.Spec.AppName != appName {
				errs = append(errs, fmt.Errorf("DNS Alias %s of the application %s is used by another application", appName, dnsAlias.Alias))
				continue
			}
			if existingAlias.Spec.Environment == dnsAlias.Environment {
				updatingRadixDNSAlias := existingAlias.DeepCopy()
				updatingRadixDNSAlias.Spec.Component = dnsAlias.Component
				aliasesToUpdate = append(aliasesToUpdate, updatingRadixDNSAlias)
				processedAliases[dnsAlias.Alias] = true
				continue
			}
		}
		aliasesToCreate = append(aliasesToCreate, app.buildRadixDNSAlias(appName, dnsAlias)) // new alias or an alias with changed environment or component
	}
	if len(errs) > 0 {
		return nil, nil, nil, stderrors.Join(errs...)
	}
	for aliasName, dnsAlias := range existingAliases {
		if _, ok := processedAliases[aliasName]; !ok && strings.EqualFold(dnsAlias.Spec.AppName, appName) {
			aliasesToDelete = append(aliasesToDelete, dnsAlias)
		}
	}
	return aliasesToCreate, aliasesToUpdate, aliasesToDelete, nil
}

func (app *ApplicationConfig) buildRadixDNSAlias(appName string, dnsAlias radixv1.DNSAlias) *radixv1.RadixDNSAlias {
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
		},
	}
}
