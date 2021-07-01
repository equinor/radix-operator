package utils

import (
	"encoding/json"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

//GetPersistentVolumeClaimMap Get map from PersistentVolumeClaim with name as key
func GetPersistentVolumeClaimMap(pvcList *[]corev1.PersistentVolumeClaim) map[string]*corev1.PersistentVolumeClaim {
	return getPersistentVolumeClaimMapManagingRandomPostfix(pvcList, true)
}

func getPersistentVolumeClaimMapManagingRandomPostfix(pvcList *[]corev1.PersistentVolumeClaim, ignoreRandomPostfixInName bool) map[string]*corev1.PersistentVolumeClaim {
	pvcMap := make(map[string]*corev1.PersistentVolumeClaim)
	for _, pvc := range *pvcList {
		pvc := pvc
		name := pvc.Name
		if ignoreRandomPostfixInName {
			name = ShortenString(name, 6)
		}
		pvcMap[name] = &pvc
	}
	return pvcMap
}

//EqualPvcLists Compare two PersistentVolumeClaim lists. When ignoreRandomPostfixInName=true - last 5 chars of the name are ignored
func EqualPvcLists(list1 *[]corev1.PersistentVolumeClaim, list2 *[]corev1.PersistentVolumeClaim, ignoreRandomPostfixInName bool) (bool, error) {
	if len(*list1) != len(*list2) {
		return false, nil
	}
	map1 := getPersistentVolumeClaimMapManagingRandomPostfix(list1, ignoreRandomPostfixInName)
	map2 := getPersistentVolumeClaimMapManagingRandomPostfix(list2, ignoreRandomPostfixInName)
	processedPvcNameSet := make(map[string]bool)
	equals, err := equalPvcMaps(map1, map2, processedPvcNameSet, ignoreRandomPostfixInName)
	if err != nil {
		return false, err
	}
	if !equals {
		return false, err
	}
	return equalPvcMaps(map2, map1, processedPvcNameSet, ignoreRandomPostfixInName)
}

func equalPvcMaps(map1 map[string]*corev1.PersistentVolumeClaim, map2 map[string]*corev1.PersistentVolumeClaim, processedKeySet map[string]bool, ignoreRandomPostfixInName bool) (bool, error) {
	for pvcName, pvc1 := range map1 {
		if _, ok := processedKeySet[pvcName]; ok {
			continue
		}
		pvc2, ok := map2[pvcName]
		if !ok {
			return false, nil
		}
		if equal, err := EqualPvcs(pvc1, pvc2, ignoreRandomPostfixInName); err != nil || !equal {
			return false, err
		}
		processedKeySet[pvcName] = false
	}
	return true, nil
}

//EqualPvcs Compare two PersistentVolumeClaim pointers
func EqualPvcs(pvc1 *corev1.PersistentVolumeClaim, pvc2 *corev1.PersistentVolumeClaim, ignoreRandomPostfixInName bool) (bool, error) {
	pvc1Copy := pvc1.DeepCopy()
	pvc1Copy.ObjectMeta.ManagedFields = nil //HACK: to avoid ManagedFields comparison
	pvc2Copy := pvc2.DeepCopy()
	pvc2Copy.ObjectMeta.ManagedFields = nil //HACK: to avoid ManagedFields comparison
	if ignoreRandomPostfixInName {
		pvc1Copy.ObjectMeta.Name = ShortenString(pvc1Copy.ObjectMeta.Name, 6)
		pvc2Copy.ObjectMeta.Name = ShortenString(pvc2Copy.ObjectMeta.Name, 6)
	}
	labels1 := pvc1Copy.ObjectMeta.Labels
	labels2 := pvc2Copy.ObjectMeta.Labels
	pvc1Copy.ObjectMeta.Labels = map[string]string{}
	pvc2Copy.ObjectMeta.Labels = map[string]string{}
	json1, err := json.Marshal(pvc1Copy)
	if err != nil {
		return false, err
	}
	json2, err := json.Marshal(pvc2Copy)
	if err != nil {
		return false, err
	}
	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(json1, json2, corev1.PersistentVolumeClaim{})
	if err != nil {
		return false, err
	}
	if !EqualStringMaps(labels1, labels2) { //to avoid label order variations
		return false, nil
	}
	return kube.IsEmptyPatch(patchBytes), nil
}
