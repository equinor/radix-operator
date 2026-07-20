package models

import (
	applicationModels "github.com/equinor/radix-operator/api-server/api/applications/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// BuildApplicationRegistration builds an ApplicationRegistration model.
func BuildApplicationRegistration(rr *radixv1.RadixRegistration) *applicationModels.ApplicationRegistration {
	appReg := applicationModels.NewApplicationRegistrationBuilder().WithRadixRegistration(rr).Build()
	appReg.HasFederatedCredentialAnnotation = hasFederatedCredentialAnnotation(rr.Annotations)
	return &appReg
}

func hasFederatedCredentialAnnotation(annotations map[string]string) bool {
	_, ok := annotations[kube.RadixFederatedCredentialsMigratedAnnotation]
	return ok
}
