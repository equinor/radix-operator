package volumemount

import (
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type persistentVolumeSpecBuilder interface {
	BuildSpec(pvName, pvcName, pvcNamespace string) corev1.PersistentVolumeSpec
}

func newDeprecatedPersistentVolumeSpecBuilder(appName, componentName string, radixVolumeMount radixv1.RadixVolumeMount) *deprecatedPersistentVolumeSpecBuilder {
	return &deprecatedPersistentVolumeSpecBuilder{
		appName:          appName,
		componentName:    componentName,
		radixVolumeMount: radixVolumeMount,
	}
}

type deprecatedPersistentVolumeSpecBuilder struct {
	appName          string
	componentName    string
	radixVolumeMount radixv1.RadixVolumeMount
}

func (b *deprecatedPersistentVolumeSpecBuilder) BuildSpec(pvName, pvcName, pvcNamespace string) corev1.PersistentVolumeSpec {

	spec := corev1.PersistentVolumeSpec{
		StorageClassName:              "",
		PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
		MountOptions:                  b.getMountOptions(),
		Capacity:                      corev1.ResourceList{corev1.ResourceStorage: b.getResourceStorage()},
		AccessModes:                   []corev1.PersistentVolumeAccessMode{b.getAccessMode()},
		ClaimRef: &corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       k8s.KindPersistentVolumeClaim,
			Namespace:  pvcNamespace,
			Name:       pvcName,
		},
		PersistentVolumeSource: corev1.PersistentVolumeSource{
			CSI: &corev1.CSIPersistentVolumeSource{
				Driver:           provisionerBlobCsiAzure,
				VolumeHandle:     b.getVolumeHandle(pvName, pvcNamespace),
				VolumeAttributes: b.getVolumeAttributes(pvcNamespace),
				NodeStageSecretRef: &corev1.SecretReference{
					Name:      defaults.GetCsiAzureVolumeMountCredsSecretName(b.componentName, b.radixVolumeMount.Name),
					Namespace: pvcNamespace,
				},
			},
		},
	}

	return spec
}

func (b *deprecatedPersistentVolumeSpecBuilder) getVolumeHandle(pvName, namespace string) string {
	// Specify a value the driver can use to uniquely identify the share in the cluster.
	// https://github.com/kubernetes-csi/csi-driver-smb/blob/master/docs/driver-parameters.md#pvpvc-usage
	return fmt.Sprintf("%s#%s#%s#%s", namespace, b.componentName, pvName, b.radixVolumeMount.Storage)
}

func (b *deprecatedPersistentVolumeSpecBuilder) getVolumeAttributes(secretNamespace string) map[string]string {
	// Ref https://github.com/kubernetes-sigs/blob-csi-driver/blob/master/docs/driver-parameters.md#static-provisioningbring-your-own-storage-container
	return map[string]string{
		csiVolumeMountAttributeContainerName:   b.radixVolumeMount.Storage,
		csiVolumeMountAttributeProtocol:        "fuse",
		csiVolumeMountAttributeSecretNamespace: secretNamespace,
	}
}

func (b *deprecatedPersistentVolumeSpecBuilder) getMountOptions() []string {
	mountOptions := []string{
		"--file-cache-timeout-in-seconds=120",
		"--use-attr-cache=true",
		"--cancel-list-on-mount-seconds=0",
		"-o allow_other",
		"-o attr_timeout=120",
		"-o entry_timeout=120",
		"-o negative_timeout=120",
	}

	if gid := b.radixVolumeMount.GID; len(gid) > 0 {
		mountOptions = append(mountOptions, fmt.Sprintf("-o gid=%s", gid))
	}
	if uid := b.radixVolumeMount.UID; len(uid) > 0 {
		mountOptions = append(mountOptions, fmt.Sprintf("-o uid=%s", uid))
	}

	if b.getAccessMode() == corev1.ReadOnlyMany {
		mountOptions = append(mountOptions, "--read-only=true")
	}

	return mountOptions
}

func (b *deprecatedPersistentVolumeSpecBuilder) getResourceStorage() resource.Quantity {
	qty := b.radixVolumeMount.RequestsStorage
	if qty.IsZero() {
		return defaultStorageCapacity
	}
	return qty
}

func (b *deprecatedPersistentVolumeSpecBuilder) getAccessMode() corev1.PersistentVolumeAccessMode {
	switch strings.ToLower(b.radixVolumeMount.AccessMode) {
	case strings.ToLower(string(corev1.ReadWriteOnce)):
		return corev1.ReadWriteOnce
	case strings.ToLower(string(corev1.ReadWriteMany)):
		return corev1.ReadWriteMany
	case strings.ToLower(string(corev1.ReadWriteOncePod)):
		return corev1.ReadWriteOncePod
	default:
		return corev1.ReadOnlyMany
	}
}

func newBlobfuse2PersistentVolumeBuilder(appName string, deployComponent radixv1.RadixCommonDeployComponent, radixVolumeMount radixv1.RadixVolumeMount) *blobfuse2PersistentVolumeSpecBuilder {
	return &blobfuse2PersistentVolumeSpecBuilder{
		appName:          appName,
		deployComponent:  deployComponent,
		radixVolumeMount: radixVolumeMount,
	}
}

type blobfuse2PersistentVolumeSpecBuilder struct {
	appName          string
	deployComponent  radixv1.RadixCommonDeployComponent
	radixVolumeMount radixv1.RadixVolumeMount
}

func (b *blobfuse2PersistentVolumeSpecBuilder) BuildSpec(pvName, pvcName, pvcNamespace string) corev1.PersistentVolumeSpec {
	spec := corev1.PersistentVolumeSpec{
		StorageClassName:              "",
		PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
		MountOptions:                  b.getMountOptions(),
		Capacity:                      corev1.ResourceList{corev1.ResourceStorage: b.getResourceStorage()},
		AccessModes:                   []corev1.PersistentVolumeAccessMode{b.getAccessMode()},
		ClaimRef: &corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       k8s.KindPersistentVolumeClaim,
			Namespace:  pvcNamespace,
			Name:       pvcName,
		},
		PersistentVolumeSource: corev1.PersistentVolumeSource{
			CSI: &corev1.CSIPersistentVolumeSource{
				Driver:           provisionerBlobCsiAzure,
				VolumeHandle:     b.getVolumeHandle(pvName, pvcNamespace),
				VolumeAttributes: b.getVolumeAttributes(pvcNamespace),
			},
		},
	}

	if b.radixVolumeMount.BlobFuse2.UseAzureIdentity == nil || !*b.radixVolumeMount.BlobFuse2.UseAzureIdentity {
		csiVolumeCredSecretName := defaults.GetCsiAzureVolumeMountCredsSecretName(b.deployComponent.GetName(), b.radixVolumeMount.Name)
		spec.CSI.NodeStageSecretRef = &corev1.SecretReference{Name: csiVolumeCredSecretName, Namespace: pvcNamespace}
	}

	return spec
}

func (b *blobfuse2PersistentVolumeSpecBuilder) getAccessMode() corev1.PersistentVolumeAccessMode {
	switch strings.ToLower(b.radixVolumeMount.BlobFuse2.AccessMode) {
	case strings.ToLower(string(corev1.ReadWriteOnce)):
		return corev1.ReadWriteOnce
	case strings.ToLower(string(corev1.ReadWriteMany)):
		return corev1.ReadWriteMany
	case strings.ToLower(string(corev1.ReadWriteOncePod)):
		return corev1.ReadWriteOncePod
	default:
		return corev1.ReadOnlyMany
	}
}

func (b *blobfuse2PersistentVolumeSpecBuilder) getResourceStorage() resource.Quantity {
	qty := b.radixVolumeMount.BlobFuse2.RequestsStorage
	if qty.IsZero() {
		return defaultStorageCapacity
	}
	return qty
}

func (b *blobfuse2PersistentVolumeSpecBuilder) getVolumeHandle(pvName, namespace string) string {
	// Specify a value the driver can use to uniquely identify the share in the cluster.
	// https://github.com/kubernetes-csi/csi-driver-smb/blob/master/docs/driver-parameters.md#pvpvc-usage
	return fmt.Sprintf("%s#%s#%s#%s", namespace, b.deployComponent.GetName(), pvName, b.radixVolumeMount.BlobFuse2.Container)
}

func (b *blobfuse2PersistentVolumeSpecBuilder) getVolumeAttributes(secretNamespace string) map[string]string {
	// Ref https://github.com/kubernetes-sigs/blob-csi-driver/blob/master/docs/driver-parameters.md#static-provisioningbring-your-own-storage-container
	attributes := map[string]string{
		csiVolumeMountAttributeContainerName: b.radixVolumeMount.BlobFuse2.Container,
		csiVolumeMountAttributeProtocol:      "fuse2",
	}
	if len(b.radixVolumeMount.BlobFuse2.StorageAccount) > 0 {
		attributes[csiVolumeAttributeStorageAccount] = b.radixVolumeMount.BlobFuse2.StorageAccount
	}
	if b.radixVolumeMount.UseAzureIdentity() {
		attributes[csiVolumeAttributeClientID] = b.deployComponent.GetIdentity().GetAzure().GetClientId()
		attributes[csiVolumeAttributeResourceGroup] = b.radixVolumeMount.BlobFuse2.ResourceGroup
		if len(b.radixVolumeMount.BlobFuse2.SubscriptionId) > 0 {
			attributes[csiVolumeAttributeSubscriptionId] = b.radixVolumeMount.BlobFuse2.SubscriptionId
		}
		if len(b.radixVolumeMount.BlobFuse2.TenantId) > 0 {
			attributes[csiVolumeAttributeTenantId] = b.radixVolumeMount.BlobFuse2.TenantId
		}
	} else {
		attributes[csiVolumeMountAttributeSecretNamespace] = secretNamespace
	}
	return attributes
}

func (b *blobfuse2PersistentVolumeSpecBuilder) getMountOptions() []string {
	mountOptions := []string{
		// "--disable-writeback-cache=true",
		"--file-cache-timeout-in-seconds=120",
		"--use-attr-cache=true",
		"--cancel-list-on-mount-seconds=0",
		"-o allow_other",
		"-o attr_timeout=120",
		"-o entry_timeout=120",
		"-o negative_timeout=120",
	}
	mountOptions = append(mountOptions, fmt.Sprintf("--use-adls=%v", b.radixVolumeMount.BlobFuse2.UseAdls != nil && *b.radixVolumeMount.BlobFuse2.UseAdls))

	if b.getAccessMode() == corev1.ReadOnlyMany {
		mountOptions = append(mountOptions, "--read-only=true")
	}

	if gid := b.radixVolumeMount.BlobFuse2.GID; len(gid) > 0 {
		mountOptions = append(mountOptions, fmt.Sprintf("-o gid=%s", gid))
	}

	if uid := b.radixVolumeMount.BlobFuse2.UID; len(uid) > 0 {
		mountOptions = append(mountOptions, fmt.Sprintf("-o uid=%s", uid))
	}

	mountOptions = append(mountOptions, b.getStreamingMountOptions()...)

	return mountOptions
}

func (b *blobfuse2PersistentVolumeSpecBuilder) getStreamingMountOptions() []string {
	var mountOptions []string
	streaming := b.radixVolumeMount.BlobFuse2.StreamingOptions
	if streaming != nil && streaming.Enabled != nil && !*streaming.Enabled {
		return nil
	}
	mountOptions = append(mountOptions, "--streaming=true")

	var streamCache uint64 = 750
	if streaming != nil && streaming.StreamCache != nil {
		streamCache = *streaming.StreamCache
	}
	mountOptions = append(mountOptions, fmt.Sprintf("--block-cache-pool-size=%v", streamCache))

	return mountOptions
}
