package applicationconfig

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (app *ApplicationConfig) syncDNSAliases(ctx context.Context) error {
	existingAliases, err := kube.GetRadixDNSAliasMap(ctx, app.radixclient)
	if err != nil {
		return err
	}
	aliasesToCreate, aliasesToUpdate, aliasesToDelete, err := app.getDNSAliasesToSync(existingAliases)
	if err != nil {
		return err
	}

	// first - delete
	for _, dnsAlias := range aliasesToDelete {
		if err := app.radixclient.RadixV1().RadixDNSAliases().Delete(ctx, dnsAlias.Name, metav1.DeleteOptions{}); err != nil {
			return fmt.Errorf("failed to delete RadixDNSAlias %s: %w", dnsAlias.Name, err)
		}
	}
	// then - update
	for _, dnsAlias := range aliasesToUpdate {
		existingDnsAlias := existingAliases[dnsAlias.Name]
		if cmp.Equal(existingDnsAlias, dnsAlias, cmpopts.EquateEmpty()) {
			continue
		}
		if _, err := app.radixclient.RadixV1().RadixDNSAliases().Update(ctx, dnsAlias, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update RadixDNSAlias %s: %w", dnsAlias.Name, err)
		}
	}
	// then - create
	for _, dnsAlias := range aliasesToCreate {
		if _, err := app.radixclient.RadixV1().RadixDNSAliases().Create(ctx, dnsAlias, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create RadixDNSAlias %s: %w", dnsAlias.Name, err)
		}
	}

	if len(app.config.Spec.DNSAlias) == 0 {
		if err := app.garbageCollectAccessToDNSAliases(ctx); err != nil {
			return err
		}
	} else {
		if err := app.grantAccessToDNSAliases(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (app *ApplicationConfig) getDNSAliasesToSync(existingAliases map[string]*radixv1.RadixDNSAlias) ([]*radixv1.RadixDNSAlias, []*radixv1.RadixDNSAlias, []*radixv1.RadixDNSAlias, error) {
	var aliasesToCreate, aliasesToUpdate, aliasesToDelete []*radixv1.RadixDNSAlias
	processedAliases := make(map[string]any)
	appName := app.registration.Name
	var errs []error
	for _, dnsAlias := range app.config.Spec.DNSAlias {
		if existingAlias, exists := existingAliases[dnsAlias.Alias]; exists {
			if existingAlias.Spec.AppName != appName {
				errs = append(errs, fmt.Errorf("failed to process dns alias %s: %w", dnsAlias.Alias, ErrDNSAliasUsedByOtherApplication))
				continue
			}
			if existingAlias.Spec.Environment == dnsAlias.Environment {
				updatingRadixDNSAlias := existingAlias.DeepCopy()
				updatingRadixDNSAlias.Spec.Component = dnsAlias.Component

				// We must reset existing OwnerReferences since it was previously and incorrectly set to RadixApplication as controller
				updatingRadixDNSAlias.OwnerReferences = nil
				if err := controllerutil.SetControllerReference(app.registration, updatingRadixDNSAlias, scheme); err != nil {
					return nil, nil, nil, fmt.Errorf("failed to set ownerreference: %w", err)
				}

				aliasesToUpdate = append(aliasesToUpdate, updatingRadixDNSAlias)
				processedAliases[dnsAlias.Alias] = true
				continue
			}
		}
		rda, err := app.buildRadixDNSAlias(appName, dnsAlias)
		if err != nil {
			return nil, nil, nil, err
		}
		aliasesToCreate = append(aliasesToCreate, rda) // new alias or an alias with changed environment or component
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

func (app *ApplicationConfig) buildRadixDNSAlias(appName string, dnsAlias radixv1.DNSAlias) (*radixv1.RadixDNSAlias, error) {
	rda := &radixv1.RadixDNSAlias{
		ObjectMeta: metav1.ObjectMeta{
			Name:   dnsAlias.Alias,
			Labels: labels.Merge(labels.ForApplicationName(appName), labels.ForComponentName(dnsAlias.Component), labels.ForEnvironmentName(dnsAlias.Environment)),
		},
		Spec: radixv1.RadixDNSAliasSpec{
			AppName:     appName,
			Environment: dnsAlias.Environment,
			Component:   dnsAlias.Component,
		},
	}

	if err := controllerutil.SetControllerReference(app.registration, rda, scheme); err != nil {
		return nil, fmt.Errorf("failed to set ownerreference: %w", err)
	}

	return rda, nil
}
