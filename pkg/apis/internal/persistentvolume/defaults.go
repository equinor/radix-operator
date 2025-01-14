package persistentvolume

const (
	CsiPersistentVolumeClaimNameTemplate = "pvc-%s-%s"              // pvc-<volumename>-<randomstring5>
	CsiPersistentVolumeNameTemplate      = "pv-radixvolumemount-%s" // pv-<guid>

	CsiMountOptionGid                       = "gid"                 // Volume mount owner GroupID. Used when drivers do not honor fsGroup securityContext setting
	CsiMountOptionUid                       = "uid"                 // Volume mount owner UserID. Used instead of GroupID
	CsiMountOptionUseAdls                   = "use-adls"            // Use ADLS or Block Blob
	CsiMountOptionStreamingEnabled          = "streaming"           // Enable Streaming
	CsiMountOptionStreamingCache            = "stream-cache-mb"     // Limit total amount of data being cached in memory to conserve memory
	CsiMountOptionStreamingMaxBlocksPerFile = "max-blocks-per-file" // Maximum number of blocks to be cached in memory for streaming
	CsiMountOptionStreamingMaxBuffers       = "max-buffers"         // The total number of buffers to be cached in memory (in MB).
	CsiMountOptionStreamingBlockSize        = "block-size-mb"       // The size of each block to be cached in memory (in MB).
	CsiMountOptionStreamingBufferSize       = "buffer-size-mb"      // The size of each buffer to be cached in memory (in MB).

	CsiVolumeAttributeProtocolParameterFuse  = "fuse"  // Protocol "blobfuse"
	CsiVolumeAttributeProtocolParameterFuse2 = "fuse2" // Protocol "blobfuse2"
	CsiVolumeAttributeStorageAccount         = "storageAccount"
	CsiVolumeAttributeClientID               = "clientID"
	CsiVolumeAttributeResourceGroup          = "resourcegroup"

	CsiAnnotationProvisionedBy                      = "pv.kubernetes.io/provisioned-by"
	CsiAnnotationProvisionerDeletionSecretName      = "volume.kubernetes.io/provisioner-deletion-secret-name"
	CsiAnnotationProvisionerDeletionSecretNamespace = "volume.kubernetes.io/provisioner-deletion-secret-namespace"

	CsiVolumeSourceDriverSecretStore                  = "secrets-store.csi.k8s.io"
	CsiVolumeSourceVolumeAttributeSecretProviderClass = "secretProviderClass"

	CsiVolumeMountAttributePvName              = "csi.storage.k8s.io/pv/name"
	CsiVolumeMountAttributePvcName             = "csi.storage.k8s.io/pvc/name"
	CsiVolumeMountAttributePvcNamespace        = "csi.storage.k8s.io/pvc/namespace"
	CsiVolumeMountAttributeSecretNamespace     = "secretnamespace"
	CsiVolumeMountAttributeProtocol            = "protocol"      // Protocol
	CsiVolumeMountAttributeContainerName       = "containerName" // Container name - foc container storages
	CsiVolumeMountAttributeProvisionerIdentity = "storage.kubernetes.io/csiProvisionerIdentity"
)
