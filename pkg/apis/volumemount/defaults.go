package volumemount

const (
	// CsiVolumeSourceDriverSecretStore Driver name for the secret store
	CsiVolumeSourceDriverSecretStore = "secrets-store.csi.k8s.io"
	// CsiVolumeSourceVolumeAttributeSecretProviderClass Secret provider class volume attribute
	CsiVolumeSourceVolumeAttributeSecretProviderClass = "secretProviderClass"

	csiPersistentVolumeClaimNameTemplate = "pvc-%s-%s"              // pvc-<volumename>-<randomstring5>
	csiPersistentVolumeNameTemplate      = "pv-radixvolumemount-%s" // pv-<guid>

	csiVolumeAttributeProtocolParameterFuse  = "fuse"  // Protocol "blobfuse"
	csiVolumeAttributeProtocolParameterFuse2 = "fuse2" // Protocol "blobfuse2"
	csiVolumeAttributeStorageAccount         = "storageAccount"
	csiVolumeAttributeClientID               = "clientID"
	csiVolumeAttributeResourceGroup          = "resourcegroup"
	csiVolumeAttributeSubscriptionId         = "subscriptionid"
	csiVolumeAttributeTenantId               = "tenantID"

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
