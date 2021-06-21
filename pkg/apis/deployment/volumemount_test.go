package deployment

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_NoVolumeMounts(t *testing.T) {
	t.Run("app", func(t *testing.T) {
		t.Parallel()
		component := utils.NewDeployComponentBuilder().WithName("app").BuildComponent()

		volumeMounts, _ := GetRadixDeployComponentVolumeMounts(&component)
		assert.Equal(t, 0, len(volumeMounts))
	})
}

func Test_CsiAzureVolumeMounts(t *testing.T) {
	t.Run("One Blob CSI Azure volume mount ", func(t *testing.T) {
		t.Parallel()
		component := utils.NewDeployComponentBuilder().WithName("app").
			WithVolumeMounts([]v1.RadixVolumeMount{
				{
					Type:    v1.MountTypeBlobCsiAzure,
					Name:    "volume1",
					Storage: "storageName1",
					Path:    "TestPath1",
				}}).
			BuildComponent()

		volumeMounts, err := GetRadixDeployComponentVolumeMounts(&component)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(volumeMounts))
		mount := volumeMounts[0]
		assert.Equal(t, "csi-az-blob-app-volume1-storageName1", mount.Name)
		assert.Equal(t, "TestPath1", mount.MountPath)
	})
	t.Run("Multiple Blob CSI Azure volume mount ", func(t *testing.T) {
		t.Parallel()
		component := utils.NewDeployComponentBuilder().WithName("app").
			WithVolumeMounts([]v1.RadixVolumeMount{
				{
					Type:    v1.MountTypeBlobCsiAzure,
					Name:    "volume1",
					Storage: "storageName1",
					Path:    "TestPath1",
				},
				{
					Type:    v1.MountTypeBlobCsiAzure,
					Name:    "volume2",
					Storage: "storageName2",
					Path:    "TestPath2",
				}}).
			BuildComponent()

		volumeMounts, err := GetRadixDeployComponentVolumeMounts(&component)
		assert.Nil(t, err)
		assert.Equal(t, 2, len(volumeMounts))
		assert.Equal(t, "csi-az-blob-app-volume1-storageName1", volumeMounts[0].Name)
		assert.Equal(t, "TestPath1", volumeMounts[0].MountPath)
		assert.Equal(t, "csi-az-blob-app-volume2-storageName2", volumeMounts[1].Name)
		assert.Equal(t, "TestPath2", volumeMounts[1].MountPath)
	})
	t.Run("Blob CSI Azure volume mount without Name", func(t *testing.T) {
		t.Parallel()
		component := utils.NewDeployComponentBuilder().WithName("app").
			WithVolumeMounts([]v1.RadixVolumeMount{
				{
					Type:    v1.MountTypeBlobCsiAzure,
					Storage: "storageName1",
					Path:    "TestPath1",
				}}).
			BuildComponent()

		_, err := GetRadixDeployComponentVolumeMounts(&component)
		assert.NotNil(t, err)
		assert.Equal(t, "name is empty for volume mount in the component app", err.Error())
	})
	t.Run("One File CSI Azure volume mount", func(t *testing.T) {
		t.Parallel()
		component := utils.NewDeployComponentBuilder().WithName("app").
			WithVolumeMounts([]v1.RadixVolumeMount{
				{
					Type:    v1.MountTypeFileCsiAzure,
					Name:    "volume1",
					Storage: "storageName1",
					Path:    "TestPath1",
				}}).
			BuildComponent()

		volumeMounts, err := GetRadixDeployComponentVolumeMounts(&component)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(volumeMounts))
		mount := volumeMounts[0]
		assert.Equal(t, "csi-az-file-app-volume1-storageName1", mount.Name)
		assert.Equal(t, "TestPath1", mount.MountPath)
	})
	t.Run("Multiple File CSI Azure volume mount", func(t *testing.T) {
		t.Parallel()
		component := utils.NewDeployComponentBuilder().WithName("app").
			WithVolumeMounts([]v1.RadixVolumeMount{
				{
					Type:    v1.MountTypeFileCsiAzure,
					Name:    "volume1",
					Storage: "storageName1",
					Path:    "TestPath1",
				},
				{
					Type:    v1.MountTypeFileCsiAzure,
					Name:    "volume2",
					Storage: "storageName2",
					Path:    "TestPath2",
				}}).
			BuildComponent()

		volumeMounts, err := GetRadixDeployComponentVolumeMounts(&component)
		assert.Nil(t, err)
		assert.Equal(t, 2, len(volumeMounts))
		assert.Equal(t, "csi-az-file-app-volume1-storageName1", volumeMounts[0].Name)
		assert.Equal(t, "TestPath1", volumeMounts[0].MountPath)
		assert.Equal(t, "csi-az-file-app-volume2-storageName2", volumeMounts[1].Name)
		assert.Equal(t, "TestPath2", volumeMounts[1].MountPath)
	})
}

func Test_BlobfuseVolumeMounts(t *testing.T) {
	t.Run("One Blobfuse volume mount ", func(t *testing.T) {
		t.Parallel()
		component := utils.NewDeployComponentBuilder().WithName("app").
			WithVolumeMounts([]v1.RadixVolumeMount{
				{
					Type:      v1.MountTypeBlob,
					Name:      "volume1",
					Container: "storageName1",
					Path:      "TestPath1",
				}}).
			BuildComponent()

		volumeMounts, err := GetRadixDeployComponentVolumeMounts(&component)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(volumeMounts))
		mount := volumeMounts[0]
		assert.Equal(t, "blobfuse-app-volume1", mount.Name)
		assert.Equal(t, "TestPath1", mount.MountPath)
	})
	t.Run("Multiple Blobfuse volume mount", func(t *testing.T) {
		t.Parallel()
		component := utils.NewDeployComponentBuilder().WithName("app").
			WithVolumeMounts([]v1.RadixVolumeMount{
				{
					Type:      v1.MountTypeBlob,
					Name:      "volume1",
					Container: "storageName1",
					Path:      "TestPath1",
				},
				{
					Type:    v1.MountTypeBlob,
					Name:    "volume2",
					Storage: "storageName2",
					Path:    "TestPath2",
				}}).
			BuildComponent()

		volumeMounts, err := GetRadixDeployComponentVolumeMounts(&component)
		assert.Nil(t, err)
		assert.Equal(t, 2, len(volumeMounts))
		assert.Equal(t, "blobfuse-app-volume1", volumeMounts[0].Name)
		assert.Equal(t, "TestPath1", volumeMounts[0].MountPath)
		assert.Equal(t, "blobfuse-app-volume2", volumeMounts[1].Name)
		assert.Equal(t, "TestPath2", volumeMounts[1].MountPath)
	})
}
