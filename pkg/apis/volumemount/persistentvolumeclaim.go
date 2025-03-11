package volumemount

import (
	"slices"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
)

// ComparePersistentVolumeClaims Compare two PersistentVolumeClaims
func ComparePersistentVolumeClaims(pvc1, pvc2 *corev1.PersistentVolumeClaim) bool {
	if pvc1 == nil || pvc2 == nil {
		return false
	}
	if pvc1.GetNamespace() != pvc2.GetNamespace() {
		return false
	}

	if !cmp.Equal(pvc1.GetLabels(), pvc2.GetLabels(), cmpopts.EquateEmpty()) {
		return false
	}

	if !cmp.Equal(pvc1.Spec.Resources, pvc2.Spec.Resources, cmpopts.EquateEmpty()) {
		return false
	}

	if !cmp.Equal(slices.Sorted(slices.Values(pvc1.Spec.AccessModes)), slices.Sorted(slices.Values(pvc2.Spec.AccessModes)), cmpopts.EquateEmpty()) {
		return false
	}

	return cmp.Equal(pvc1.Spec.VolumeMode, pvc2.Spec.VolumeMode)
}
