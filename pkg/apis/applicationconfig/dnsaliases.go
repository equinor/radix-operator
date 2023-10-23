package applicationconfig

import (
	"context"
	"fmt"
	"strings"

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
	for _, dnsAlias := range app.config.Spec.DNSAlias {
		if radixDNSAlias, ok := radixDNSAliasesMap[dnsAlias.Domain]; ok {
			if !strings.EqualFold(appName, radixDNSAlias.Spec.AppName) {
				return fmt.Errorf("existing DNS alias domain %s is used by the application %s", dnsAlias.Domain, radixDNSAlias.Spec.AppName)
			}
			if strings.EqualFold(dnsAlias.Environment, radixDNSAlias.Spec.Environment) &&
				strings.EqualFold(dnsAlias.Component, radixDNSAlias.Spec.Component) {

			}
		}
	}
	fmt.Println(len(radixDNSAliasesMap))
	return nil
}

func (app *ApplicationConfig) getRadixDNSAliasesMap(appName string) (map[string]*radixv1.RadixDNSAlias, error) {
	dnsAliasList, err := app.radixclient.RadixV1().RadixDNSAliases().List(context.Background(), metav1.ListOptions{LabelSelector: labels.ForApplicationName(appName).String()})
	if err != nil {
		return nil, err
	}
	return slice.Reduce(dnsAliasList.Items, make(map[string]*radixv1.RadixDNSAlias), func(acc map[string]*radixv1.RadixDNSAlias, dnsAlias radixv1.RadixDNSAlias) map[string]*radixv1.RadixDNSAlias {
		acc[dnsAlias.Name] = &dnsAlias
		return acc
	}), err
}
