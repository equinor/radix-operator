package volumemount

const (
	// CsiVolumeSourceDriverSecretStore Driver name for the secret store
	CsiVolumeSourceDriverSecretStore = "secrets-store.csi.k8s.io"
	// CsiVolumeSourceVolumeAttributeSecretProviderClass Secret provider class volume attribute
	CsiVolumeSourceVolumeAttributeSecretProviderClass = "secretProviderClass"
	// ReadOnlyMountOption The readonly volume mount option for CSI fuse driver
	ReadOnlyMountOption = "-o ro"

	csiPersistentVolumeClaimNameTemplate = "pvc-%s-%s"              // pvc-<volumename>-<randomstring5>
	csiPersistentVolumeNameTemplate      = "pv-radixvolumemount-%s" // pv-<guid>

	csiMountOptionGid                       = "gid"                 // Volume mount owner GroupID. Used when drivers do not honor fsGroup securityContext setting
	csiMountOptionUid                       = "uid"                 // Volume mount owner UserID. Used instead of GroupID
	csiMountOptionUseAdls                   = "use-adls"            // Use ADLS or Block Blob
	csiMountOptionStreamingEnabled          = "streaming"           // Enable Streaming
	csiMountOptionStreamingCache            = "stream-cache-mb"     // Limit total amount of data being cached in memory to conserve memory
	csiMountOptionStreamingMaxBlocksPerFile = "max-blocks-per-file" // Maximum number of blocks to be cached in memory for streaming
	csiMountOptionStreamingMaxBuffers       = "max-buffers"         // The total number of buffers to be cached in memory (in MB).
	csiMountOptionStreamingBlockSize        = "block-size-mb"       // The size of each block to be cached in memory (in MB).
	csiMountOptionStreamingBufferSize       = "buffer-size-mb"      // The size of each buffer to be cached in memory (in MB).

	csiVolumeAttributeProtocolParameterFuse  = "fuse"  // Protocol "blobfuse"
	csiVolumeAttributeProtocolParameterFuse2 = "fuse2" // Protocol "blobfuse2"
	csiVolumeAttributeStorageAccount         = "storageAccount"
	csiVolumeAttributeClientID               = "clientID"
	csiVolumeAttributeResourceGroup          = "resourcegroup"

	csiVolumeMountAttributePvName              = "csi.storage.k8s.io/pv/name"
	csiVolumeMountAttributePvcName             = "csi.storage.k8s.io/pvc/name"
	csiVolumeMountAttributePvcNamespace        = "csi.storage.k8s.io/pvc/namespace"
	csiVolumeMountAttributeSecretNamespace     = "secretnamespace"
	csiVolumeMountAttributeProtocol            = "protocol"      // Protocol
	csiVolumeMountAttributeContainerName       = "containerName" // Container name - foc container storages
	csiVolumeMountAttributeProvisionerIdentity = "storage.kubernetes.io/csiProvisionerIdentity"

	nameRandPartLength = 5 // The length of a trailing random part in names

	provisionerBlobCsiAzure string = "blob.csi.azure.com" // Use of azure/csi driver for blob in Azure storage account
)
