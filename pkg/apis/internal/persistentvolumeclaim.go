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
