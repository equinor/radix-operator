package volumemount

import "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// GetBlobFuse2VolumeMountStorageAccount Get storage account name from BlobFuse2 volume mount
func GetBlobFuse2VolumeMountStorageAccount(volumeMount v1.RadixVolumeMount) string {
	if volumeMount.BlobFuse2 != nil {
		return volumeMount.BlobFuse2.StorageAccount
	}
	return ""
}
