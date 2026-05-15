package models

import (
	applicationModels "github.com/equinor/radix-operator/api-server/api/applications/models"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// BuildApplicationRegistration builds an ApplicationRegistration model.
func BuildApplicationRegistration(rr *radixv1.RadixRegistration) *applicationModels.ApplicationRegistration {
	appReg := applicationModels.NewApplicationRegistrationBuilder().WithRadixRegistration(rr).Build()
	return &appReg
}
