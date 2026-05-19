package kubequery

import (
	"context"

	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetDNSAliases returns all RadixDNSAliases for the specified application.
func GetDNSAliases(ctx context.Context, client radixclient.Interface, radixApplication *radixv1.RadixApplication) []radixv1.RadixDNSAlias {
	if radixApplication == nil {
		return nil
	}

	return slice.Reduce(radixApplication.Spec.DNSAlias, []radixv1.RadixDNSAlias{}, func(acc []radixv1.RadixDNSAlias, dnsAlias radixv1.DNSAlias) []radixv1.RadixDNSAlias {
		radixDNSAlias, err := client.RadixV1().RadixDNSAliases().Get(ctx, dnsAlias.Alias, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) && !errors.IsForbidden(err) {
				log.Ctx(ctx).Error().Err(err).Msgf("failed to get DNS alias %s", dnsAlias.Alias)
			}
			return acc
		}
		return append(acc, *radixDNSAlias)
	})
}
