package volumemount

import (
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	defaultAttributeCacheTimeout    uint32 = 0
	defaultFileCacheTimeout         uint32 = 120
	defaultBlockCacheBlockSize      uint32 = 4
	defaultBlockCacheDiskSize       uint32 = 0
	defaultBlockCacheDiskTimeout    uint32 = 120
	defaultBlockCachePrefetchCount  uint32 = 11
	defaultBlockCachePrefetchOnOpen bool   = false
	defaultBlockCacheParallelism    uint32 = 8
)

type persistentVolumeSpecBuilder interface {
	BuildSpec(pvcName, pvcNamespace string) corev1.PersistentVolumeSpec
}

func newDeprecatedPersistentVolumeSpecBuilder(appName, envName string, deployComponent radixv1.RadixCommonDeployComponent, radixVolumeMount radixv1.RadixVolumeMount) *deprecatedPersistentVolumeSpecBuilder {
	return &deprecatedPersistentVolumeSpecBuilder{
		appName:          appName,
		envName:          envName,
		deployComponent:  deployComponent,
		radixVolumeMount: radixVolumeMount,
	}
}

type deprecatedPersistentVolumeSpecBuilder struct {
	appName          string
	envName          string
	deployComponent  radixv1.RadixCommonDeployComponent
	radixVolumeMount radixv1.RadixVolumeMount
}

func (b *deprecatedPersistentVolumeSpecBuilder) BuildSpec(pvcName, pvcNamespace string) corev1.PersistentVolumeSpec {
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
				VolumeHandle:     b.getVolumeHandle(),
				VolumeAttributes: b.getVolumeAttributes(pvcNamespace),
				NodeStageSecretRef: &corev1.SecretReference{
					Name:      defaults.GetCsiAzureVolumeMountCredsSecretName(b.deployComponent.GetName(), b.radixVolumeMount.Name),
					Namespace: utils.GetEnvironmentNamespace(b.appName, b.envName),
				},
			},
		},
	}

	return spec
}

func (b *deprecatedPersistentVolumeSpecBuilder) getVolumeHandle() string {
	// Specify a value the driver can use to uniquely identify the share in the cluster.
	// https://github.com/kubernetes-csi/csi-driver-smb/blob/master/docs/driver-parameters.md#pvpvc-usage
	return fmt.Sprintf("radixvolumemount#%s#%s#%s#%s#%s", b.appName, b.envName, b.deployComponent.GetName(), b.radixVolumeMount.Name, uuid.New().String())
}

//nolint:staticcheck
func (b *deprecatedPersistentVolumeSpecBuilder) getVolumeAttributes(secretNamespace string) map[string]string {
	// Ref https://github.com/kubernetes-sigs/blob-csi-driver/blob/master/docs/driver-parameters.md#static-provisioningbring-your-own-storage-container
	return map[string]string{
		csiVolumeMountAttributeContainerName:   b.radixVolumeMount.Storage,
		csiVolumeMountAttributeProtocol:        "fuse",
		csiVolumeMountAttributeSecretNamespace: secretNamespace,
	}
}

//nolint:staticcheck
func (b *deprecatedPersistentVolumeSpecBuilder) getMountOptions() []string {
	mountOptions := []string{
		"--file-cache-timeout-in-seconds=120",
		"--cancel-list-on-mount-seconds=0",
		"--attr-cache-timeout=0",
		"--allow-other",
		"--attr-timeout=0",
		"--entry-timeout=0",
		"--negative-timeout=0",
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

//nolint:staticcheck
func (b *deprecatedPersistentVolumeSpecBuilder) getResourceStorage() resource.Quantity {
	qty := b.radixVolumeMount.RequestsStorage
	if qty.IsZero() {
		return defaultStorageCapacity
	}
	return qty
}

//nolint:staticcheck
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

func newBlobfuse2PersistentVolumeBuilder(appName, envName string, deployComponent radixv1.RadixCommonDeployComponent, radixVolumeMount radixv1.RadixVolumeMount) *blobfuse2PersistentVolumeSpecBuilder {
	return &blobfuse2PersistentVolumeSpecBuilder{
		appName:          appName,
		envName:          envName,
		deployComponent:  deployComponent,
		radixVolumeMount: radixVolumeMount,
	}
}

type blobfuse2PersistentVolumeSpecBuilder struct {
	appName          string
	envName          string
	deployComponent  radixv1.RadixCommonDeployComponent
	radixVolumeMount radixv1.RadixVolumeMount
}

func (b *blobfuse2PersistentVolumeSpecBuilder) BuildSpec(pvcName, pvcNamespace string) corev1.PersistentVolumeSpec {
	spec := corev1.PersistentVolumeSpec{
		StorageClassName:              "",
		PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
		MountOptions:                  b.mountOptions(),
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
				VolumeHandle:     b.getVolumeHandle(),
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

func (b *blobfuse2PersistentVolumeSpecBuilder) getVolumeHandle() string {
	// Specify a value the driver can use to uniquely identify the share in the cluster.
	// https://github.com/kubernetes-csi/csi-driver-smb/blob/master/docs/driver-parameters.md#pvpvc-usage
	return fmt.Sprintf("radixvolumemount#%s#%s#%s#%s#%s", b.appName, b.envName, b.deployComponent.GetName(), b.radixVolumeMount.Name, uuid.New().String())
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

func (b *blobfuse2PersistentVolumeSpecBuilder) mountOptions() []string {
	// --disable-writeback-cache must be set to true, otherwise changes in source will not be propagated to mounts
	// Documentation (https://github.com/Azure/azure-storage-fuse/blob/main/doc/blobfuse2_mount.md) doesn't explain much about this flag,
	// but seems to cause sync issues (stale data) if omitted,
	// ref https://github.com/Azure/azure-storage-fuse/issues/1195, https://techcommunity.microsoft.com/blog/azurepaasblog/how-to-troubleshoot-blobfuse2-issues/4110844

	mountOptions := []string{
		"--disable-writeback-cache=true",
		"--cancel-list-on-mount-seconds=0",
		"--allow-other",
		"--attr-timeout=0",
		"--entry-timeout=0",
		"--negative-timeout=0",
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

	mountOptions = append(mountOptions, b.attributeCacheMountOptions()...)
	mountOptions = append(mountOptions, b.cacheMountOptions()...)
	return mountOptions
}

func (b *blobfuse2PersistentVolumeSpecBuilder) cacheMountOptions() []string {
	switch b.resolveCacheMode() {
	case radixv1.BlobFuse2CacheModeBlock:
		return b.blockCacheMountOptions()
	case radixv1.BlobFuse2CacheModeFile:
		return b.fileCacheMountOptions()
	case radixv1.BlobFuse2CacheModeDirectIO:
		return b.directIOMountOptions()
	default:
		return b.blockCacheMountOptions() // Fallback to block cache in case of unknown cache mode
	}
}

func (b *blobfuse2PersistentVolumeSpecBuilder) blockCacheMountOptions() []string {
	opts := []string{
		"--block-cache",
	}

	blockSize := defaultBlockCacheBlockSize
	poolSize := uint32(0)
	diskSize := defaultBlockCacheDiskSize
	diskTimeout := defaultBlockCacheDiskTimeout
	prefetchCount := defaultBlockCachePrefetchCount
	prefetchOnOpen := defaultBlockCachePrefetchOnOpen
	parallelism := defaultBlockCacheParallelism

	if blockCache := b.radixVolumeMount.BlobFuse2.BlockCacheOptions; blockCache != nil {
		if v := blockCache.BlockSize; v != nil {
			blockSize = *v
		}
		if v := blockCache.PoolSize; v != nil {
			poolSize = *v
		}
		if v := blockCache.DiskSize; v != nil {
			diskSize = *v
		}
		if v := blockCache.DiskTimeout; v != nil {
			diskTimeout = *v
		}
		if v := blockCache.PrefetchCount; v != nil {
			prefetchCount = *v
		}
		if v := blockCache.PrefetchOnOpen; v != nil {
			prefetchOnOpen = *v
		}
		if v := blockCache.Parallelism; v != nil {
			parallelism = *v
		}
	}

	opts = append(opts, fmt.Sprintf("--block-cache-block-size=%d", blockSize))
	opts = append(opts, fmt.Sprintf("--block-cache-pool-size=%d", max(poolSize, max(1, prefetchCount)*blockSize)))
	opts = append(opts, fmt.Sprintf("--block-cache-prefetch=%d", prefetchCount))
	opts = append(opts, fmt.Sprintf("--block-cache-prefetch-on-open=%t", prefetchOnOpen && prefetchCount > 0))
	opts = append(opts, fmt.Sprintf("--block-cache-parallelism=%d", parallelism))

	if diskSize > 0 {
		opts = append(opts, fmt.Sprintf("--block-cache-path=%s", fmt.Sprintf("/mnt/%s#blockcache", b.getVolumeHandle())))
		opts = append(opts, fmt.Sprintf("--block-cache-disk-size=%d", max(diskSize, max(1, prefetchCount)*blockSize)))
		opts = append(opts, fmt.Sprintf("--block-cache-disk-timeout=%d", diskTimeout))
	}

	return opts
}

func (*blobfuse2PersistentVolumeSpecBuilder) directIOMountOptions() []string {
	return []string{"-o direct_io"}
}

func (b *blobfuse2PersistentVolumeSpecBuilder) fileCacheMountOptions() []string {
	timeout := defaultFileCacheTimeout
	if fileCache := b.radixVolumeMount.BlobFuse2.FileCacheOptions; fileCache != nil && fileCache.Timeout != nil {
		timeout = *fileCache.Timeout
	}
	return []string{
		fmt.Sprintf("--file-cache-timeout=%d", timeout),
	}
}

func (b *blobfuse2PersistentVolumeSpecBuilder) resolveCacheMode() radixv1.BlobFuse2CacheMode {
	if b.radixVolumeMount.BlobFuse2.CacheMode != nil {
		return *b.radixVolumeMount.BlobFuse2.CacheMode
	}

	// Use file cache if streaming.enabled is explicitly set to false
	if streaming := b.radixVolumeMount.BlobFuse2.StreamingOptions; streaming != nil && streaming.Enabled != nil && !*streaming.Enabled {
		return radixv1.BlobFuse2CacheModeFile
	}

	return radixv1.BlobFuse2CacheModeBlock
}

func (b *blobfuse2PersistentVolumeSpecBuilder) attributeCacheMountOptions() []string {
	timeout := defaultAttributeCacheTimeout
	if b.radixVolumeMount.BlobFuse2.AttributeCacheOptions != nil && b.radixVolumeMount.BlobFuse2.AttributeCacheOptions.Timeout != nil {
		timeout = *b.radixVolumeMount.BlobFuse2.AttributeCacheOptions.Timeout
	}
	return []string{
		fmt.Sprintf("--attr-cache-timeout=%d", timeout),
	}
}
