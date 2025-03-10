package volumemount

import (
	"strings"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/internal"
	corev1 "k8s.io/api/core/v1"
)

// EqualPersistentVolumes Compare two PersistentVolumes
func EqualPersistentVolumes(pv1, pv2 *corev1.PersistentVolume) bool {
	if pv1 == nil || pv2 == nil || pv1.Spec.CSI == nil || pv2.Spec.CSI == nil {
		return false
	}
	// Ignore for now, due to during transition period this would affect existing volume mounts, managed by a provisioner. When all volume mounts gets labels, uncomment these lines
	//if !utils.EqualStringMaps(pv1.GetLabels(), pv2.GetLabels()) {
	//	return false
	//}
	expectedClonedVolumeAttrs := cloneMap(pv1.Spec.CSI.VolumeAttributes, csiVolumeMountAttributePvName, csiVolumeMountAttributePvcName, csiVolumeMountAttributePvcNamespace, csiVolumeMountAttributeProvisionerIdentity)
	actualClonedVolumeAttrs := cloneMap(pv2.Spec.CSI.VolumeAttributes, csiVolumeMountAttributePvName, csiVolumeMountAttributePvcName, csiVolumeMountAttributePvcNamespace, csiVolumeMountAttributeProvisionerIdentity)
	if !utils.EqualStringMaps(expectedClonedVolumeAttrs, actualClonedVolumeAttrs) {
		return false
	}
	if !utils.EqualStringMaps(getMountOptionsMap(pv1.Spec.MountOptions), getMountOptionsMap(pv2.Spec.MountOptions)) {
		return false
	}

	if pv1.Spec.Capacity[corev1.ResourceStorage] != pv2.Spec.Capacity[corev1.ResourceStorage] ||
		len(pv1.Spec.AccessModes) != len(pv2.Spec.AccessModes) ||
		(len(pv1.Spec.AccessModes) == 1 && pv1.Spec.AccessModes[0] != pv2.Spec.AccessModes[0]) ||
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
		if pv2.Spec.ClaimRef == nil ||
			!internal.EqualTillPostfix(pv1.Spec.ClaimRef.Name, pv2.Spec.ClaimRef.Name, nameRandPartLength) ||
			!internal.EqualTillPostfix(pv1.Spec.ClaimRef.Name, pv2.Spec.ClaimRef.Name, nameRandPartLength) ||
			pv1.Spec.ClaimRef.Namespace != pv2.Spec.ClaimRef.Namespace ||
			pv1.Spec.ClaimRef.Kind != pv2.Spec.ClaimRef.Kind {
			return false
		}
	} else if pv2.Spec.ClaimRef != nil {
		return false
	}
	return true
}

func cloneMap(original map[string]string, ignoreKeys ...string) map[string]string {
	clonedMap := make(map[string]string, len(original))
	ignoreKeysMap := convertToSet(ignoreKeys)
	for key, value := range original {
		if _, ok := ignoreKeysMap[key]; !ok {
			clonedMap[key] = value
		}
	}
	return clonedMap
}

func convertToSet(ignoreKeys []string) map[string]struct{} {
	return slice.Reduce(ignoreKeys, make(map[string]struct{}), func(acc map[string]struct{}, item string) map[string]struct{} {
		acc[item] = struct{}{}
		return acc
	})
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
