package utils

import (
	"encoding/json"
	"fmt"
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
func EqualStorageClassLists(scList1, scList2 *[]storagev1.StorageClass) (bool, error) {
	if len(*scList1) != len(*scList2) {
		return false, fmt.Errorf("different StorageClass list sizes: %v, %v", len(*scList1), len(*scList2))
	}
	scMap1 := GetStorageClassMap(scList1)
	scMap2 := GetStorageClassMap(scList2)
	for scName, sc1 := range scMap1 {
		sc2, ok := scMap2[scName]
		if !ok {
			return false, fmt.Errorf("StorageClass not found by name '%s' in second list", scName)
		}
		if equal, err := EqualStorageClasses(sc1, sc2); err != nil || !equal {
			return false, err
		}
	}
	return true, nil
}

//EqualStorageClasses Compare two StorageClass pointers
func EqualStorageClasses(sc1, sc2 *storagev1.StorageClass) (bool, error) {
	sc1Copy, labels1, params1, mountOptions1 := getStorageClassCopyWithCollections(sc1)
	sc2Copy, labels2, params2, mountOptions2 := getStorageClassCopyWithCollections(sc2)
	patchBytes, err := getStorageClassesPatch(sc1Copy, sc2Copy)
	if err != nil {
		return false, err
	}
	if !EqualStringMaps(labels1, labels2) {
		return false, nil //StorageClasses labels are not equal
	}
	if !EqualStringMaps(params1, params2) {
		return false, nil //StorageClasses parameters are not equal
	}
	if !EqualStringLists(mountOptions1, mountOptions2) {
		return false, nil //StorageClass-es MountOptions are not equal
	}
	if !kube.IsEmptyPatch(patchBytes) {
		return false, nil //StorageClasses properties are not equal
	}
	return true, nil
}

func getStorageClassCopyWithCollections(sc *storagev1.StorageClass) (*storagev1.StorageClass, map[string]string, map[string]string, []string) {
	scCopy := sc.DeepCopy()
	scCopy.ObjectMeta.ManagedFields = nil //HACK: to avoid ManagedFields comparison
	//to avoid label order variations
	labels := scCopy.ObjectMeta.Labels
	scCopy.ObjectMeta.Labels = map[string]string{}
	//to avoid Parameters order variations
	scParams := scCopy.Parameters
	scCopy.Parameters = map[string]string{}
	//to avoid MountOptions order variations
	scMountOptions := scCopy.MountOptions
	scCopy.MountOptions = []string{}
	return scCopy, labels, scParams, scMountOptions
}

func getStorageClassesPatch(sc1, sc2 *storagev1.StorageClass) ([]byte, error) {
	json1, err := json.Marshal(sc1)
	if err != nil {
		return []byte{}, err
	}
	json2, err := json.Marshal(sc2)
	if err != nil {
		return []byte{}, err
	}
	return strategicpatch.CreateTwoWayMergePatch(json1, json2, storagev1.StorageClass{})
}
