package application

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetOwnerReferenceOfRegistration Gets owner reference given registration. Resources that an RR owns
func (app Application) GetOwnerReferenceOfRegistration() []metav1.OwnerReference {
	trueVar := true
	return []metav1.OwnerReference{
		metav1.OwnerReference{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixRegistration",
			Name:       app.registration.Name,
			UID:        app.registration.UID,
			Controller: &trueVar,
		},
	}
}

// GetOwnerReferenceOfRegistrationWithName Gets owner reference given registration with custom name. Resources that an RR owns
func (app Application) GetOwnerReferenceOfRegistrationWithName(name string) []metav1.OwnerReference {
	trueVar := true
	return []metav1.OwnerReference{
		metav1.OwnerReference{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixRegistration",
			Name:       name,
			UID:        app.registration.UID,
			Controller: &trueVar,
		},
	}
}
