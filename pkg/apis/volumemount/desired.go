package volumemount

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	pvcNameHashLength          = 10
	pvNameHashLength           = 20
	volumeHandleHashLength     = 20
	blockCachePathMountOption  = "--block-cache-path"
	mountOptionKeyValSeparator = "="
)

var (
	defaultStorageCapacity resource.Quantity = resource.MustParse("1Mi")
)

// desiredVolumeResources holds the desired PersistentVolume and PersistentVolumeClaim for a BlobFuse2 volume mount.
// Names are deterministic, derived from a hash of the desired spec and labels.
type desiredVolumeResources struct {
	pv  *corev1.PersistentVolume
	pvc *corev1.PersistentVolumeClaim
}

func buildDesiredVolumeResources(rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent, radixVolumeMount radixv1.RadixVolumeMount) (*desiredVolumeResources, error) {
	if !radixVolumeMount.HasBlobFuse2() {
		return nil, fmt.Errorf("volume mount %s in component %s is not a BlobFuse2 volume mount", radixVolumeMount.Name, component.GetName())
	}
	volumeName, err := GetVolumeMountVolumeName(&radixVolumeMount, component.GetName())
	if err != nil {
		return nil, err
	}

	appName, envName, namespace := rd.Spec.AppName, rd.Spec.Environment, rd.GetNamespace() //nolint:staticcheck
	specBuilder := newBlobfuse2PersistentVolumeBuilder(appName, envName, component, radixVolumeMount)

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Labels: radixlabels.ForBlobCSIAzurePersistentVolume(appName, namespace, component.GetName(), radixVolumeMount),
		},
		Spec: specBuilder.BuildSpec("", namespace, ""),
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Labels:    radixlabels.ForBlobCSIAzurePersistentVolumeClaim(appName, component.GetName(), radixVolumeMount),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{getVolumeMountAccessMode(radixVolumeMount)},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: getVolumeCapacityOrDefault(radixVolumeMount)},
			},
			StorageClassName: new(""), // use "" to avoid to use the "default" storage class
			VolumeMode:       new(corev1.PersistentVolumeFilesystem),
		},
	}

	hash, err := hashDesiredVolumeResources(namespace, pv, pvc)
	if err != nil {
		return nil, fmt.Errorf("failed to compute hash for volume mount %s in component %s: %w", radixVolumeMount.Name, component.GetName(), err)
	}

	pvName := fmt.Sprintf(csiPersistentVolumeNameTemplate, hash[:pvNameHashLength])
	pvcName := fmt.Sprintf(csiPersistentVolumeClaimNameTemplate, volumeName, hash[:pvcNameHashLength])
	volumeHandle := buildVolumeHandle(appName, envName, component.GetName(), radixVolumeMount.Name, hash[:volumeHandleHashLength])

	pv.SetName(pvName)
	pv.Spec = specBuilder.BuildSpec(pvcName, namespace, volumeHandle)
	pvc.SetName(pvcName)
	pvc.Spec.VolumeName = pvName

	return &desiredVolumeResources{pv: pv, pvc: pvc}, nil
}

// hashDesiredVolumeResources returns a hex encoded sha256 of a canonical encoding of the desired PV and PVC.
// Fields that are derived from names or random values are left out, and lists are sorted,
// so the same desired spec always gives the same hash.
func hashDesiredVolumeResources(namespace string, pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) (string, error) {
	pvSpec := pv.Spec.DeepCopy()
	if pvSpec.ClaimRef != nil {
		pvSpec.ClaimRef.Name = ""
		pvSpec.ClaimRef.UID = ""
		pvSpec.ClaimRef.ResourceVersion = ""
	}
	if pvSpec.CSI != nil {
		pvSpec.CSI.VolumeHandle = ""
	}
	pvSpec.MountOptions = normalizeMountOptions(pvSpec.MountOptions)
	pvSpec.AccessModes = sortedAccessModes(pvSpec.AccessModes)

	pvcSpec := pvc.Spec.DeepCopy()
	pvcSpec.VolumeName = ""
	pvcSpec.AccessModes = sortedAccessModes(pvcSpec.AccessModes)

	// encoding/json sorts map keys and keeps struct field order, which makes the encoding canonical
	canonical := struct {
		Namespace string                            `json:"namespace"`
		PVLabels  map[string]string                 `json:"pvLabels"`
		PVSpec    *corev1.PersistentVolumeSpec      `json:"pvSpec"`
		PVCLabels map[string]string                 `json:"pvcLabels"`
		PVCSpec   *corev1.PersistentVolumeClaimSpec `json:"pvcSpec"`
	}{
		Namespace: namespace,
		PVLabels:  pv.GetLabels(),
		PVSpec:    pvSpec,
		PVCLabels: pvc.GetLabels(),
		PVCSpec:   pvcSpec,
	}
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
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
	case strings.ToLower(string(corev1.ReadWriteOncePod)):
		return corev1.ReadWriteOncePod
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
