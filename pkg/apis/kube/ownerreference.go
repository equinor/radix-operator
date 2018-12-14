package kube

import (
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetOwnerReferenceOfRegistration Gets owner reference given registration. Resources that an RR owns
// TODO : This should be moved closer to Application domain/package
func GetOwnerReferenceOfRegistration(radixRegistration *v1.RadixRegistration) []metav1.OwnerReference {
	trueVar := true
	return []metav1.OwnerReference{
		metav1.OwnerReference{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixRegistration",
			Name:       radixRegistration.Name,
			UID:        radixRegistration.UID,
			Controller: &trueVar,
		},
	}
}

// GetOwnerReferenceOfRegistrationWithName Gets owner reference given registration with custom name. Resources that an RR owns
// TODO : This should be moved closer to Application domain/package
func GetOwnerReferenceOfRegistrationWithName(name string, radixRegistration *v1.RadixRegistration) []metav1.OwnerReference {
	trueVar := true
	return []metav1.OwnerReference{
		metav1.OwnerReference{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixRegistration",
			Name:       name,
			UID:        radixRegistration.UID,
			Controller: &trueVar,
		},
	}
}

// GetOwnerReferenceOfDeploymentWithName Gets owner reference given deployment with custom name. Resources that an RD owns
// TODO : This should be moved closer to Deployment domain/package
func GetOwnerReferenceOfDeploymentWithName(name string, radixDeployment *v1.RadixDeployment) []metav1.OwnerReference {
	trueVar := true
	return []metav1.OwnerReference{
		metav1.OwnerReference{
			APIVersion: "radix.equinor.com/v1", //need to hardcode these values for now - seems they are missing from the CRD in k8s 1.8
			Kind:       "RadixDeployment",
			Name:       name,
			UID:        radixDeployment.UID,
			Controller: &trueVar,
		},
	}
}
