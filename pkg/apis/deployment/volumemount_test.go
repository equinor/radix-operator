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

		volumeMounts := GetRadixDeployComponentVolumeMounts(&component)
		assert.Equal(t, 0, len(volumeMounts))
	})
}

func Test_BlobCsiAzureVolumeMounts(t *testing.T) {
	t.Run("Valid Blob CSI Azure volume mount ", func(t *testing.T) {
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

		volumeMounts := GetRadixDeployComponentVolumeMounts(&component)
		assert.Equal(t, 1, len(volumeMounts))
		mount := volumeMounts[0]
		assert.Equal(t, "csi-az-blob-app-volume1-storageName1", mount.Name)
		assert.Equal(t, "TestPath1", mount.MountPath)
	})
}
