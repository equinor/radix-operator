package utils

import (
	"encoding/json"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

//GetStorageClassMap Get map from StorageClassList with name as key
func GetStorageClassMap(scList *[]storagev1.StorageClass) map[string]*storagev1.StorageClass {
	scMap := make(map[string]*storagev1.StorageClass)
	for _, sc := range *scList {
		sc := sc
		scMap[sc.Name] = &sc
	}
	return scMap
}

//EqualStorageClassLists Compare two StorageClass lists
func EqualStorageClassLists(list1 *[]storagev1.StorageClass, list2 *[]storagev1.StorageClass) (bool, error) {
	if len(*list1) != len(*list2) {
		return false, nil
	}
	map1 := GetStorageClassMap(list1)
	map2 := GetStorageClassMap(list2)
	processedPvcNameSet := make(map[string]bool)
	equals, err := equalStorageClassMaps(map1, map2, processedPvcNameSet)
	if err != nil {
		return false, err
	}
	if !equals {
		return false, err
	}
	return equalStorageClassMaps(map2, map1, processedPvcNameSet)
}

func equalStorageClassMaps(map1 map[string]*storagev1.StorageClass, map2 map[string]*storagev1.StorageClass, processedKeySet map[string]bool) (bool, error) {
	for pvcName, pvc1 := range map1 {
		if _, ok := processedKeySet[pvcName]; ok {
			continue
		}
		pvc2, ok := map2[pvcName]
		if !ok {
			return false, nil
		}
		if equal, err := EqualStorageClasses(pvc1, pvc2); err != nil || !equal {
			return false, err
		}
		processedKeySet[pvcName] = false
	}
	return true, nil
}

//EqualStorageClasses Compare two StorageClass pointers
func EqualStorageClasses(sc1 *storagev1.StorageClass, sc2 *storagev1.StorageClass) (bool, error) {
	sc1Copy := sc1.DeepCopy()
	sc1Copy.ObjectMeta.ManagedFields = nil //HACK: to avoid ManagedFields comparison
	sc2Copy := sc2.DeepCopy()
	sc2Copy.ObjectMeta.ManagedFields = nil //HACK: to avoid ManagedFields comparison
	json1, err := json.Marshal(sc1Copy)
	if err != nil {
		return false, err
	}
	json2, err := json.Marshal(sc2Copy)
	if err != nil {
		return false, err
	}
	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(json1, json2, storagev1.StorageClass{})
	if err != nil {
		return false, err
	}
	return kube.IsEmptyPatch(patchBytes), nil
}
