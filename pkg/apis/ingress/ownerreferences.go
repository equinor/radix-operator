package ingress

import (
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetOwnerReferenceOfIngress Get an Ingress as an owner reference
func GetOwnerReferenceOfIngress(ingress *networkingv1.Ingress) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: networkingv1.SchemeGroupVersion.Identifier(),
			Kind:       k8s.KindIngress,
			Name:       ingress.Name,
			UID:        ingress.UID,
			Controller: pointers.Ptr(true),
		},
	}
}
