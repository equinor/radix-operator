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

type testScenario struct {
	name               string
	volumeMount        v1.RadixVolumeMount
	expectedVolumeName string
	expectedError      string
}

func Test_ValidFileCsiAzureVolumeMounts(t *testing.T) {
	testScenarios := []testScenario{
		{
			volumeMount: v1.RadixVolumeMount{
				Type:    v1.MountTypeFileCsiAzure,
				Name:    "volume1",
				Storage: "storageName1",
				Path:    "TestPath1",
			},
			expectedVolumeName: "csi-az-file-app-volume1-storageName1",
		},
		{
			volumeMount: v1.RadixVolumeMount{
				Type:    v1.MountTypeFileCsiAzure,
				Name:    "volume2",
				Storage: "storageName2",
				Path:    "TestPath2",
			},
			expectedVolumeName: "csi-az-file-app-volume2-storageName2",
		},
	}
	t.Run("One File CSI Azure volume mount ", func(t *testing.T) {
		t.Parallel()
		component := utils.NewDeployComponentBuilder().WithName("app").
			WithVolumeMounts([]v1.RadixVolumeMount{testScenarios[0].volumeMount}).
			BuildComponent()

		volumeMounts, err := GetRadixDeployComponentVolumeMounts(&component)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(volumeMounts))
		mount := volumeMounts[0]
		assert.Equal(t, testScenarios[0].expectedVolumeName, mount.Name)
		assert.Equal(t, testScenarios[0].volumeMount.Path, mount.MountPath)
	})
	t.Run("Multiple File CSI Azure volume mount", func(t *testing.T) {
		t.Parallel()
		component := utils.NewDeployComponentBuilder().WithName("app").
			WithVolumeMounts([]v1.RadixVolumeMount{testScenarios[0].volumeMount, testScenarios[1].volumeMount}).
			BuildComponent()

		volumeMounts, err := GetRadixDeployComponentVolumeMounts(&component)
		assert.Nil(t, err)
		for idx, testCase := range testScenarios {
			assert.Equal(t, 2, len(volumeMounts))
			assert.Equal(t, testCase.expectedVolumeName, volumeMounts[idx].Name)
			assert.Equal(t, testCase.volumeMount.Path, volumeMounts[idx].MountPath)
		}
	})
}

func Test_ValidBlobCsiAzureVolumeMounts(t *testing.T) {
	testScenarios := []testScenario{
		{
			volumeMount: v1.RadixVolumeMount{
				Type:    v1.MountTypeBlobCsiAzure,
				Name:    "volume1",
				Storage: "storageName1",
				Path:    "TestPath1",
			},
			expectedVolumeName: "csi-az-blob-app-volume1-storageName1",
		},
		{
			volumeMount: v1.RadixVolumeMount{
				Type:    v1.MountTypeBlobCsiAzure,
				Name:    "volume2",
				Storage: "storageName2",
				Path:    "TestPath2",
			},
			expectedVolumeName: "csi-az-blob-app-volume2-storageName2",
		},
	}
	t.Run("One Blob CSI Azure volume mount ", func(t *testing.T) {
		t.Parallel()
		component := utils.NewDeployComponentBuilder().WithName("app").
			WithVolumeMounts([]v1.RadixVolumeMount{testScenarios[0].volumeMount}).
			BuildComponent()

		volumeMounts, err := GetRadixDeployComponentVolumeMounts(&component)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(volumeMounts))
		mount := volumeMounts[0]
		assert.Equal(t, testScenarios[0].expectedVolumeName, mount.Name)
		assert.Equal(t, testScenarios[0].volumeMount.Path, mount.MountPath)
	})
	t.Run("Multiple Blob CSI Azure volume mount ", func(t *testing.T) {
		t.Parallel()
		component := utils.NewDeployComponentBuilder().WithName("app").
			WithVolumeMounts([]v1.RadixVolumeMount{testScenarios[0].volumeMount, testScenarios[1].volumeMount}).
			BuildComponent()

		volumeMounts, err := GetRadixDeployComponentVolumeMounts(&component)
		assert.Nil(t, err)
		for idx, testCase := range testScenarios {
			assert.Equal(t, 2, len(volumeMounts))
			assert.Equal(t, testCase.expectedVolumeName, volumeMounts[idx].Name)
			assert.Equal(t, testCase.volumeMount.Path, volumeMounts[idx].MountPath)
		}
	})
}

func Test_FailBlobCsiAzureVolumeMounts(t *testing.T) {
	testScenarios := []testScenario{
		{
			name: "Missed volume mount name",
			volumeMount: v1.RadixVolumeMount{
				Type:    v1.MountTypeBlobCsiAzure,
				Storage: "storageName1",
				Path:    "TestPath1",
			},
			expectedError: "name is empty for volume mount in the component app",
		},
		{
			name: "Missed volume mount storage",
			volumeMount: v1.RadixVolumeMount{
				Type: v1.MountTypeBlobCsiAzure,
				Name: "volume1",
				Path: "TestPath1",
			},
			expectedError: "storage is empty for volume mount volume1 in the component app",
		},
		{
			name: "Missed volume mount path",
			volumeMount: v1.RadixVolumeMount{
				Type:    v1.MountTypeBlobCsiAzure,
				Name:    "volume1",
				Storage: "storageName1",
			},
			expectedError: "path is empty for volume mount volume1 in the component app",
		},
	}
	t.Run("Failing Blob CSI Azure volume mount", func(t *testing.T) {
		t.Parallel()
		for _, testCase := range testScenarios {
			t.Logf("Test case: %s", testCase.name)
			component := utils.NewDeployComponentBuilder().WithName("app").
				WithVolumeMounts([]v1.RadixVolumeMount{
					testCase.volumeMount}).
				BuildComponent()

			_, err := GetRadixDeployComponentVolumeMounts(&component)
			assert.NotNil(t, err)
			assert.Equal(t, testCase.expectedError, err.Error())
		}
	})
}

func Test_BlobfuseAzureVolumeMounts(t *testing.T) {
	testScenarios := []testScenario{
		{
			volumeMount: v1.RadixVolumeMount{
				Type:      v1.MountTypeBlobCsiAzure,
				Name:      "volume1",
				Container: "storageName1",
				Path:      "TestPath1",
			},
			expectedVolumeName: "blobfuse-app-volume1",
		},
		{
			volumeMount: v1.RadixVolumeMount{
				Type:      v1.MountTypeBlobCsiAzure,
				Name:      "volume2",
				Container: "storageName2",
				Path:      "TestPath2",
			},
			expectedVolumeName: "blobfuse-app-volume2",
		},
	}
	t.Run("One Blobfuse Azure volume mount", func(t *testing.T) {
		t.Parallel()
		component := utils.NewDeployComponentBuilder().WithName("app").
			WithVolumeMounts([]v1.RadixVolumeMount{testScenarios[0].volumeMount}).
			BuildComponent()

		volumeMounts, err := GetRadixDeployComponentVolumeMounts(&component)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(volumeMounts))
		mount := volumeMounts[0]
		assert.Equal(t, testScenarios[0].expectedVolumeName, mount.Name)
		assert.Equal(t, testScenarios[0].volumeMount.Path, mount.MountPath)
	})
	t.Run("Multiple Blobfuse Azure volume mount", func(t *testing.T) {
		t.Parallel()
		component := utils.NewDeployComponentBuilder().WithName("app").
			WithVolumeMounts([]v1.RadixVolumeMount{testScenarios[0].volumeMount, testScenarios[1].volumeMount}).
			BuildComponent()

		volumeMounts, err := GetRadixDeployComponentVolumeMounts(&component)
		assert.Nil(t, err)
		for idx, testCase := range testScenarios {
			assert.Equal(t, 2, len(volumeMounts))
			assert.Equal(t, testCase.expectedVolumeName, volumeMounts[idx].Name)
			assert.Equal(t, testCase.volumeMount.Path, volumeMounts[idx].MountPath)
		}
	})
}
