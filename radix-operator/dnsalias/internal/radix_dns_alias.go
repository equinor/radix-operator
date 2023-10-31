package internal

import (
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildRadixDNSAlias Build a RadixDNSAlias
func BuildRadixDNSAlias(appName, componentName, envName, domain string) *radixv1.RadixDNSAlias {
	return &radixv1.RadixDNSAlias{
		ObjectMeta: metav1.ObjectMeta{
			Name:   domain,
			Labels: labels.Merge(labels.ForApplicationName(appName), labels.ForComponentName(componentName)),
		},
		Spec: radixv1.RadixDNSAliasSpec{
			AppName:     appName,
			Environment: envName,
			Component:   componentName,
		}}
}

// GetDNSAliasHost Gets DNS alias domain host.
// Example for the domain "my-app" and the cluster "Playground": my-app.playground.radix.equinor.com
func GetDNSAliasHost(domain string, dnsZone string) string {
	return fmt.Sprintf("%s.%s", domain, dnsZone)
}
