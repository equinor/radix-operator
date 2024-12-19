package persistentvolume

import (
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
)

// GetPersistentVolumeClaimMap Get map from PersistentVolumeClaim with name as key
func GetPersistentVolumeClaimMap(pvcList *[]corev1.PersistentVolumeClaim) map[string]*corev1.PersistentVolumeClaim {
	pvcMap := make(map[string]*corev1.PersistentVolumeClaim)
	for _, pvc := range *pvcList {
		pvc := pvc
		name := pvc.Name
		pvcMap[name] = &pvc
	}
	return pvcMap
}

// EqualPersistentVolumeClaims Compare two PersistentVolumeClaims
func EqualPersistentVolumeClaims(pvc1, pvc2 *corev1.PersistentVolumeClaim) bool {
	if pvc1 == nil || pvc2 == nil {
		return false
	}
	if pvc1.GetNamespace() != pvc2.GetNamespace() {
		return false
	}
	if !utils.EqualStringMaps(getAnnotations(pvc1), getAnnotations(pvc2)) {
		return false
	}
	if !utils.EqualStringMaps(pvc1.GetLabels(), pvc2.GetLabels()) {
		return false
	}
	// ignore pvc1.Spec.StorageClassName != pvc2.Spec.StorageClassName for transition period
	pvc1StorageCapacity, existsPvc1StorageCapacity := pvc1.Spec.Resources.Requests[corev1.ResourceStorage]
	pvc2StorageCapacity, existsPvc2StorageCapacity := pvc2.Spec.Resources.Requests[corev1.ResourceStorage]
	if (existsPvc1StorageCapacity != existsPvc2StorageCapacity) ||
		(existsPvc1StorageCapacity && pvc1StorageCapacity.Cmp(pvc2StorageCapacity) != 0) {
		return false
	}
	if len(pvc1.Spec.AccessModes) != len(pvc2.Spec.AccessModes) {
		return false
	}
	if len(pvc1.Spec.AccessModes) == 1 && pvc1.Spec.AccessModes[0] != pvc2.Spec.AccessModes[0] {
		return false
	}
	volumeMode1 := pointers.Val(pvc1.Spec.VolumeMode)
	volumeMode2 := pointers.Val(pvc2.Spec.VolumeMode)
	if volumeMode1 != volumeMode2 {
		return false
	}
	return true
}

func getAnnotations(pvc *corev1.PersistentVolumeClaim) map[string]string {
	annotations := make(map[string]string)
	for key, value := range pvc.GetAnnotations() {
		if key == "pv.kubernetes.io/bind-completed" ||
			key == "pv.kubernetes.io/bound-by-controller" ||
			key == "volume.beta.kubernetes.io/storage-provisioner" ||
			key == "volume.kubernetes.io/storage-provisioner" {
			continue // ignore automatically added annotation(s)
		}
		annotations[key] = value
	}
	return annotations
}
