package volumemount

const (
	// csiVolumeSourceDriverSecretStore Driver name for the secret store
	csiVolumeSourceDriverSecretStore = "secrets-store.csi.k8s.io"
	// csiVolumeSourceVolumeAttributeSecretProviderClass Secret provider class volume attribute
	csiVolumeSourceVolumeAttributeSecretProviderClass = "secretProviderClass"

	csiPersistentVolumeClaimNameTemplate = "pvc-%s-%s-%s" // pvc-<componentname>-<volumemountname>-<ulid>
	csiPersistentVolumeNameTemplate      = "pv-%s-%s"     // pv-<appname>-<ulid>
	csiVolumeAttributeStorageAccount     = "storageAccount"
	csiVolumeAttributeClientID           = "clientID"
	csiVolumeAttributeResourceGroup      = "resourcegroup"
	csiVolumeAttributeSubscriptionId     = "subscriptionid"
	csiVolumeAttributeTenantId           = "tenantID"

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
