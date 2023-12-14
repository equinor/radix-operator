package internal

import (
	"github.com/equinor/radix-common/utils/pointers"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetOwnerReferences Gets RadixDNSAlias as an owner reference
func GetOwnerReferences(radixDNSAlias *radixv1.RadixDNSAlias) []metav1.OwnerReference {
	return []metav1.OwnerReference{{
		APIVersion: radixv1.SchemeGroupVersion.Identifier(),
		Kind:       radixv1.KindRadixDNSAlias,
		Name:       radixDNSAlias.Name,
		UID:        radixDNSAlias.UID,
		Controller: pointers.Ptr(true),
	},
	}
}
