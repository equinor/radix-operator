package deployment

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetOwnerReferenceOfDeploymentWithName Gets owner reference given deployment with custom name. Resources that an RD owns
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
