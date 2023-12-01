package application

import (
	"github.com/equinor/radix-operator/pkg/apis/radix"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetOwnerReference Gets owner reference of application
func (app Application) getOwnerReference() []metav1.OwnerReference {
	return GetOwnerReferenceOfRegistration(app.registration)
}

// GetOwnerReferenceOfRegistration Gets owner reference given registration. Resources that an RR owns
func GetOwnerReferenceOfRegistration(registration *radixv1.RadixRegistration) []metav1.OwnerReference {
	trueVar := true
	return []metav1.OwnerReference{
		{
			APIVersion: radix.APIVersion,
			Kind:       radix.KindRadixRegistration,
			Name:       registration.Name,
			UID:        registration.UID,
			Controller: &trueVar,
		},
	}
}
