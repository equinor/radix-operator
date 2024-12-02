package internal

import (
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
)

// GetPersistentVolumeMap Get map from PersistentVolumeList with name as key
func GetPersistentVolumeMap(pvList *[]corev1.PersistentVolume) map[string]*corev1.PersistentVolume {
	pvMap := make(map[string]*corev1.PersistentVolume)
	for _, pv := range *pvList {
		pv := pv
		pvMap[pv.Name] = &pv
	}
	return pvMap
}

// EqualPersistentVolumes Compare two PersistentVolumes
func EqualPersistentVolumes(pv1, pv2 *corev1.PersistentVolume) (bool, error) {
	// Ignore for now, due to during transition period this would affect existing volume mounts, managed by a provisioner
	// if !utils.EqualStringMaps(pv1.GetLabels(), pv2.GetLabels()) {
	// 	return false, nil
	// }
	if !utils.EqualStringMaps(getAnnotations(pv1), getAnnotations(pv2)) {
		return false, nil
	}
	if !utils.EqualStringMaps(pv1.Spec.CSI.VolumeAttributes, pv2.Spec.CSI.VolumeAttributes) {
		return false, nil
	}
	if !utils.EqualStringMaps(getMountOptionsMap(pv1.Spec.MountOptions), getMountOptionsMap(pv2.Spec.MountOptions)) {
		return false, nil
	}
	if pv1.Spec.StorageClassName != pv2.Spec.StorageClassName ||
		pv1.Spec.Capacity[corev1.ResourceStorage] != pv2.Spec.Capacity[corev1.ResourceStorage] ||
		len(pv1.Spec.AccessModes) != len(pv2.Spec.AccessModes) ||
		(len(pv1.Spec.AccessModes) != 1 && pv1.Spec.AccessModes[0] != pv2.Spec.AccessModes[0]) ||
		pv1.Spec.CSI.Driver != pv2.Spec.CSI.Driver {
		return false, nil
	}
	if pv1.Spec.CSI.NodeStageSecretRef != nil {
		if pv2.Spec.CSI.NodeStageSecretRef == nil || pv1.Spec.CSI.NodeStageSecretRef.Name != pv2.Spec.CSI.NodeStageSecretRef.Name {
			return false, nil
		}
	} else if pv2.Spec.CSI.NodeStageSecretRef != nil {
		return false, nil
	}
	if pv1.Spec.ClaimRef != nil {
		if pv2.Spec.ClaimRef == nil || pv1.Spec.ClaimRef.Name != pv2.Spec.ClaimRef.Name {
			return false, nil
		}
	} else if pv2.Spec.ClaimRef != nil {
		return false, nil
	}
	return true, nil
}

func getAnnotations(pv *corev1.PersistentVolume) map[string]string {
	annotations := make(map[string]string)
	for key, value := range pv.GetAnnotations() {
		if key == "kubectl.kubernetes.io/last-applied-configuration" {
			continue // ignore automatically added annotation(s)
		}
		annotations[key] = value
	}
	return annotations
}

func getMountOptionsMap(mountOptions []string) map[string]string {
	return slice.Reduce(mountOptions, make(map[string]string), func(acc map[string]string, item string) map[string]string {
		if len(item) == 0 {
			return acc
		}
		itemParts := strings.Split(item, "=")
		key, value := "", ""
		if len(itemParts) > 0 {
			key = itemParts[0]
		}
		if key == "--tmp-path" {
			return acc // ignore tmp-path, which eventually can be introduced
		}
		if len(itemParts) > 1 {
			value = itemParts[1]
		}
		if len(key) > 0 {
			acc[key] = value
		}
		return acc
	})
}
