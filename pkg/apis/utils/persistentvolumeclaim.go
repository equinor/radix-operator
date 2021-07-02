package utils

import (
	"encoding/json"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

//GetPersistentVolumeClaimMap Get map from PersistentVolumeClaim with name as key
func GetPersistentVolumeClaimMap(pvcList *[]corev1.PersistentVolumeClaim) map[string]*corev1.PersistentVolumeClaim {
	return getPersistentVolumeClaimMap(pvcList, true)
}

func getPersistentVolumeClaimMap(pvcList *[]corev1.PersistentVolumeClaim, ignoreRandomPostfixInName bool) map[string]*corev1.PersistentVolumeClaim {
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
func EqualPvcLists(pvcList1, pvcList2 *[]corev1.PersistentVolumeClaim, ignoreRandomPostfixInName bool) (bool, error) {
	if len(*pvcList1) != len(*pvcList2) {
		return false, nil
	}
	pvcMap1 := getPersistentVolumeClaimMap(pvcList1, ignoreRandomPostfixInName)
	pvcMap2 := getPersistentVolumeClaimMap(pvcList2, ignoreRandomPostfixInName)
	for pvcName, pvc1 := range pvcMap1 {
		pvc2, ok := pvcMap2[pvcName]
		if !ok {
			return false, nil
		}
		if equal, err := EqualPvcs(pvc1, pvc2, ignoreRandomPostfixInName); err != nil || !equal {
			return false, err
		}
	}
	return true, nil
}

//EqualPvcs Compare two PersistentVolumeClaim pointers
func EqualPvcs(pvc1 *corev1.PersistentVolumeClaim, pvc2 *corev1.PersistentVolumeClaim, ignoreRandomPostfixInName bool) (bool, error) {
	pvc1Copy, labels1 := getPvcCopyWithLabels(pvc1, ignoreRandomPostfixInName)
	pvc2Copy, labels2 := getPvcCopyWithLabels(pvc2, ignoreRandomPostfixInName)
	patchBytes, err := getPvcPatch(pvc1Copy, pvc2Copy)
	if err != nil {
		return false, err
	}
	return EqualStringMaps(labels1, labels2) &&
		kube.IsEmptyPatch(patchBytes), nil
}

func getPvcCopyWithLabels(pvc *corev1.PersistentVolumeClaim, ignoreRandomPostfixInName bool) (*corev1.PersistentVolumeClaim, map[string]string) {
	pvcCopy := pvc.DeepCopy()
	pvcCopy.ObjectMeta.ManagedFields = nil //HACK: to avoid ManagedFields comparison
	if ignoreRandomPostfixInName {
		pvcCopy.ObjectMeta.Name = ShortenString(pvcCopy.ObjectMeta.Name, 6)
	}
	//to avoid label order variations
	labels := pvcCopy.ObjectMeta.Labels
	pvcCopy.ObjectMeta.Labels = map[string]string{}
	return pvcCopy, labels
}

func getPvcPatch(pvc1, pvc2 *corev1.PersistentVolumeClaim) ([]byte, error) {
	json1, err := json.Marshal(pvc1)
	if err != nil {
		return nil, err
	}
	json2, err := json.Marshal(pvc2)
	if err != nil {
		return nil, err
	}
	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(json1, json2, corev1.PersistentVolumeClaim{})
	if err != nil {
		return nil, err
	}
	return patchBytes, nil
}
