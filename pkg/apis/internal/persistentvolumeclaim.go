package internal

import (
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
)

// GetPersistentVolumeClaimMap Get map from PersistentVolumeClaim with name as key
func GetPersistentVolumeClaimMap(pvcList *[]corev1.PersistentVolumeClaim, ignoreRandomPostfixInName bool) map[string]*corev1.PersistentVolumeClaim {
	pvcMap := make(map[string]*corev1.PersistentVolumeClaim)
	for _, pvc := range *pvcList {
		pvc := pvc
		name := pvc.Name
		if ignoreRandomPostfixInName {
			name = utils.ShortenString(name, 6)
		}
		pvcMap[name] = &pvc
	}
	return pvcMap
}

// EqualPersistentVolumeClaims Compare two PersistentVolumeClaims
func EqualPersistentVolumeClaims(pvc1, pvc2 *corev1.PersistentVolumeClaim) bool {
	if pvc1.GetNamespace() != pvc2.GetNamespace() {
		return false
	}
	if !utils.EqualStringMaps(pvc1.GetAnnotations(), pvc2.GetAnnotations()) {
		return false
	}
	if !utils.EqualStringMaps(pvc1.GetLabels(), pvc2.GetLabels()) {
		return false
	}
	// ignore pvc1.Spec.StorageClassName != pvc2.Spec.StorageClassName for transition period
	if pvc1.Spec.Resources.Requests[corev1.ResourceStorage] != pvc2.Spec.Resources.Requests[corev1.ResourceStorage] ||
		len(pvc1.Spec.AccessModes) != len(pvc2.Spec.AccessModes) ||
		(len(pvc1.Spec.AccessModes) != 1 && pvc1.Spec.AccessModes[0] != pvc2.Spec.AccessModes[0]) ||
		pvc1.Spec.VolumeMode != pvc2.Spec.VolumeMode {
		return false
	}
	return true
}
