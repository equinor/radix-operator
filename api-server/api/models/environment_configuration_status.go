package models

import (
	environmentModels "github.com/equinor/radix-operator/api-server/api/environments/models"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

func getEnvironmentConfigurationStatus(re *radixv1.RadixEnvironment) environmentModels.ConfigurationStatus {
	switch {
	case re == nil:
		return environmentModels.Pending
	case re.Status.Orphaned || re.Status.OrphanedTimestamp != nil:
		return environmentModels.Orphan
	case re.Status.Reconciled.IsZero():
		return environmentModels.Pending
	default:
		return environmentModels.Consistent
	}
}
