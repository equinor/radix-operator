package deployment

import (
	"github.com/equinor/radix-common/utils"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

func getOwnerReferencesOfDeployment(radixDeployment *v1.RadixDeployment) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		getOwnerReferenceOfDeployment(radixDeployment),
	}
}

func getOwnerReferenceOfDeployment(radixDeployment *v1.RadixDeployment) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "radix.equinor.com/v1", //need to hardcode these values for now - seems they are missing from the CRD in k8s 1.8
		Kind:       "RadixDeployment",
		Name:       radixDeployment.Name,
		UID:        radixDeployment.UID,
		Controller: utils.BoolPtr(true),
	}
}

func getOwnerReferenceOfSecretProviderClass(secretProviderClass *secretsstorev1.SecretProviderClass) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "secrets-store.csi.x-k8s.io/v1",
		Kind:       "SecretProviderClass",
		Name:       secretProviderClass.Name,
		UID:        secretProviderClass.UID,
		//Controller is not set due too only one OwnerReference's controller can be set as `true`
	}
}

func isOwnerReference(targetMeta, ownerMeta metav1.ObjectMeta) bool {
	for _, targetOwnerReference := range targetMeta.OwnerReferences {
		if targetOwnerReference.Name == ownerMeta.Name && targetOwnerReference.UID == ownerMeta.UID {
			return true
		}
	}
	return false
}
