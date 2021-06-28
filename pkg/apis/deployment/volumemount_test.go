package deployment

import (
	"context"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type pvcTestScenario struct {
	volumeMountTestScenario
	pvc corev1.PersistentVolumeClaim
}

func TestVolumeMountTestSuite(t *testing.T) {
	suite.Run(t, new(VolumeMountTestSuite))
}

func (suite *VolumeMountTestSuite) SetupSuite() {
	suite.radixCommonDeployComponentFactories = []v1.RadixCommonDeployComponentFactory{
		v1.RadixDeployComponentFactory{},
		v1.RadixDeployJobComponentFactory{},
	}
}

func (suite *VolumeMountTestSuite) SetupTest() {
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
			volumeMount:        v1.RadixVolumeMount{Type: v1.MountTypeFileCsiAzure, Name: "volume1", Storage: "storageName1", Path: "TestPath1"},
			expectedVolumeName: "csi-az-file-app-volume1-storageName1",
		},
		{
			volumeMount:        v1.RadixVolumeMount{Type: v1.MountTypeFileCsiAzure, Name: "volume2", Storage: "storageName2", Path: "TestPath2"},
			expectedVolumeName: "csi-az-file-app-volume2-storageName2",
		},
	}
	suite.T().Run("One File CSI Azure volume mount ", func(t *testing.T) {
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			t.Logf("Test case '%s' for component '%s'", scenarios[0].name, factory.GetTargetType())
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
			volumeMount:        v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Name: "volume1", Storage: "storageName1", Path: "TestPath1"},
			expectedVolumeName: "csi-az-blob-app-volume1-storageName1",
		},
		{
			volumeMount:        v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Name: "volume2", Storage: "storageName2", Path: "TestPath2"},
			expectedVolumeName: "csi-az-blob-app-volume2-storageName2",
		},
	}
	suite.T().Run("One Blob CSI Azure volume mount ", func(t *testing.T) {
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			t.Logf("Test case '%s' for component '%s'", scenarios[0].name, factory.GetTargetType())
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
			t.Logf("Test case '%s' for component '%s'", scenarios[0].name, factory.GetTargetType())
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
			name:          "Missed volume mount name",
			volumeMount:   v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Storage: "storageName1", Path: "TestPath1"},
			expectedError: "name is empty for volume mount in the component app",
		},
		{
			name:          "Missed volume mount storage",
			volumeMount:   v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Name: "volume1", Path: "TestPath1"},
			expectedError: "storage is empty for volume mount volume1 in the component app",
		},
		{
			name:          "Missed volume mount path",
			volumeMount:   v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Name: "volume1", Storage: "storageName1"},
			expectedError: "path is empty for volume mount volume1 in the component app",
		},
	}
	suite.T().Run("Failing Blob CSI Azure volume mount", func(t *testing.T) {
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {

			for _, testCase := range scenarios {
				t.Logf("Test case '%s' for component '%s'", testCase.name, factory.GetTargetType())
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
			volumeMount:        v1.RadixVolumeMount{Type: v1.MountTypeBlob, Name: "volume1", Container: "storageName1", Path: "TestPath1"},
			expectedVolumeName: "blobfuse-app-volume1",
		},
		{
			volumeMount:        v1.RadixVolumeMount{Type: v1.MountTypeBlob, Name: "volume2", Container: "storageName2", Path: "TestPath2"},
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

func (suite *VolumeMountTestSuite) Test_GetNewVolumes() {
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
			name:                          "Blob CSI Azure volume",
			volumeMount:                   v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Name: "volume1", Storage: "storage1", Path: "path1", GID: "1000"},
			expectedVolumeName:            "csi-az-blob-some-component-volume1-storage1",
			expectedVolumeClaimNamePrefix: "pvc-csi-az-blob-some-component-volume1-storage1",
		},
		{
			name:                          "File CSI Azure volume",
			volumeMount:                   v1.RadixVolumeMount{Type: v1.MountTypeFileCsiAzure, Name: "volume1", Storage: "storage1", Path: "path1", GID: "1000"},
			expectedVolumeName:            "csi-az-file-some-component-volume1-storage1",
			expectedVolumeClaimNamePrefix: "pvc-csi-az-file-some-component-volume1-storage1",
		},
	}
	blobFuseScenario := volumeMountTestScenario{
		name:               "Blob Azure FlexVolume",
		volumeMount:        v1.RadixVolumeMount{Type: v1.MountTypeBlob, Name: "volume1", Container: "storage1", Path: "path1"},
		expectedVolumeName: "blobfuse-some-component-volume1",
	}
	suite.T().Run("CSI Azure volumes", func(t *testing.T) {
		t.Parallel()
		for _, scenario := range scenarios {
			t.Logf("Scenario '%s'", scenario.name)
			mounts := []v1.RadixVolumeMount{scenario.volumeMount}
			volumes, err := GetVolumes(suite.kubeclient, namespace, environment, componentName, mounts)
			assert.Nil(t, err)
			assert.Len(t, volumes, 1)
			volume := volumes[0]
			assert.Equal(t, scenario.expectedVolumeName, volume.Name)
			assert.NotNil(t, volume.PersistentVolumeClaim)
			assert.Contains(t, volume.PersistentVolumeClaim.ClaimName, scenario.expectedVolumeClaimNamePrefix)
		}
	})
	suite.T().Run("Blobfuse-flex volume", func(t *testing.T) {
		t.Parallel()
		mounts := []v1.RadixVolumeMount{blobFuseScenario.volumeMount}
		volumes, err := GetVolumes(suite.kubeclient, namespace, environment, componentName, mounts)
		assert.Nil(t, err)
		assert.Len(t, volumes, 1)
		volume := volumes[0]
		assert.Equal(t, blobFuseScenario.expectedVolumeName, volume.Name)
		assert.Nil(t, volume.PersistentVolumeClaim)
		assert.NotNil(t, volume.FlexVolume)
		assert.Equal(t, "azure/blobfuse", volume.FlexVolume.Driver)
		assert.Equal(t, "volume1", volume.FlexVolume.Options["name"])
		assert.Equal(t, "storage1", volume.FlexVolume.Options["container"])
		assert.Equal(t, "--file-cache-timeout-in-seconds=120", volume.FlexVolume.Options["mountoptions"])
		assert.Equal(t, "/tmp/some-namespace/some-component/some-env/blob/volume1/storage1", volume.FlexVolume.Options["tmppath"])
	})
	suite.T().Run("CSI Azure and Blobfuse-flex volumes", func(t *testing.T) {
		t.Parallel()
		for _, scenario := range append(scenarios, blobFuseScenario) {
			mounts := []v1.RadixVolumeMount{scenario.volumeMount}
			volumes, err := GetVolumes(suite.kubeclient, namespace, environment, componentName, mounts)
			assert.Nil(t, err)
			assert.Len(t, volumes, 1)
			volume := volumes[0]
			assert.Equal(t, scenario.expectedVolumeName, volume.Name)
			assert.Equal(t, len(scenario.expectedVolumeClaimNamePrefix) > 0, volume.PersistentVolumeClaim != nil)
		}
	})
	suite.T().Run("Unsupported volume type", func(t *testing.T) {
		t.Parallel()
		mounts := []v1.RadixVolumeMount{
			v1.RadixVolumeMount{Type: "unsupported-type", Name: "volume1", Container: "storage1", Path: "path1"},
		}
		volumes, err := GetVolumes(suite.kubeclient, namespace, environment, componentName, mounts)
		assert.Len(t, volumes, 0)
		assert.NotNil(t, err)
		assert.Equal(t, "unsupported volume type unsupported-type", err.Error())
	})
}

func (suite *VolumeMountTestSuite) Test_GetExistingCsiVolumes() {
	namespace := "some-namespace"
	environment := "some-env"
	componentName := "some-component"
	volumeMountName := "volume1"
	basePvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-pvc-name",
			Namespace: namespace,
			Labels: map[string]string{
				kube.RadixAppLabel:       appName,
				kube.RadixComponentLabel: componentName,
				//kube.RadixMountTypeLabel:       string(mountType),//set below
				kube.RadixVolumeMountNameLabel: volumeMountName,
			},
		},
	}
	pvcBlobAzure := modifyPvc(basePvc, func(v *corev1.PersistentVolumeClaim) {
		v.ObjectMeta.Labels[kube.RadixMountTypeLabel] = string(v1.MountTypeBlobCsiAzure)
	})
	pvcFileAzure := modifyPvc(basePvc, func(v *corev1.PersistentVolumeClaim) {
		v.ObjectMeta.Labels[kube.RadixMountTypeLabel] = string(v1.MountTypeFileCsiAzure)
	})
	scenarioBlobAzure := volumeMountTestScenario{
		name:                          "Blob CSI Azure volume",
		volumeMount:                   v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Name: volumeMountName, Storage: "storage1", Path: "path1", GID: "1000"},
		expectedVolumeName:            "csi-az-blob-some-component-volume1-storage1",
		expectedVolumeClaimNamePrefix: "pvc-csi-az-blob-some-component-volume1-storage1",
	}
	scenarioFileAzure := volumeMountTestScenario{
		name:                          "File CSI Azure volume",
		volumeMount:                   v1.RadixVolumeMount{Type: v1.MountTypeFileCsiAzure, Name: volumeMountName, Storage: "storage1", Path: "path1", GID: "1000"},
		expectedVolumeName:            "csi-az-file-some-component-volume1-storage1",
		expectedVolumeClaimNamePrefix: "pvc-csi-az-file-some-component-volume1-storage1",
	}
	scenarios := []pvcTestScenario{
		{
			volumeMountTestScenario: scenarioBlobAzure,
			pvc:                     modifyPvc(pvcBlobAzure, func(v *corev1.PersistentVolumeClaim) { v.Status.Phase = corev1.ClaimBound }),
		},
		{
			volumeMountTestScenario: scenarioBlobAzure,
			pvc:                     modifyPvc(pvcBlobAzure, func(v *corev1.PersistentVolumeClaim) { v.Status.Phase = corev1.ClaimPending }),
		},
		{
			volumeMountTestScenario: scenarioFileAzure,
			pvc:                     modifyPvc(pvcFileAzure, func(v *corev1.PersistentVolumeClaim) { v.Status.Phase = corev1.ClaimBound }),
		},
		{
			volumeMountTestScenario: scenarioFileAzure,
			pvc:                     modifyPvc(pvcFileAzure, func(v *corev1.PersistentVolumeClaim) { v.Status.Phase = corev1.ClaimPending }),
		},
	}

	suite.T().Run("CSI Azure volumes", func(t *testing.T) {
		t.Parallel()
		for _, scenario := range scenarios {
			t.Logf("Scenario '%s' for volume mount type '%s', PVC status phase '%v'", scenario.name, string(scenario.volumeMount.Type), scenario.pvc.Status.Phase)
			suite.kubeclient.CoreV1().PersistentVolumeClaims(namespace).Create(context.TODO(), &scenario.pvc, metav1.CreateOptions{})

			mounts := []v1.RadixVolumeMount{scenario.volumeMount}
			volumes, err := GetVolumes(suite.kubeclient, namespace, environment, componentName, mounts)
			assert.Nil(t, err)
			assert.Len(t, volumes, 1)
			volume := volumes[0]
			assert.Equal(t, scenario.expectedVolumeName, volume.Name)
			assert.NotNil(t, volume.PersistentVolumeClaim)
			assert.Equal(t, volume.PersistentVolumeClaim.ClaimName, scenario.pvc.Name)
		}
	})
}

func modifyPvc(pvc corev1.PersistentVolumeClaim, with func(*corev1.PersistentVolumeClaim)) corev1.PersistentVolumeClaim {
	with(&pvc)
	return pvc
}
