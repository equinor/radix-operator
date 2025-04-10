package volumemount

import (
	"slices"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/internal"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
)

// ComparePersistentVolumes Compare two PersistentVolumes
func ComparePersistentVolumes(pv1, pv2 *corev1.PersistentVolume) bool {
	if pv1 == nil || pv2 == nil || pv1.Spec.CSI == nil || pv2.Spec.CSI == nil {
		return false
	}

	// Ignore for now, due to during transition period this would affect existing volume mounts, managed by a provisioner. When all volume mounts gets labels, uncomment these lines
	//if !cmp.Equal(pv1.GetLabels(), pv2.GetLabels(), cmpopts.EquateEmpty()) {
	//	return false
	//}

	ignoreMapKeys := cmpopts.IgnoreMapEntries(func(k, _ string) bool {
		keys := []string{csiVolumeMountAttributePvName, csiVolumeMountAttributePvcName, csiVolumeMountAttributePvcNamespace, csiVolumeMountAttributeProvisionerIdentity}
		return slices.Contains(keys, k)
	})
	if !cmp.Equal(pv1.Spec.CSI.VolumeAttributes, pv2.Spec.CSI.VolumeAttributes, cmpopts.EquateEmpty(), ignoreMapKeys) {
		return false
	}

	argNamesOnly := cmp.Comparer(func(val1, val2 string) bool {
		argNames := []string{"--block-cache-path"}
		if v, found := slice.FindFirst(argNames, func(argName string) bool { return strings.Split(val1, "=")[0] == argName }); found {
			val1 = v
		}
		if v, found := slice.FindFirst(argNames, func(argName string) bool { return strings.Split(val2, "=")[0] == argName }); found {
			val2 = v
		}
		return val1 == val2
	})
	if !cmp.Equal(slices.Sorted(slices.Values(pv1.Spec.MountOptions)), slices.Sorted(slices.Values(pv2.Spec.MountOptions)), cmpopts.EquateEmpty(), argNamesOnly) {
		return false
	}

	if !cmp.Equal(pv1.Spec.Capacity, pv2.Spec.Capacity, cmpopts.EquateEmpty()) {
		return false
	}

	if !cmp.Equal(slices.Sorted(slices.Values(pv1.Spec.AccessModes)), slices.Sorted(slices.Values(pv2.Spec.AccessModes)), cmpopts.EquateEmpty()) {
		return false
	}

	if pv1.Spec.CSI.Driver != pv2.Spec.CSI.Driver {
		return false
	}

	if !cmp.Equal(pv1.Spec.CSI.NodeStageSecretRef, pv2.Spec.CSI.NodeStageSecretRef) {
		return false
	}

	claimRefNameComparer := cmp.FilterPath(
		func(p cmp.Path) bool {
			return p.String() == "Name"
		},
		cmp.Comparer(
			func(claimName1, claimName2 string) bool {
				return internal.EqualTillPostfix(claimName1, claimName2, nameRandPartLength)
			},
		),
	)
	ignoreClaimRefFields := cmpopts.IgnoreFields(corev1.ObjectReference{}, "UID", "ResourceVersion")
	return cmp.Equal(pv1.Spec.ClaimRef, pv2.Spec.ClaimRef, claimRefNameComparer, ignoreClaimRefFields)
}
