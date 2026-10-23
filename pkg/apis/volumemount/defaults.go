package volumemount

const (
	// CsiVolumeSourceDriverSecretStore Driver name for the secret store
	CsiVolumeSourceDriverSecretStore = "secrets-store.csi.k8s.io"
	// CsiVolumeSourceVolumeAttributeSecretProviderClass Secret provider class volume attribute
	CsiVolumeSourceVolumeAttributeSecretProviderClass = "secretProviderClass"

	csiPersistentVolumeClaimNameTemplate = "pvc-%s-%s"              // pvc-<volumename>-<hash>
	csiPersistentVolumeNameTemplate      = "pv-radixvolumemount-%s" // pv-radixvolumemount-<hash>
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

	// legacyMountTypeAzureBlob is the mount type label value of credential secrets for the removed azure-blob volume mount type.
	// Secrets with this label can still exist, and must be garbage collected.
	legacyMountTypeAzureBlob = "azure-blob"

	provisionerBlobCsiAzure string = "blob.csi.azure.com" // Use of azure/csi driver for blob in Azure storage account
)
