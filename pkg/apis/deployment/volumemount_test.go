package deployment

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"testing"
)

type VolumeMountTestSuite struct {
	suite.Suite
	radixCommonDeployComponentFactories []v1.RadixCommonDeployComponentFactory
}

func TestVolumeMountTestSuite(t *testing.T) {
	suite.Run(t, new(VolumeMountTestSuite))
}

func (suite *VolumeMountTestSuite) SetupTest() {
	suite.radixCommonDeployComponentFactories = []v1.RadixCommonDeployComponentFactory{
		v1.RadixDeployComponentFactory{},
		v1.RadixDeployJobComponentFactory{},
	}
}

func (suite *VolumeMountTestSuite) Test_NoVolumeMounts() {
	suite.T().Run("app", func(t *testing.T) {
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {

			component := utils.NewDeployCommonComponentBuilder(factory).
				WithName("app").
				BuildComponent()

			volumeMounts, _ := GetRadixDeployComponentVolumeMounts(component)
			assert.Equal(t, 0, len(volumeMounts))
		}
	})
}

type testScenario struct {
	name               string
	volumeMount        v1.RadixVolumeMount
	expectedVolumeName string
	expectedError      string
}

func (suite *VolumeMountTestSuite) Test_ValidFileCsiAzureVolumeMounts() {
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
	suite.T().Run("One File CSI Azure volume mount ", func(t *testing.T) {
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			t.Logf("Test case %s for component %s", testScenarios[0].name, factory.GetTargetType())
			component := utils.NewDeployCommonComponentBuilder(factory).
				WithName("app").
				WithVolumeMounts([]v1.RadixVolumeMount{testScenarios[0].volumeMount}).
				BuildComponent()

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(component)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(volumeMounts))
			mount := volumeMounts[0]
			assert.Equal(t, testScenarios[0].expectedVolumeName, mount.Name)
			assert.Equal(t, testScenarios[0].volumeMount.Path, mount.MountPath)
		}
	})
	suite.T().Run("Multiple File CSI Azure volume mount", func(t *testing.T) {
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			component := utils.NewDeployCommonComponentBuilder(factory).
				WithName("app").
				WithVolumeMounts([]v1.RadixVolumeMount{testScenarios[0].volumeMount, testScenarios[1].volumeMount}).
				BuildComponent()

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(component)
			assert.Nil(t, err)
			for idx, testCase := range testScenarios {
				assert.Equal(t, 2, len(volumeMounts))
				assert.Equal(t, testCase.expectedVolumeName, volumeMounts[idx].Name)
				assert.Equal(t, testCase.volumeMount.Path, volumeMounts[idx].MountPath)
			}
		}
	})
}

func (suite *VolumeMountTestSuite) Test_ValidBlobCsiAzureVolumeMounts() {
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
	suite.T().Run("One Blob CSI Azure volume mount ", func(t *testing.T) {
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			t.Logf("Test case %s for component %s", testScenarios[0].name, factory.GetTargetType())
			component := utils.NewDeployCommonComponentBuilder(factory).WithName("app").
				WithVolumeMounts([]v1.RadixVolumeMount{testScenarios[0].volumeMount}).
				BuildComponent()

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(component)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(volumeMounts))
			mount := volumeMounts[0]
			assert.Equal(t, testScenarios[0].expectedVolumeName, mount.Name)
			assert.Equal(t, testScenarios[0].volumeMount.Path, mount.MountPath)
		}
	})
	suite.T().Run("Multiple Blob CSI Azure volume mount ", func(t *testing.T) {
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			t.Logf("Test case %s for component %s", testScenarios[0].name, factory.GetTargetType())
			component := utils.NewDeployCommonComponentBuilder(factory).
				WithName("app").
				WithVolumeMounts([]v1.RadixVolumeMount{testScenarios[0].volumeMount, testScenarios[1].volumeMount}).
				BuildComponent()

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(component)
			assert.Nil(t, err)
			for idx, testCase := range testScenarios {
				assert.Equal(t, 2, len(volumeMounts))
				assert.Equal(t, testCase.expectedVolumeName, volumeMounts[idx].Name)
				assert.Equal(t, testCase.volumeMount.Path, volumeMounts[idx].MountPath)
			}
		}
	})
}

func (suite *VolumeMountTestSuite) Test_FailBlobCsiAzureVolumeMounts() {
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
	suite.T().Run("Failing Blob CSI Azure volume mount", func(t *testing.T) {
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {

			for _, testCase := range testScenarios {
				t.Logf("Test case %s for component %s", testCase.name, factory.GetTargetType())
				component := utils.NewDeployCommonComponentBuilder(factory).
					WithName("app").
					WithVolumeMounts([]v1.RadixVolumeMount{
						testCase.volumeMount}).
					BuildComponent()

				_, err := GetRadixDeployComponentVolumeMounts(component)
				assert.NotNil(t, err)
				assert.Equal(t, testCase.expectedError, err.Error())
			}
		}
	})
}

//Blobfuse support has been deprecated, this test to be deleted, when Blobfuse logic is deleted
func (suite *VolumeMountTestSuite) Test_BlobfuseAzureVolumeMounts() {
	testScenarios := []testScenario{
		{
			volumeMount: v1.RadixVolumeMount{
				Type:      v1.MountTypeBlob,
				Name:      "volume1",
				Container: "storageName1",
				Path:      "TestPath1",
			},
			expectedVolumeName: "blobfuse-app-volume1",
		},
		{
			volumeMount: v1.RadixVolumeMount{
				Type:      v1.MountTypeBlob,
				Name:      "volume2",
				Container: "storageName2",
				Path:      "TestPath2",
			},
			expectedVolumeName: "blobfuse-app-volume2",
		},
	}
	suite.T().Run("One Blobfuse Azure volume mount", func(t *testing.T) {
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
	suite.T().Run("Multiple Blobfuse Azure volume mount", func(t *testing.T) {
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
