package models

import (
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/api-server/api/applications/models"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// BuildDNSExternalAliases builds DNSExternalAliases model list
func BuildDNSExternalAliases(rdList []radixv1.RadixDeployment) []models.DNSExternalAlias {
	return slice.Reduce(rdList, make([]models.DNSExternalAlias, 0), func(acc []models.DNSExternalAlias, radixDeployment radixv1.RadixDeployment) []models.DNSExternalAlias {
		if !radixDeployment.Status.ActiveTo.IsZero() {
			return acc
		}
		for _, deployComponent := range radixDeployment.Spec.Components {
			acc = append(acc, slice.Map(deployComponent.ExternalDNS, func(externalAlias radixv1.RadixDeployExternalDNS) models.DNSExternalAlias {
				return models.DNSExternalAlias{
					URL:             externalAlias.FQDN,
					ComponentName:   deployComponent.GetName(),
					EnvironmentName: radixDeployment.Spec.Environment,
				}
			})...)
		}
		return acc
	})
}
