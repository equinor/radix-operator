package volumemount

import (
	"maps"
	"slices"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

var (
	// PV label keys that must have the desired values. Other PV label keys are ignored.
	matchedPersistentVolumeLabelKeys = []string{kube.RadixAppLabel, kube.RadixNamespace, kube.RadixComponentLabel, kube.RadixVolumeMountNameLabel}

	// CSI volume attributes that are set by a provisioner, and are therefore ignored.
	provisionerVolumeAttributeKeys = []string{csiVolumeMountAttributePvName, csiVolumeMountAttributePvcName, csiVolumeMountAttributePvcNamespace, csiVolumeMountAttributeProvisionerIdentity}
)

// matchesDesired returns true if the existing PV and PVC are bound to each other, and their spec and labels match the desired PV and PVC.
// Fields that are derived from names or random values (names, claimRef uid/resourceVersion, volumeHandle and the block cache path value) are not compared.
func matchesDesired(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim, desiredPV *corev1.PersistentVolume, desiredPVC *corev1.PersistentVolumeClaim) bool {
	if pv == nil || pvc == nil || desiredPV == nil || desiredPVC == nil {
		return false
	}

	if pvc.Spec.VolumeName != pv.GetName() {
		return false
	}
	if claimRef := pv.Spec.ClaimRef; claimRef == nil || claimRef.Namespace != pvc.GetNamespace() || claimRef.Name != pvc.GetName() {
		return false
	}

	return pvcMatchesDesired(pvc, desiredPVC) && pvMatchesDesired(pv, desiredPV)
}

func pvcMatchesDesired(pvc, desired *corev1.PersistentVolumeClaim) bool {
	if pvc.GetNamespace() != desired.GetNamespace() {
		return false
	}
	if !cmp.Equal(pvc.GetLabels(), desired.GetLabels(), cmpopts.EquateEmpty()) {
		return false
	}
	if !cmp.Equal(pvc.Spec.Resources, desired.Spec.Resources, cmpopts.EquateEmpty()) {
		return false
	}
	if !slices.Equal(sortedAccessModes(pvc.Spec.AccessModes), sortedAccessModes(desired.Spec.AccessModes)) {
		return false
	}
	if !cmp.Equal(pvc.Spec.VolumeMode, desired.Spec.VolumeMode) {
		return false
	}
	return ptr.Deref(pvc.Spec.StorageClassName, "") == ptr.Deref(desired.Spec.StorageClassName, "")
}

func pvMatchesDesired(pv, desired *corev1.PersistentVolume) bool {
	if pv.Spec.CSI == nil || desired.Spec.CSI == nil {
		return false
	}
	for _, key := range matchedPersistentVolumeLabelKeys {
		if pv.GetLabels()[key] != desired.GetLabels()[key] {
			return false
		}
	}
	if !cmp.Equal(pv.Spec.Capacity, desired.Spec.Capacity, cmpopts.EquateEmpty()) {
		return false
	}
	if !slices.Equal(sortedAccessModes(pv.Spec.AccessModes), sortedAccessModes(desired.Spec.AccessModes)) {
		return false
	}
	if !slices.Equal(normalizeMountOptions(pv.Spec.MountOptions), normalizeMountOptions(desired.Spec.MountOptions)) {
		return false
	}
	if pv.Spec.CSI.Driver != desired.Spec.CSI.Driver {
		return false
	}
	if !cmp.Equal(pv.Spec.CSI.NodeStageSecretRef, desired.Spec.CSI.NodeStageSecretRef) {
		return false
	}
	if !maps.Equal(withoutProvisionerVolumeAttributes(pv.Spec.CSI.VolumeAttributes), withoutProvisionerVolumeAttributes(desired.Spec.CSI.VolumeAttributes)) {
		return false
	}
	if pv.Spec.PersistentVolumeReclaimPolicy != desired.Spec.PersistentVolumeReclaimPolicy {
		return false
	}
	return pv.Spec.StorageClassName == desired.Spec.StorageClassName
}

func withoutProvisionerVolumeAttributes(attributes map[string]string) map[string]string {
	filtered := make(map[string]string, len(attributes))
	for key, value := range attributes {
		if !slices.Contains(provisionerVolumeAttributeKeys, key) {
			filtered[key] = value
		}
	}
	return filtered
}
