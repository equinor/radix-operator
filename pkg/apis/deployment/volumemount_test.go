package deployment

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"testing"
)

type VolumeMountTestSuite struct {
	suite.Suite
	radixCommonDeployComponentFactories []v1.RadixCommonDeployComponentFactory
	kubeclient                          kubernetes.Interface
}

type volumeMountTestScenario struct {
	name                          string
	volumeMount                   v1.RadixVolumeMount
	expectedVolumeName            string
	expectedError                 string
	expectedVolumeClaimNamePrefix string
}

func TestVolumeMountTestSuite(t *testing.T) {
	suite.Run(t, new(VolumeMountTestSuite))
}

func (suite *VolumeMountTestSuite) SetupSuite() {
	suite.radixCommonDeployComponentFactories = []v1.RadixCommonDeployComponentFactory{
		v1.RadixDeployComponentFactory{},
		v1.RadixDeployJobComponentFactory{},
	}
	suite.kubeclient = kubefake.NewSimpleClientset()
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

func (suite *VolumeMountTestSuite) Test_ValidFileCsiAzureVolumeMounts() {
	scenarios := []volumeMountTestScenario{
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
			t.Logf("Test case %s for component %s", scenarios[0].name, factory.GetTargetType())
			component := utils.NewDeployCommonComponentBuilder(factory).
				WithName("app").
				WithVolumeMounts([]v1.RadixVolumeMount{scenarios[0].volumeMount}).
				BuildComponent()

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(component)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(volumeMounts))
			mount := volumeMounts[0]
			assert.Equal(t, scenarios[0].expectedVolumeName, mount.Name)
			assert.Equal(t, scenarios[0].volumeMount.Path, mount.MountPath)
		}
	})
	suite.T().Run("Multiple File CSI Azure volume mount", func(t *testing.T) {
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			component := utils.NewDeployCommonComponentBuilder(factory).
				WithName("app").
				WithVolumeMounts([]v1.RadixVolumeMount{scenarios[0].volumeMount, scenarios[1].volumeMount}).
				BuildComponent()

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(component)
			assert.Nil(t, err)
			for idx, testCase := range scenarios {
				assert.Equal(t, 2, len(volumeMounts))
				assert.Equal(t, testCase.expectedVolumeName, volumeMounts[idx].Name)
				assert.Equal(t, testCase.volumeMount.Path, volumeMounts[idx].MountPath)
			}
		}
	})
}

func (suite *VolumeMountTestSuite) Test_ValidBlobCsiAzureVolumeMounts() {
	scenarios := []volumeMountTestScenario{
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
			t.Logf("Test case %s for component %s", scenarios[0].name, factory.GetTargetType())
			component := utils.NewDeployCommonComponentBuilder(factory).WithName("app").
				WithVolumeMounts([]v1.RadixVolumeMount{scenarios[0].volumeMount}).
				BuildComponent()

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(component)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(volumeMounts))
			mount := volumeMounts[0]
			assert.Equal(t, scenarios[0].expectedVolumeName, mount.Name)
			assert.Equal(t, scenarios[0].volumeMount.Path, mount.MountPath)
		}
	})
	suite.T().Run("Multiple Blob CSI Azure volume mount ", func(t *testing.T) {
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			t.Logf("Test case %s for component %s", scenarios[0].name, factory.GetTargetType())
			component := utils.NewDeployCommonComponentBuilder(factory).
				WithName("app").
				WithVolumeMounts([]v1.RadixVolumeMount{scenarios[0].volumeMount, scenarios[1].volumeMount}).
				BuildComponent()

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(component)
			assert.Nil(t, err)
			for idx, testCase := range scenarios {
				assert.Equal(t, 2, len(volumeMounts))
				assert.Equal(t, testCase.expectedVolumeName, volumeMounts[idx].Name)
				assert.Equal(t, testCase.volumeMount.Path, volumeMounts[idx].MountPath)
			}
		}
	})
}

func (suite *VolumeMountTestSuite) Test_FailBlobCsiAzureVolumeMounts() {
	scenarios := []volumeMountTestScenario{
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

			for _, testCase := range scenarios {
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
	scenarios := []volumeMountTestScenario{
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
			WithVolumeMounts([]v1.RadixVolumeMount{scenarios[0].volumeMount}).
			BuildComponent()

		volumeMounts, err := GetRadixDeployComponentVolumeMounts(&component)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(volumeMounts))
		mount := volumeMounts[0]
		assert.Equal(t, scenarios[0].expectedVolumeName, mount.Name)
		assert.Equal(t, scenarios[0].volumeMount.Path, mount.MountPath)
	})
	suite.T().Run("Multiple Blobfuse Azure volume mount", func(t *testing.T) {
		t.Parallel()
		component := utils.NewDeployComponentBuilder().WithName("app").
			WithVolumeMounts([]v1.RadixVolumeMount{scenarios[0].volumeMount, scenarios[1].volumeMount}).
			BuildComponent()

		volumeMounts, err := GetRadixDeployComponentVolumeMounts(&component)
		assert.Nil(t, err)
		for idx, testCase := range scenarios {
			assert.Equal(t, 2, len(volumeMounts))
			assert.Equal(t, testCase.expectedVolumeName, volumeMounts[idx].Name)
			assert.Equal(t, testCase.volumeMount.Path, volumeMounts[idx].MountPath)
		}
	})
}

func (suite *VolumeMountTestSuite) Test_GetVolumes() {
	namespace := "some-namespace"
	environment := "some-env"
	componentName := "some-component"
	suite.T().Run("No volumes in component", func(t *testing.T) {
		t.Parallel()
		volumes, err := GetVolumes(suite.kubeclient, namespace, environment, componentName, []v1.RadixVolumeMount{})
		assert.Nil(t, err)
		assert.Len(t, volumes, 0)
	})
	scenarios := []volumeMountTestScenario{
		{
			name: "Blob CSI Azure volume",
			volumeMount: v1.RadixVolumeMount{
				Type:    v1.MountTypeBlobCsiAzure,
				Name:    "volume1",
				Storage: "storage1",
				Path:    "path1",
				GID:     "1000",
			},
			expectedVolumeName:            "csi-az-blob-some-component-volume1-storage1",
			expectedVolumeClaimNamePrefix: "pvc-csi-az-blob-some-component-volume1-storage1",
		},
	}
	suite.T().Run("BlobCsiAzure volume", func(t *testing.T) {
		t.Parallel()
		mounts := []v1.RadixVolumeMount{scenarios[0].volumeMount}
		volumes, err := GetVolumes(suite.kubeclient, namespace, environment, componentName, mounts)
		assert.Nil(t, err)
		assert.Len(t, volumes, 1)
		volume := volumes[0]
		assert.Equal(t, scenarios[0].expectedVolumeName, volume.Name)
		assert.NotNil(t, volume.PersistentVolumeClaim)
		assert.Contains(t, volume.PersistentVolumeClaim.ClaimName, scenarios[0].expectedVolumeClaimNamePrefix)
	})
}
