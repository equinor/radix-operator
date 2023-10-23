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

const (
	dnsAliasIngressNameTemplate = "%s.%s.custom-domain" // <component-name>.<dns0alias-domain>.custom-domain
)

func (app *ApplicationConfig) createOrUpdateDNSAliases() error {
	appName := app.registration.Name
	var errs []error
	ctx := context.Background()
	radixDNSAliasesMap, err := app.getRadixDNSAliasesMap(ctx, appName)
	if err != nil {
		return err
	}
	for _, dnsAlias := range app.config.Spec.DNSAlias {
		if radixDNSAlias, ok := radixDNSAliasesMap[dnsAlias.Domain]; ok {
			if !strings.EqualFold(appName, radixDNSAlias.Spec.AppName) {
				return fmt.Errorf("existing DNS alias domain %s is used by the application %s", dnsAlias.Domain, radixDNSAlias.Spec.AppName)
			}
			if strings.EqualFold(dnsAlias.Environment, radixDNSAlias.Spec.Environment) &&
				strings.EqualFold(dnsAlias.Component, radixDNSAlias.Spec.Component) {
				continue // no changes
			}
			if err = app.updateRadixDNSAlias(radixDNSAlias, dnsAlias, err, ctx); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if err = app.createRadixDNSAlias(dnsAlias, appName, err, ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Concat(errs)
}

func (app *ApplicationConfig) updateRadixDNSAlias(radixDNSAlias *radixv1.RadixDNSAlias, dnsAlias radixv1.DNSAlias, err error, ctx context.Context) error {
	updatedRadixDNSAlias := radixDNSAlias.DeepCopy()
	updatedRadixDNSAlias.Spec.Environment = dnsAlias.Environment
	updatedRadixDNSAlias.Spec.Component = dnsAlias.Component
	_, err = app.radixclient.RadixV1().RadixDNSAliases().Update(ctx, updatedRadixDNSAlias, metav1.UpdateOptions{})
	return err
}

func (app *ApplicationConfig) createRadixDNSAlias(dnsAlias radixv1.DNSAlias, appName string, err error, ctx context.Context) error {
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
	_, err = app.radixclient.RadixV1().RadixDNSAliases().Create(ctx, &radixDNSAlias, metav1.CreateOptions{})
	return err
}

func (app *ApplicationConfig) getRadixDNSAliasesMap(ctx context.Context, appName string) (map[string]*radixv1.RadixDNSAlias, error) {
	dnsAliasList, err := app.radixclient.RadixV1().RadixDNSAliases().List(context.Background(), metav1.ListOptions{LabelSelector: labels.ForApplicationName(appName).String()})
	if err != nil {
		return nil, err
	}
	return slice.Reduce(dnsAliasList.Items, make(map[string]*radixv1.RadixDNSAlias), func(acc map[string]*radixv1.RadixDNSAlias, dnsAlias radixv1.RadixDNSAlias) map[string]*radixv1.RadixDNSAlias {
		acc[dnsAlias.Name] = &dnsAlias
		return acc
	}), err
}
