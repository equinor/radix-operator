package utils

import (
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
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

// EqualPersistentVolumeLists Compare two PersistentVolume lists
func EqualPersistentVolumeLists(list1, list2 *[]corev1.PersistentVolume) (bool, error) {
	if len(*list1) != len(*list2) {
		return false, fmt.Errorf("different PersistentVolume list sizes: %v, %v", len(*list1), len(*list2))
	}
	map1 := GetPersistentVolumeMap(list1)
	map2 := GetPersistentVolumeMap(list2)
	for pvName, pv1 := range map1 {
		pv2, ok := map2[pvName]
		if !ok {
			return false, fmt.Errorf("PersistentVolume not found by name %s in second list", pvName)
		}
		if equal, err := EqualPersistentVolumes(pv1, pv2); err != nil || !equal {
			return false, err
		}
	}
	return true, nil
}

// EqualPersistentVolumes Compare two PersistentVolumes
func EqualPersistentVolumes(pv1, pv2 *corev1.PersistentVolume) (bool, error) {
	pv1Copy, labels1, attribs1, mountOptions1 := getPersistentVolumeCopyWithCollections(pv1)
	pv2Copy, labels2, attribs2, mountOptions2 := getPersistentVolumeCopyWithCollections(pv2)
	patchBytes, err := getPersistentVolumePatch(pv1Copy, pv2Copy)
	if err != nil {
		return false, err
	}
	if !EqualStringMaps(labels1, labels2) {
		return false, nil // PersistentVolume labels are not equal
	}
	if !EqualStringMaps(attribs1, attribs2) {
		return false, nil // PersistentVolume parameters are not equal
	}
	if !EqualStringLists(mountOptions1, mountOptions2) {
		return false, nil // PersistentVolume-es MountOptions are not equal
	}
	if !kube.IsEmptyPatch(patchBytes) {
		return false, nil // PersistentVolume properties are not equal
	}
	return true, nil
}

func getPersistentVolumeCopyWithCollections(pv *corev1.PersistentVolume) (*corev1.PersistentVolume, map[string]string, map[string]string, []string) {
	pvCopy := pv.DeepCopy()
	pvCopy.ObjectMeta.ManagedFields = nil // HACK: to avoid ManagedFields comparison
	// to avoid label order variations
	labels := pvCopy.ObjectMeta.Labels
	pvCopy.ObjectMeta.Labels = map[string]string{}
	// to avoid Attribs order variations
	pvAttribs := pvCopy.Spec.CSI.VolumeAttributes
	pvCopy.Spec.CSI.VolumeAttributes = map[string]string{}
	// to avoid MountOptions order variations
	mountOptions := pvCopy.Spec.MountOptions
	pvCopy.Spec.MountOptions = []string{}
	return pvCopy, labels, pvAttribs, mountOptions
}

func getPersistentVolumePatch(pv1, pv2 *corev1.PersistentVolume) ([]byte, error) {
	json1, err := json.Marshal(pv1)
	if err != nil {
		return []byte{}, err
	}
	json2, err := json.Marshal(pv2)
	if err != nil {
		return []byte{}, err
	}
	return strategicpatch.CreateTwoWayMergePatch(json1, json2, corev1.PersistentVolume{})
}
