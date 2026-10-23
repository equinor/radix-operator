package internal

import (
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
)

// BuildRadixDNSAlias Build a RadixDNSAlias
func BuildRadixDNSAlias(appName, componentName, envName, alias string) *radixv1.RadixDNSAlias {
	return &radixv1.RadixDNSAlias{
		Name:   alias,
		Labels: labels.Merge(labels.ForApplicationName(appName), labels.ForComponentName(componentName)),
		Spec: radixv1.RadixDNSAliasSpec{
			AppName:     appName,
			Environment: envName,
			Component:   componentName,
		}}
}

// GetDNSAliasHost Gets DNS alias host.
// Example for the alias "my-app" and the cluster "Playground": my-app.playground.radix.equinor.com
func GetDNSAliasHost(alias string, dnsZone string) string {
	return fmt.Sprintf("%s.%s", alias, dnsZone)
}
