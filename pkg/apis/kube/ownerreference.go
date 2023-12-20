package kube

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func mergeOwnerReferences(ownerReferences1 []metav1.OwnerReference, ownerReferences2 []metav1.OwnerReference) []metav1.OwnerReference {
	uidMap := make(map[types.UID]bool)
	var mergedList []metav1.OwnerReference
	for _, ownerReference := range append(ownerReferences1, ownerReferences2...) {
		if len(ownerReference.UID) > 0 && !uidMap[ownerReference.UID] {
			uidMap[ownerReference.UID] = true
			mergedList = append(mergedList, ownerReference)
		}
	}
	return mergedList
}
