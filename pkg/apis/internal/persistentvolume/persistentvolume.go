package persistentvolume

import (
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
)

// EqualPersistentVolumes Compare two PersistentVolumes
func EqualPersistentVolumes(pv1, pv2 *corev1.PersistentVolume) bool {
	if pv1 == nil || pv2 == nil {
		return false
	}
	// Ignore for now, due to during transition period this would affect existing volume mounts, managed by a provisioner
	// if !utils.EqualStringMaps(pv1.GetLabels(), pv2.GetLabels()) {
	// 	return false, nil
	// }
	if !utils.EqualStringMaps(getPvAnnotations(pv1), getPvAnnotations(pv2)) {
		return false
	}
	if !utils.EqualStringMaps(getVolumeAttributes(pv1), getVolumeAttributes(pv2)) {
		return false
	}
	if !utils.EqualStringMaps(getMountOptionsMap(pv1.Spec.MountOptions), getMountOptionsMap(pv2.Spec.MountOptions)) {
		return false
	}
	// ignore pv1.Spec.StorageClassName != pv2.Spec.StorageClassName for transition period
	if pv1.Spec.Capacity[corev1.ResourceStorage] != pv2.Spec.Capacity[corev1.ResourceStorage] ||
		len(pv1.Spec.AccessModes) != len(pv2.Spec.AccessModes) ||
		(len(pv1.Spec.AccessModes) != 1 && pv1.Spec.AccessModes[0] != pv2.Spec.AccessModes[0]) ||
		pv1.Spec.CSI.Driver != pv2.Spec.CSI.Driver {
		return false
	}
	if pv1.Spec.CSI.NodeStageSecretRef != nil {
		if pv2.Spec.CSI.NodeStageSecretRef == nil || pv1.Spec.CSI.NodeStageSecretRef.Name != pv2.Spec.CSI.NodeStageSecretRef.Name {
			return false
		}
	} else if pv2.Spec.CSI.NodeStageSecretRef != nil {
		return false
	}
	if pv1.Spec.ClaimRef != nil {
		if pv2.Spec.ClaimRef == nil || pv1.Spec.ClaimRef.Name != pv2.Spec.ClaimRef.Name {
			return false
		}
	} else if pv2.Spec.ClaimRef != nil {
		return false
	}
	return true
}

// EqualPersistentVolumesForTest Compare two PersistentVolumes for test
func EqualPersistentVolumesForTest(expectedPv, actualPv *corev1.PersistentVolume) bool {
	if expectedPv == nil || actualPv == nil {
		return false
	}
	// Ignore for now, due to during transition period this would affect existing volume mounts, managed by a provisioner
	// if !utils.EqualStringMaps(expectedPv.GetLabels(), actualPv.GetLabels()) {
	// 	return false, nil
	// }
	if !utils.EqualStringMaps(getPvAnnotations(expectedPv), getPvAnnotations(actualPv)) {
		return false
	}
	expectedClonedAttrs := cloneMap(expectedPv.Spec.CSI.VolumeAttributes, CsiVolumeMountAttributePvName, CsiVolumeMountAttributePvcName, CsiVolumeMountAttributeProvisionerIdentity)
	actualClonedAttrs := cloneMap(actualPv.Spec.CSI.VolumeAttributes, CsiVolumeMountAttributePvName, CsiVolumeMountAttributePvcName, CsiVolumeMountAttributeProvisionerIdentity)
	if !utils.EqualStringMaps(expectedClonedAttrs, actualClonedAttrs) {
		return false
	}
	expectedNameAttr := expectedPv.Spec.CSI.VolumeAttributes[CsiVolumeMountAttributePvName]
	actualNameAttr := actualPv.Spec.CSI.VolumeAttributes[CsiVolumeMountAttributePvName]
	if len(expectedNameAttr) == 0 || len(actualNameAttr) == 0 {
		return false
	}
	if expectedNameAttr[:20] != actualNameAttr[:20] {
		return false
	}
	expectedPvcNameAttr := expectedPv.Spec.CSI.VolumeAttributes[CsiVolumeMountAttributePvcName]
	actualPvcNameAttr := actualPv.Spec.CSI.VolumeAttributes[CsiVolumeMountAttributePvcName]
	if expectedPvcNameAttr[:len(expectedPvcNameAttr)-5] != actualPvcNameAttr[:len(actualPvcNameAttr)-5] {
		return false
	}

	if !utils.EqualStringMaps(getMountOptionsMap(expectedPv.Spec.MountOptions), getMountOptionsMap(actualPv.Spec.MountOptions)) {
		return false
	}
	// ignore expectedPv.Spec.StorageClassName != actualPv.Spec.StorageClassName for transition period
	if expectedPv.Spec.Capacity[corev1.ResourceStorage] != actualPv.Spec.Capacity[corev1.ResourceStorage] ||
		len(expectedPv.Spec.AccessModes) != len(actualPv.Spec.AccessModes) ||
		(len(expectedPv.Spec.AccessModes) != 1 && expectedPv.Spec.AccessModes[0] != actualPv.Spec.AccessModes[0]) ||
		expectedPv.Spec.CSI.Driver != actualPv.Spec.CSI.Driver {
		return false
	}
	if expectedPv.Spec.CSI.NodeStageSecretRef != nil {
		if actualPv.Spec.CSI.NodeStageSecretRef == nil || expectedPv.Spec.CSI.NodeStageSecretRef.Name != actualPv.Spec.CSI.NodeStageSecretRef.Name {
			return false
		}
	} else if actualPv.Spec.CSI.NodeStageSecretRef != nil {
		return false
	}
	if expectedPv.Spec.ClaimRef != nil {
		if actualPv.Spec.ClaimRef == nil { // ignore  || expectedPv.Spec.ClaimRef.Name != actualPv.Spec.ClaimRef.Name
			return false
		}
	} else if actualPv.Spec.ClaimRef != nil {
		return false
	}
	return true
}

func cloneMap(original map[string]string, ignoreKeys ...string) map[string]string {
	copy := make(map[string]string, len(original))
	ignoreKeysMap := convertToSet(ignoreKeys)
	for key, value := range original {
		if _, ok := ignoreKeysMap[key]; !ok {
			copy[key] = value
		}
	}
	return copy
}

func convertToSet(ignoreKeys []string) map[string]struct{} {
	return slice.Reduce(ignoreKeys, make(map[string]struct{}), func(acc map[string]struct{}, item string) map[string]struct{} {
		acc[item] = struct{}{}
		return acc
	})
}

func getPvAnnotations(pv *corev1.PersistentVolume) map[string]string {
	annotations := make(map[string]string)
	for key, value := range pv.GetAnnotations() {
		if key == "kubectl.kubernetes.io/last-applied-configuration" {
			continue // ignore automatically added annotation(s)
		}
		annotations[key] = value
	}
	return annotations
}

func getVolumeAttributes(pv *corev1.PersistentVolume) map[string]string {
	attributes := make(map[string]string)
	for key, value := range pv.Spec.CSI.VolumeAttributes {
		if key == CsiVolumeMountAttributeProvisionerIdentity {
			continue // ignore automatically added attribute(s)
		}
		attributes[key] = value
	}
	return attributes
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
