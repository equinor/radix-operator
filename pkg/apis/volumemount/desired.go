package volumemount

import (
	"fmt"
	"slices"
	"strings"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/oklog/ulid/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	blockCachePathMountOption  = "--block-cache-path"
	mountOptionKeyValSeparator = "="
)

var (
	defaultStorageCapacity resource.Quantity = resource.MustParse("1Mi")
)

// desiredVolumeResources holds the desired PersistentVolume and PersistentVolumeClaim for a BlobFuse2 volume mount.
// Each build gets new unique names, so a new pair never conflicts with an existing pair, e.g. one that is still terminating.
type desiredVolumeResources struct {
	pv  *corev1.PersistentVolume
	pvc *corev1.PersistentVolumeClaim
}

// buildDesiredVolumeResources builds the desired PV and PVC for the BlobFuse2 volume mount of the component in the RadixDeployment, for the application appName.
// The PV, the PVC and the CSI volume handle get the same new ULID as unique part of their names.
func buildDesiredVolumeResources(appName string, rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent, radixVolumeMount radixv1.RadixVolumeMount) (*desiredVolumeResources, error) {
	if !radixVolumeMount.HasBlobFuse2() {
		return nil, fmt.Errorf("volume mount %s in component %s is not a BlobFuse2 volume mount", radixVolumeMount.Name, component.GetName())
	}

	envName, namespace := rd.Spec.Environment, rd.GetNamespace()
	uniquePart := strings.ToLower(ulid.Make().String())
	pvName := fmt.Sprintf(csiPersistentVolumeNameTemplate, appName, uniquePart)
	pvcName := fmt.Sprintf(csiPersistentVolumeClaimNameTemplate, component.GetName(), radixVolumeMount.Name, uniquePart)
	volumeHandle := buildVolumeHandle(appName, envName, component.GetName(), radixVolumeMount.Name, uniquePart)

	pv := &corev1.PersistentVolume{
		Name:   pvName,
		Labels: radixlabels.ForBlobCSIAzurePersistentVolume(appName, namespace, component.GetName(), radixVolumeMount),
		Spec:   newBlobfuse2PersistentVolumeBuilder(component, radixVolumeMount).buildSpec(pvcName, namespace, volumeHandle),
	}
	pvc := &corev1.PersistentVolumeClaim{
		Name:      pvcName,
		Namespace: namespace,
		Labels:    radixlabels.ForBlobCSIAzurePersistentVolumeClaim(appName, component.GetName(), radixVolumeMount),
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{getVolumeMountAccessMode(radixVolumeMount)},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: getVolumeCapacityOrDefault(radixVolumeMount)},
			},
			StorageClassName: new(""), // use "" to avoid to use the "default" storage class
			VolumeMode:       new(corev1.PersistentVolumeFilesystem),
			VolumeName:       pvName,
		},
	}

	return &desiredVolumeResources{pv: pv, pvc: pvc}, nil
}

// normalizeMountOptions returns sorted mount options, where options with values derived from names (like the block cache path) only keep the option name
func normalizeMountOptions(mountOptions []string) []string {
	normalized := make([]string, 0, len(mountOptions))
	for _, option := range mountOptions {
		normalized = append(normalized, normalizeMountOption(option))
	}
	slices.Sort(normalized)
	return normalized
}

func normalizeMountOption(option string) string {
	if name, _, _ := strings.Cut(option, mountOptionKeyValSeparator); name == blockCachePathMountOption {
		return name
	}
	return option
}

func sortedAccessModes(accessModes []corev1.PersistentVolumeAccessMode) []corev1.PersistentVolumeAccessMode {
	return slices.Sorted(slices.Values(accessModes))
}

// getVolumeMountAccessMode returns the access mode of the PV and PVC. The supported values are the ones allowed by the RadixApplication schema.
func getVolumeMountAccessMode(radixVolumeMount radixv1.RadixVolumeMount) corev1.PersistentVolumeAccessMode {
	var accessMode string
	if radixVolumeMount.BlobFuse2 != nil {
		accessMode = radixVolumeMount.BlobFuse2.AccessMode
	}
	switch strings.ToLower(accessMode) {
	case strings.ToLower(string(corev1.ReadWriteOnce)):
		return corev1.ReadWriteOnce
	case strings.ToLower(string(corev1.ReadWriteMany)):
		return corev1.ReadWriteMany
	}
	return corev1.ReadOnlyMany // default access mode
}

func getVolumeCapacityOrDefault(radixVolumeMount radixv1.RadixVolumeMount) resource.Quantity {
	qty := radixVolumeMount.GetRequestsStorage()
	if qty.IsZero() {
		return defaultStorageCapacity
	}
	return qty
}
