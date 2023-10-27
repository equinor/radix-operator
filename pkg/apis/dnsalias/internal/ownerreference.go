package internal

import (
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/radix"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetOwnerReference Gets RadixDNSAlias as an owner reference
func GetOwnerReference(alias *radixv1.RadixDNSAlias) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: radix.APIVersion,
		Kind:       radix.KindRadixDNSAlias,
		Name:       alias.Name,
		UID:        alias.UID,
		Controller: pointers.Ptr(true),
	}
}
