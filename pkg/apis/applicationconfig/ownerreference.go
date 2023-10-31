package applicationconfig

import (
	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/radix"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getOwnerReferenceOfApplication(radixApplication *radixv1.RadixApplication) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: radix.APIVersion,
		Kind:       radix.KindRadixApplication,
		Name:       radixApplication.Name,
		UID:        radixApplication.UID,
		Controller: utils.BoolPtr(true),
	}
}
