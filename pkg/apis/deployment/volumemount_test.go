package deployment

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type VolumeMountTestSuite struct {
	suite.Suite
	radixCommonDeployComponentFactories []v1.RadixCommonDeployComponentFactory
}

type TestEnv struct {
	kubeclient           kubernetes.Interface
	radixclient          radixclient.Interface
	secretproviderclient secretProviderClient.Interface
	prometheusclient     prometheusclient.Interface
	kubeUtil             *kube.Kube
}

type volumeMountTestScenario struct {
	name                  string
	radixVolumeMount      v1.RadixVolumeMount
	expectedVolumeName    string
	expectedError         string
	expectedPvcNamePrefix string
}

type deploymentVolumesTestScenario struct {
	name                                string
	props                               expectedPvcScProperties
	radixVolumeMounts                   []v1.RadixVolumeMount
	volumes                             []corev1.Volume
	existingStorageClassesBeforeTestRun []storagev1.StorageClass
	existingPvcsBeforeTestRun           []corev1.PersistentVolumeClaim
	existingStorageClassesAfterTestRun  []storagev1.StorageClass
	existingPvcsAfterTestRun            []corev1.PersistentVolumeClaim
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

func getTestEnv() TestEnv {
	testEnv := TestEnv{
		kubeclient:           kubefake.NewSimpleClientset(),
		radixclient:          radix.NewSimpleClientset(),
		secretproviderclient: secretproviderfake.NewSimpleClientset(),
		prometheusclient:     prometheusfake.NewSimpleClientset(),
	}
	kubeUtil, _ := kube.New(testEnv.kubeclient, testEnv.radixclient, testEnv.secretproviderclient)
	testEnv.kubeUtil = kubeUtil
	return testEnv
}

func getDeployment(testEnv TestEnv) *Deployment {
	return &Deployment{
		kubeclient:              testEnv.kubeclient,
		radixclient:             testEnv.radixclient,
		kubeutil:                testEnv.kubeUtil,
		prometheusperatorclient: testEnv.prometheusclient,
	}
}

func (suite *VolumeMountTestSuite) Test_NoVolumeMounts() {
	suite.T().Run("app", func(t *testing.T) {
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {

			component := utils.NewDeployCommonComponentBuilder(factory).
				WithName("app").
				BuildComponent()

			volumeMounts, _ := GetRadixDeployComponentVolumeMounts(component, "")
			assert.Equal(t, 0, len(volumeMounts))
		}
	})
}

func (suite *VolumeMountTestSuite) Test_ValidFileCsiAzureVolumeMounts() {
	scenarios := []volumeMountTestScenario{
		{
			radixVolumeMount:   v1.RadixVolumeMount{Type: v1.MountTypeFileCsiAzure, Name: "volume1", Storage: "storageName1", Path: "TestPath1"},
			expectedVolumeName: "csi-az-file-app-volume1-storageName1",
		},
		{
			radixVolumeMount:   v1.RadixVolumeMount{Type: v1.MountTypeFileCsiAzure, Name: "volume2", Storage: "storageName2", Path: "TestPath2"},
			expectedVolumeName: "csi-az-file-app-volume2-storageName2",
		},
	}
	suite.T().Run("One File CSI Azure volume mount ", func(t *testing.T) {
		expectedVolumeCount := map[v1.RadixComponentType]int{
			v1.RadixComponentTypeComponent:    1,
			v1.RadixComponentTypeJobScheduler: 0,
		}
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			t.Logf("Test case %s for component %s", scenarios[0].name, factory.GetTargetType())
			component := utils.NewDeployCommonComponentBuilder(factory).
				WithName("app").
				WithVolumeMounts([]v1.RadixVolumeMount{scenarios[0].radixVolumeMount}).
				BuildComponent()

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(component, "")
			assert.Nil(t, err)
			assert.Equal(t, expectedVolumeCount[component.GetType()], len(volumeMounts))
			if len(volumeMounts) > 0 {
				mount := volumeMounts[0]
				assert.Equal(t, scenarios[0].expectedVolumeName, mount.Name)
				assert.Equal(t, scenarios[0].radixVolumeMount.Path, mount.MountPath)
			}
		}
	})
	suite.T().Run("Multiple File CSI Azure volume mount", func(t *testing.T) {
		expectedVolumeCount := map[v1.RadixComponentType]int{
			v1.RadixComponentTypeComponent:    2,
			v1.RadixComponentTypeJobScheduler: 0,
		}
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			component := utils.NewDeployCommonComponentBuilder(factory).
				WithName("app").
				WithVolumeMounts([]v1.RadixVolumeMount{scenarios[0].radixVolumeMount, scenarios[1].radixVolumeMount}).
				BuildComponent()

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(component, "")
			assert.Nil(t, err)
			for idx, testCase := range scenarios {
				assert.Equal(t, expectedVolumeCount[component.GetType()], len(volumeMounts))
				if len(volumeMounts) > 0 {
					assert.Equal(t, testCase.expectedVolumeName, volumeMounts[idx].Name)
					assert.Equal(t, testCase.radixVolumeMount.Path, volumeMounts[idx].MountPath)
				}
			}
		}
	})
}

func (suite *VolumeMountTestSuite) Test_ValidBlobCsiAzureVolumeMounts() {
	scenarios := []volumeMountTestScenario{
		{
			radixVolumeMount:   v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Name: "volume1", Storage: "storageName1", Path: "TestPath1"},
			expectedVolumeName: "csi-az-blob-app-volume1-storageName1",
		},
		{
			radixVolumeMount:   v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Name: "volume2", Storage: "storageName2", Path: "TestPath2"},
			expectedVolumeName: "csi-az-blob-app-volume2-storageName2",
		},
	}
	suite.T().Run("One Blob CSI Azure volume mount ", func(t *testing.T) {
		expectedVolumeCount := map[v1.RadixComponentType]int{
			v1.RadixComponentTypeComponent:    1,
			v1.RadixComponentTypeJobScheduler: 0,
		}
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			t.Logf("Test case %s for component %s", scenarios[0].name, factory.GetTargetType())
			component := utils.NewDeployCommonComponentBuilder(factory).WithName("app").
				WithVolumeMounts([]v1.RadixVolumeMount{scenarios[0].radixVolumeMount}).
				BuildComponent()

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(component, "")
			assert.Nil(t, err)
			assert.Equal(t, expectedVolumeCount[component.GetType()], len(volumeMounts))
			if len(volumeMounts) > 0 {
				mount := volumeMounts[0]
				assert.Equal(t, scenarios[0].expectedVolumeName, mount.Name)
				assert.Equal(t, scenarios[0].radixVolumeMount.Path, mount.MountPath)
			}
		}
	})
	suite.T().Run("Multiple Blob CSI Azure volume mount ", func(t *testing.T) {
		expectedVolumeCount := map[v1.RadixComponentType]int{
			v1.RadixComponentTypeComponent:    2,
			v1.RadixComponentTypeJobScheduler: 0,
		}
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			t.Logf("Test case %s for component %s", scenarios[0].name, factory.GetTargetType())
			component := utils.NewDeployCommonComponentBuilder(factory).
				WithName("app").
				WithVolumeMounts([]v1.RadixVolumeMount{scenarios[0].radixVolumeMount, scenarios[1].radixVolumeMount}).
				BuildComponent()

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(component, "")
			assert.Nil(t, err)
			for idx, testCase := range scenarios {
				assert.Equal(t, expectedVolumeCount[component.GetType()], len(volumeMounts))
				if len(volumeMounts) > 0 {
					assert.Equal(t, testCase.expectedVolumeName, volumeMounts[idx].Name)
					assert.Equal(t, testCase.radixVolumeMount.Path, volumeMounts[idx].MountPath)
				}
			}
		}
	})
}

func (suite *VolumeMountTestSuite) Test_FailBlobCsiAzureVolumeMounts() {
	scenarios := []volumeMountTestScenario{
		{
			name:             "Missed volume mount name",
			radixVolumeMount: v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Storage: "storageName1", Path: "TestPath1"},
			expectedError:    "name is empty for volume mount in the component app",
		},
		{
			name:             "Missed volume mount storage",
			radixVolumeMount: v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Name: "volume1", Path: "TestPath1"},
			expectedError:    "storage is empty for volume mount volume1 in the component app",
		},
		{
			name:             "Missed volume mount path",
			radixVolumeMount: v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Name: "volume1", Storage: "storageName1"},
			expectedError:    "path is empty for volume mount volume1 in the component app",
		},
	}
	suite.T().Run("Failing Blob CSI Azure volume mount", func(t *testing.T) {
		expectedError := map[v1.RadixComponentType]bool{
			v1.RadixComponentTypeComponent:    true,
			v1.RadixComponentTypeJobScheduler: false,
		}
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			for _, testCase := range scenarios {
				t.Logf("Test case %s for component %s", testCase.name, factory.GetTargetType())
				component := utils.NewDeployCommonComponentBuilder(factory).
					WithName("app").
					WithVolumeMounts([]v1.RadixVolumeMount{
						testCase.radixVolumeMount}).
					BuildComponent()

				_, err := GetRadixDeployComponentVolumeMounts(component, "")
				if expectedError[component.GetType()] {
					assert.NotNil(t, err)
					assert.Equal(t, testCase.expectedError, err.Error())
				} else {
					assert.Nil(t, err)
				}
			}
		}
	})
}

//Blobfuse support has been deprecated, this test to be deleted, when Blobfuse logic is deleted
func (suite *VolumeMountTestSuite) Test_BlobfuseAzureVolumeMounts() {
	scenarios := []volumeMountTestScenario{
		{
			radixVolumeMount:   v1.RadixVolumeMount{Type: v1.MountTypeBlob, Name: "volume1", Container: "storageName1", Path: "TestPath1"},
			expectedVolumeName: "blobfuse-app-volume1",
		},
		{
			radixVolumeMount:   v1.RadixVolumeMount{Type: v1.MountTypeBlob, Name: "volume2", Container: "storageName2", Path: "TestPath2"},
			expectedVolumeName: "blobfuse-app-volume2",
		},
	}
	suite.T().Run("One Blobfuse Azure volume mount", func(t *testing.T) {
		t.Parallel()
		component := utils.NewDeployComponentBuilder().WithName("app").
			WithVolumeMounts([]v1.RadixVolumeMount{scenarios[0].radixVolumeMount}).
			BuildComponent()

		volumeMounts, err := GetRadixDeployComponentVolumeMounts(&component, "")
		assert.Nil(t, err)
		assert.Equal(t, 1, len(volumeMounts))
		mount := volumeMounts[0]
		assert.Equal(t, scenarios[0].expectedVolumeName, mount.Name)
		assert.Equal(t, scenarios[0].radixVolumeMount.Path, mount.MountPath)
	})
	suite.T().Run("Multiple Blobfuse Azure volume mount", func(t *testing.T) {
		t.Parallel()
		component := utils.NewDeployComponentBuilder().WithName("app").
			WithVolumeMounts([]v1.RadixVolumeMount{scenarios[0].radixVolumeMount, scenarios[1].radixVolumeMount}).
			BuildComponent()

		volumeMounts, err := GetRadixDeployComponentVolumeMounts(&component, "")
		assert.Nil(t, err)
		for idx, testCase := range scenarios {
			assert.Equal(t, 2, len(volumeMounts))
			assert.Equal(t, testCase.expectedVolumeName, volumeMounts[idx].Name)
			assert.Equal(t, testCase.radixVolumeMount.Path, volumeMounts[idx].MountPath)
		}
	})
}

func (suite *VolumeMountTestSuite) Test_GetNewVolumes() {
	namespace := "some-namespace"
	environment := "some-env"
	componentName := "some-component"
	suite.T().Run("No volumes in component", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts([]v1.RadixVolumeMount{}).BuildComponent()
		volumes, err := GetVolumes(testEnv.kubeclient, testEnv.kubeUtil, namespace, environment, &component, "")
		assert.Nil(t, err)
		assert.Len(t, volumes, 0)
	})
	scenarios := []volumeMountTestScenario{
		{
			name:                  "Blob CSI Azure volume",
			radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Name: "volume1", Storage: "storage1", Path: "path1", GID: "1000"},
			expectedVolumeName:    "csi-az-blob-some-component-volume1-storage1",
			expectedPvcNamePrefix: "pvc-csi-az-blob-some-component-volume1-storage1",
		},
		{
			name:                  "File CSI Azure volume",
			radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeFileCsiAzure, Name: "volume1", Storage: "storage1", Path: "path1", GID: "1000"},
			expectedVolumeName:    "csi-az-file-some-component-volume1-storage1",
			expectedPvcNamePrefix: "pvc-csi-az-file-some-component-volume1-storage1",
		},
	}
	blobFuseScenario := volumeMountTestScenario{
		name:               "Blob Azure FlexVolume",
		radixVolumeMount:   v1.RadixVolumeMount{Type: v1.MountTypeBlob, Name: "volume1", Container: "storage1", Path: "path1"},
		expectedVolumeName: "blobfuse-some-component-volume1",
	}
	suite.T().Run("CSI Azure volumes", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		for _, scenario := range scenarios {
			t.Logf("Scenario %s", scenario.name)
			component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts([]v1.RadixVolumeMount{scenario.radixVolumeMount}).BuildComponent()
			volumes, err := GetVolumes(testEnv.kubeclient, testEnv.kubeUtil, namespace, environment, &component, "")
			assert.Nil(t, err)
			assert.Len(t, volumes, 1)
			volume := volumes[0]
			assert.Equal(t, scenario.expectedVolumeName, volume.Name)
			assert.NotNil(t, volume.PersistentVolumeClaim)
			assert.Contains(t, volume.PersistentVolumeClaim.ClaimName, scenario.expectedPvcNamePrefix)
		}
	})
	suite.T().Run("Blobfuse-flex volume", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts([]v1.RadixVolumeMount{blobFuseScenario.radixVolumeMount}).BuildComponent()
		volumes, err := GetVolumes(testEnv.kubeclient, testEnv.kubeUtil, namespace, environment, &component, "")
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
		testEnv := getTestEnv()
		for _, scenario := range append(scenarios, blobFuseScenario) {
			component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts([]v1.RadixVolumeMount{scenario.radixVolumeMount}).BuildComponent()
			volumes, err := GetVolumes(testEnv.kubeclient, testEnv.kubeUtil, namespace, environment, &component, "")
			assert.Nil(t, err)
			assert.Len(t, volumes, 1)
			volume := volumes[0]
			assert.Equal(t, scenario.expectedVolumeName, volume.Name)
			assert.Equal(t, len(scenario.expectedPvcNamePrefix) > 0, volume.PersistentVolumeClaim != nil)
		}
	})
	suite.T().Run("Unsupported volume type", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		mounts := []v1.RadixVolumeMount{
			{Type: "unsupported-type", Name: "volume1", Container: "storage1", Path: "path1"},
		}
		component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts(mounts).BuildComponent()
		volumes, err := GetVolumes(testEnv.kubeclient, testEnv.kubeUtil, namespace, environment, &component, "")
		assert.Len(t, volumes, 0)
		assert.NotNil(t, err)
		assert.Equal(t, "unsupported volume type unsupported-type", err.Error())
	})
}

func (suite *VolumeMountTestSuite) Test_GetCsiVolumesWithExistingPvcs() {
	namespace := "some-namespace"
	environment := "some-env"
	componentName := "some-component"
	scenarios := []pvcTestScenario{
		{
			volumeMountTestScenario: volumeMountTestScenario{
				name:                  "Blob CSI Azure volume, PVS phase: Bound",
				radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Name: "volume1", Storage: "storage1", Path: "path1", GID: "1000"},
				expectedVolumeName:    "csi-az-blob-some-component-volume1-storage1",
				expectedPvcNamePrefix: "existing-blob-pvc-name1",
			},
			pvc: createPvc(namespace, componentName, v1.MountTypeBlobCsiAzure, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Name = "existing-blob-pvc-name1"
				pvc.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = "volume1"
				pvc.Status.Phase = corev1.ClaimBound
			}),
		},
		{
			volumeMountTestScenario: volumeMountTestScenario{
				name:                  "Blob CSI Azure volume, PVS phase: Pending",
				radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Name: "volume2", Storage: "storage2", Path: "path2", GID: "1000"},
				expectedVolumeName:    "csi-az-blob-some-component-volume2-storage2",
				expectedPvcNamePrefix: "existing-blob-pvc-name2",
			},
			pvc: createPvc(namespace, componentName, v1.MountTypeBlobCsiAzure, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Name = "existing-blob-pvc-name2"
				pvc.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = "volume2"
				pvc.Status.Phase = corev1.ClaimPending
			}),
		},
		{
			volumeMountTestScenario: volumeMountTestScenario{
				name:                  "File CSI Azure volume, PVS phase: Bound",
				radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeFileCsiAzure, Name: "volume3", Storage: "storage3", Path: "path3", GID: "1000"},
				expectedVolumeName:    "csi-az-file-some-component-volume3-storage3",
				expectedPvcNamePrefix: "existing-file-pvc-name1",
			},
			pvc: createPvc(namespace, componentName, v1.MountTypeFileCsiAzure, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Name = "existing-file-pvc-name1"
				pvc.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = "volume3"
				pvc.Status.Phase = corev1.ClaimBound
			}),
		},
		{
			volumeMountTestScenario: volumeMountTestScenario{
				name:                  "File CSI Azure volume, PVS phase: Pending",
				radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeFileCsiAzure, Name: "volume4", Storage: "storage4", Path: "path4", GID: "1000"},
				expectedVolumeName:    "csi-az-file-some-component-volume4-storage4",
				expectedPvcNamePrefix: "existing-file-pvc-name2",
			},
			pvc: createPvc(namespace, componentName, v1.MountTypeFileCsiAzure, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Name = "existing-file-pvc-name2"
				pvc.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = "volume4"
				pvc.Status.Phase = corev1.ClaimBound
			}),
		},
	}

	suite.T().Run("CSI Azure volumes with existing PVC", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		for _, scenario := range scenarios {
			t.Logf("Scenario %s for volume mount type %s, PVC status phase '%v'", scenario.name, string(scenario.radixVolumeMount.Type), scenario.pvc.Status.Phase)
			_, _ = testEnv.kubeclient.CoreV1().PersistentVolumeClaims(namespace).Create(context.TODO(), &scenario.pvc, metav1.CreateOptions{})

			component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts([]v1.RadixVolumeMount{scenario.radixVolumeMount}).BuildComponent()
			volumes, err := GetVolumes(testEnv.kubeclient, testEnv.kubeUtil, namespace, environment, &component, "")
			assert.Nil(t, err)
			assert.Len(t, volumes, 1)
			assert.Equal(t, scenario.expectedVolumeName, volumes[0].Name)
			assert.NotNil(t, volumes[0].PersistentVolumeClaim)
			assert.Equal(t, volumes[0].PersistentVolumeClaim.ClaimName, scenario.pvc.Name)
		}
	})

	suite.T().Run("CSI Azure volumes with no existing PVC", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		for _, scenario := range scenarios {
			t.Logf("Scenario %s for volume mount type %s, PVC status phase '%v'", scenario.name, string(scenario.radixVolumeMount.Type), scenario.pvc.Status.Phase)

			component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts([]v1.RadixVolumeMount{scenario.radixVolumeMount}).BuildComponent()
			volumes, err := GetVolumes(testEnv.kubeclient, testEnv.kubeUtil, namespace, environment, &component, "")
			assert.Nil(t, err)
			assert.Len(t, volumes, 1)
			assert.Equal(t, scenario.expectedVolumeName, volumes[0].Name)
			assert.NotNil(t, volumes[0].PersistentVolumeClaim)
			assert.NotEqual(t, volumes[0].PersistentVolumeClaim.ClaimName, scenario.pvc.Name)
			assert.NotContains(t, volumes[0].PersistentVolumeClaim.ClaimName, scenario.expectedPvcNamePrefix)
		}
	})
}

func (suite *VolumeMountTestSuite) Test_GetVolumesForComponent() {
	appName := "any-app"
	environment := "some-env"
	namespace := fmt.Sprintf("%s-%s", appName, environment)
	componentName := "some-component"
	scenarios := []pvcTestScenario{
		{
			volumeMountTestScenario: volumeMountTestScenario{
				name:                  "Blob CSI Azure volume, Status phase: Bound",
				radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Name: "blob-volume1", Storage: "storage1", Path: "path1", GID: "1000"},
				expectedVolumeName:    "csi-az-blob-some-component-blob-volume1-storage1",
				expectedPvcNamePrefix: "pvc-csi-az-blob-some-component-blob-volume1-storage1",
			},
			pvc: createPvc(namespace, componentName, v1.MountTypeBlobCsiAzure, func(pvc *corev1.PersistentVolumeClaim) { pvc.Status.Phase = corev1.ClaimBound }),
		},
		{
			volumeMountTestScenario: volumeMountTestScenario{
				name:                  "Blob CSI Azure volume, Status phase: Pending",
				radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Name: "blob-volume2", Storage: "storage2", Path: "path2", GID: "1000"},
				expectedVolumeName:    "csi-az-blob-some-component-blob-volume2-storage2",
				expectedPvcNamePrefix: "pvc-csi-az-blob-some-component-blob-volume2-storage2",
			},
			pvc: createPvc(namespace, componentName, v1.MountTypeBlobCsiAzure, func(pvc *corev1.PersistentVolumeClaim) { pvc.Status.Phase = corev1.ClaimPending }),
		},
		{
			volumeMountTestScenario: volumeMountTestScenario{
				name:                  "File CSI Azure volume, Status phase: Bound",
				radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeFileCsiAzure, Name: "file-volume1", Storage: "storage3", Path: "path3", GID: "1000"},
				expectedVolumeName:    "csi-az-file-some-component-file-volume1-storage3",
				expectedPvcNamePrefix: "pvc-csi-az-file-some-component-file-volume1-storage3",
			},
			pvc: createPvc(namespace, componentName, v1.MountTypeFileCsiAzure, func(pvc *corev1.PersistentVolumeClaim) { pvc.Status.Phase = corev1.ClaimBound }),
		},
		{
			volumeMountTestScenario: volumeMountTestScenario{
				name:                  "File CSI Azure volume, Status phase: Pending",
				radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeFileCsiAzure, Name: "file-volume2", Storage: "storage4", Path: "path4", GID: "1000"},
				expectedVolumeName:    "csi-az-file-some-component-file-volume2-storage4",
				expectedPvcNamePrefix: "pvc-csi-az-file-some-component-file-volume2-storage4",
			},
			pvc: createPvc(namespace, componentName, v1.MountTypeFileCsiAzure, func(pvc *corev1.PersistentVolumeClaim) { pvc.Status.Phase = corev1.ClaimPending }),
		},
	}

	suite.T().Run("No volumes", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		deployment := getDeployment(testEnv)
		for _, factory := range suite.radixCommonDeployComponentFactories {
			t.Logf("Test case for component %s", factory.GetTargetType())

			deployment.radixDeployment = buildRd(appName, environment, componentName, []v1.RadixVolumeMount{})
			deployComponent := deployment.radixDeployment.Spec.Components[0]

			volumes, err := deployment.GetVolumesForComponent(&deployComponent)

			assert.Nil(t, err)
			assert.Len(t, volumes, 0)
		}
	})
	suite.T().Run("Exists volume", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		deployment := getDeployment(testEnv)
		for _, factory := range suite.radixCommonDeployComponentFactories {
			for _, scenario := range scenarios {
				t.Logf("Test case %s for component %s", scenario.name, factory.GetTargetType())

				deployment.radixDeployment = buildRd(appName, environment, componentName, []v1.RadixVolumeMount{scenario.radixVolumeMount})
				deployComponent := deployment.radixDeployment.Spec.Components[0]

				volumes, err := deployment.GetVolumesForComponent(&deployComponent)

				assert.Nil(t, err)
				assert.Len(t, volumes, 1)
				assert.Equal(t, scenario.expectedVolumeName, volumes[0].Name)
				assert.NotNil(t, volumes[0].PersistentVolumeClaim)
				assert.Contains(t, volumes[0].PersistentVolumeClaim.ClaimName, scenario.expectedPvcNamePrefix)
			}
		}
	})
}

type expectedPvcScProperties struct {
	appName                 string
	environment             string
	componentName           string
	radixVolumeMountName    string
	radixStorageName        string
	pvcName                 string
	storageClassName        string
	radixVolumeMountType    v1.MountType
	requestsVolumeMountSize string
	volumeAccessMode        corev1.PersistentVolumeAccessMode
	volumeName              string
	scProvisioner           string
	scSecretName            string
	scTmpPath               string
	scGid                   string
	scUid                   string
	namespace               string
}

func (suite *VolumeMountTestSuite) Test_GetRadixDeployComponentVolumeMounts() {
	appName := "any-app"
	environment := "some-env"
	componentName := "some-component"
	scenarios := []volumeMountTestScenario{
		{
			name:                  "Blob CSI Azure volume, Status phase: Bound",
			radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Name: "blob-volume1", Storage: "storage1", Path: "path1", GID: "1000"},
			expectedVolumeName:    "csi-az-blob-some-component-blob-volume1-storage1",
			expectedPvcNamePrefix: "pvc-csi-az-blob-some-component-blob-volume1-storage1",
		},
		{
			name:                  "Blob CSI Azure volume, Status phase: Pending",
			radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeBlobCsiAzure, Name: "blob-volume2", Storage: "storage2", Path: "path2", GID: "1000"},
			expectedVolumeName:    "csi-az-blob-some-component-blob-volume2-storage2",
			expectedPvcNamePrefix: "pvc-csi-az-blob-some-component-blob-volume2-storage2",
		},
		{
			name:                  "File CSI Azure volume, Status phase: Bound",
			radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeFileCsiAzure, Name: "file-volume1", Storage: "storage3", Path: "path3", GID: "1000"},
			expectedVolumeName:    "csi-az-file-some-component-file-volume1-storage3",
			expectedPvcNamePrefix: "pvc-csi-az-file-some-component-file-volume1-storage3",
		},
		{
			name:                  "File CSI Azure volume, Status phase: Pending",
			radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeFileCsiAzure, Name: "file-volume2", Storage: "storage4", Path: "path4", GID: "1000"},
			expectedVolumeName:    "csi-az-file-some-component-file-volume2-storage4",
			expectedPvcNamePrefix: "pvc-csi-az-file-some-component-file-volume2-storage4",
		},
	}

	suite.T().Run("No volumes", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		deployment := getDeployment(testEnv)
		for _, factory := range suite.radixCommonDeployComponentFactories {
			t.Logf("Test case for component %s", factory.GetTargetType())

			deployment.radixDeployment = buildRd(appName, environment, componentName, []v1.RadixVolumeMount{})
			deployComponent := deployment.radixDeployment.Spec.Components[0]

			volumes, err := GetRadixDeployComponentVolumeMounts(&deployComponent, "")

			assert.Nil(t, err)
			assert.Len(t, volumes, 0)
		}
	})
	suite.T().Run("Exists volume", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		deployment := getDeployment(testEnv)
		for _, factory := range suite.radixCommonDeployComponentFactories {
			for _, scenario := range scenarios {
				t.Logf("Test case %s for component %s", scenario.name, factory.GetTargetType())

				deployment.radixDeployment = buildRd(appName, environment, componentName, []v1.RadixVolumeMount{scenario.radixVolumeMount})
				deployComponent := deployment.radixDeployment.Spec.Components[0]

				volumes, err := GetRadixDeployComponentVolumeMounts(&deployComponent, "")

				assert.Nil(t, err)
				assert.Len(t, volumes, 1)
				assert.Equal(t, scenario.expectedVolumeName, volumes[0].Name)
				assert.Equal(t, scenario.expectedVolumeName, volumes[0].Name)
				assert.Equal(t, scenario.radixVolumeMount.Path, volumes[0].MountPath)
			}
		}
	})
}

func (suite *VolumeMountTestSuite) Test_CreateOrUpdateCsiAzureBlobVolumeResources() {
	scenarios := []volumeMountTestScenario{
		{
			radixVolumeMount:   v1.RadixVolumeMount{Type: v1.MountTypeFileCsiAzure, Name: "volume1", Storage: "storageName1", Path: "TestPath1"},
			expectedVolumeName: "csi-az-file-app-volume1-storageName1",
		},
		{
			radixVolumeMount:   v1.RadixVolumeMount{Type: v1.MountTypeFileCsiAzure, Name: "volume2", Storage: "storageName2", Path: "TestPath2"},
			expectedVolumeName: "csi-az-file-app-volume2-storageName2",
		},
	}
	suite.T().Run("One File CSI Azure volume mount ", func(t *testing.T) {
		expectedVolumeCount := map[v1.RadixComponentType]int{
			v1.RadixComponentTypeComponent:    1,
			v1.RadixComponentTypeJobScheduler: 0,
		}
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			t.Logf("Test case %s for component %s", scenarios[0].name, factory.GetTargetType())
			component := utils.NewDeployCommonComponentBuilder(factory).
				WithName("app").
				WithVolumeMounts([]v1.RadixVolumeMount{scenarios[0].radixVolumeMount}).
				BuildComponent()

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(component, "")
			assert.Nil(t, err)
			assert.Equal(t, expectedVolumeCount[component.GetType()], len(volumeMounts))
			if len(volumeMounts) > 0 {
				mount := volumeMounts[0]
				assert.Equal(t, scenarios[0].expectedVolumeName, mount.Name)
				assert.Equal(t, scenarios[0].radixVolumeMount.Path, mount.MountPath)
			}
		}
	})
	suite.T().Run("Multiple File CSI Azure volume mount", func(t *testing.T) {
		expectedVolumeCount := map[v1.RadixComponentType]int{
			v1.RadixComponentTypeComponent:    2,
			v1.RadixComponentTypeJobScheduler: 0,
		}
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			component := utils.NewDeployCommonComponentBuilder(factory).
				WithName("app").
				WithVolumeMounts([]v1.RadixVolumeMount{scenarios[0].radixVolumeMount, scenarios[1].radixVolumeMount}).
				BuildComponent()

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(component, "")
			assert.Nil(t, err)
			for idx, testCase := range scenarios {
				assert.Equal(t, expectedVolumeCount[component.GetType()], len(volumeMounts))
				if len(volumeMounts) > 0 {
					assert.Equal(t, testCase.expectedVolumeName, volumeMounts[idx].Name)
					assert.Equal(t, testCase.radixVolumeMount.Path, volumeMounts[idx].MountPath)
				}
			}
		}
	})
}

func (suite *VolumeMountTestSuite) Test_CreateOrUpdateCsiAzureResources() {
	appName := "any-app"
	environment := "some-env"
	componentName := "some-component"

	var scenarios []deploymentVolumesTestScenario
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcScProperties) deploymentVolumesTestScenario {
			return deploymentVolumesTestScenario{
				name:  "Create new volume",
				props: props,
				radixVolumeMounts: []v1.RadixVolumeMount{
					createRadixVolumeMount(props, func(vm *v1.RadixVolumeMount) {}),
				},
				volumes: []corev1.Volume{
					createVolume(props, func(v *corev1.Volume) {}),
				},
				existingPvcsBeforeTestRun: []corev1.PersistentVolumeClaim{},
				existingPvcsAfterTestRun: []corev1.PersistentVolumeClaim{
					createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
				},
				existingStorageClassesBeforeTestRun: []storagev1.StorageClass{},
				existingStorageClassesAfterTestRun: []storagev1.StorageClass{
					createExpectedStorageClass(props, func(sc *storagev1.StorageClass) {}),
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobVolume1Storage1(nil)),
			getScenario(getPropsCsiFileVolume2Storage2(nil)),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		type scenarioProperties struct {
			changedNewRadixVolumeName        string
			changedNewRadixVolumeStorageName string
			expectedVolumeName               string
			expectedNewSecretName            string
			expectedNewPvcName               string
			expectedNewStorageClassName      string
			expectedNewScTmpPath             string
		}
		getScenario := func(props expectedPvcScProperties, scenarioProps scenarioProperties) deploymentVolumesTestScenario {
			return deploymentVolumesTestScenario{
				name:  "Update storage in existing volume name and storage",
				props: props,
				radixVolumeMounts: []v1.RadixVolumeMount{
					createRadixVolumeMount(props, func(vm *v1.RadixVolumeMount) {
						vm.Name = scenarioProps.changedNewRadixVolumeName
						vm.Storage = scenarioProps.changedNewRadixVolumeStorageName
					}),
				},
				volumes: []corev1.Volume{
					createVolume(props, func(v *corev1.Volume) {
						v.Name = scenarioProps.expectedVolumeName
					}),
				},
				existingPvcsBeforeTestRun: []corev1.PersistentVolumeClaim{
					createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
				},
				existingPvcsAfterTestRun: []corev1.PersistentVolumeClaim{
					createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {
						pvc.ObjectMeta.Name = scenarioProps.expectedNewPvcName
						pvc.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = scenarioProps.changedNewRadixVolumeName
						pvc.Spec.StorageClassName = utils.StringPtr(scenarioProps.expectedNewStorageClassName)
					}),
				},
				existingStorageClassesBeforeTestRun: []storagev1.StorageClass{
					createExpectedStorageClass(props, func(sc *storagev1.StorageClass) {}),
				},
				existingStorageClassesAfterTestRun: []storagev1.StorageClass{
					createExpectedStorageClass(props, func(sc *storagev1.StorageClass) {
						sc.ObjectMeta.Name = scenarioProps.expectedNewStorageClassName
						sc.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = scenarioProps.changedNewRadixVolumeName
						setStorageClassMountOption(sc, "--tmp-path", scenarioProps.expectedNewScTmpPath)
						setStorageClassStorageParameter(props.radixVolumeMountType, scenarioProps.changedNewRadixVolumeStorageName, sc)
						sc.Parameters[csiStorageClassProvisionerSecretNameParameter] = scenarioProps.expectedNewSecretName
						sc.Parameters[csiStorageClassNodeStageSecretNameParameter] = scenarioProps.expectedNewSecretName
					}),
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobVolume1Storage1(nil), scenarioProperties{
				changedNewRadixVolumeName:        "volume101",
				changedNewRadixVolumeStorageName: "storage101",
				expectedVolumeName:               "csi-az-blob-some-component-volume101-storage101",
				expectedNewSecretName:            "some-component-volume101-csiazurecreds",
				expectedNewPvcName:               "pvc-csi-az-blob-some-component-volume101-storage101-12345",
				expectedNewStorageClassName:      "sc-any-app-some-env-csi-az-blob-some-component-volume101-storage101",
				expectedNewScTmpPath:             "/tmp/any-app-some-env/csi-az-blob/some-component/volume101/storage101",
			}),
			getScenario(getPropsCsiFileVolume2Storage2(nil), scenarioProperties{
				changedNewRadixVolumeName:        "volume101",
				changedNewRadixVolumeStorageName: "storage101",
				expectedVolumeName:               "csi-az-file-some-component-volume101-storage101",
				expectedNewSecretName:            "some-component-volume101-csiazurecreds",
				expectedNewPvcName:               "pvc-csi-az-file-some-component-volume101-storage101-12345",
				expectedNewStorageClassName:      "sc-any-app-some-env-csi-az-file-some-component-volume101-storage101",
				expectedNewScTmpPath:             "/tmp/any-app-some-env/csi-az-file/some-component/volume101/storage101",
			}),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcScProperties) deploymentVolumesTestScenario {
			storageClassForAnotherNamespace := createRandomStorageClass(props, utils.RandString(10), utils.RandString(10))
			storageClassForAnotherComponent := createRandomStorageClass(props, props.namespace, utils.RandString(10))
			pvcForAnotherNamespace := createRandomPvc(props, utils.RandString(10), utils.RandString(10))
			pvcForAnotherComponent := createRandomPvc(props, props.namespace, utils.RandString(10))
			return deploymentVolumesTestScenario{
				name:  "Garbage collect orphaned PVCs and StorageClasses",
				props: props,
				radixVolumeMounts: []v1.RadixVolumeMount{
					createRadixVolumeMount(props, func(vm *v1.RadixVolumeMount) {}),
				},
				volumes: []corev1.Volume{
					createVolume(props, func(v *corev1.Volume) {}),
				},
				existingPvcsBeforeTestRun: []corev1.PersistentVolumeClaim{
					createRandomPvc(props, props.namespace, props.componentName),
					pvcForAnotherNamespace,
					pvcForAnotherComponent,
				},
				existingPvcsAfterTestRun: []corev1.PersistentVolumeClaim{
					createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
					pvcForAnotherNamespace,
					pvcForAnotherComponent,
				},
				existingStorageClassesBeforeTestRun: []storagev1.StorageClass{
					createRandomStorageClass(props, props.namespace, props.componentName),
					storageClassForAnotherNamespace,
					storageClassForAnotherComponent,
				},
				existingStorageClassesAfterTestRun: []storagev1.StorageClass{
					createExpectedStorageClass(props, func(sc *storagev1.StorageClass) {}),
					storageClassForAnotherNamespace,
					storageClassForAnotherComponent,
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobVolume1Storage1(nil)),
			getScenario(getPropsCsiFileVolume2Storage2(nil)),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcScProperties) deploymentVolumesTestScenario {
			return deploymentVolumesTestScenario{
				name:  "Set readonly volume",
				props: props,
				radixVolumeMounts: []v1.RadixVolumeMount{
					createRadixVolumeMount(props, func(vm *v1.RadixVolumeMount) { vm.AccessMode = string(corev1.ReadOnlyMany) }),
				},
				volumes: []corev1.Volume{
					createVolume(props, func(v *corev1.Volume) {}),
				},
				existingPvcsBeforeTestRun: []corev1.PersistentVolumeClaim{
					createRandomPvc(props, props.namespace, props.componentName),
				},
				existingPvcsAfterTestRun: []corev1.PersistentVolumeClaim{
					createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {
						pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
					}),
				},
				existingStorageClassesBeforeTestRun: []storagev1.StorageClass{
					createRandomStorageClass(props, props.namespace, props.componentName),
				},
				existingStorageClassesAfterTestRun: []storagev1.StorageClass{
					createExpectedStorageClass(props, func(sc *storagev1.StorageClass) {
						sc.MountOptions = append(sc.MountOptions, "-o ro")
					}),
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobVolume1Storage1(nil)),
			getScenario(getPropsCsiFileVolume2Storage2(nil)),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcScProperties) deploymentVolumesTestScenario {
			return deploymentVolumesTestScenario{
				name:  "Set ReadWriteOnce volume",
				props: props,
				radixVolumeMounts: []v1.RadixVolumeMount{
					createRadixVolumeMount(props, func(vm *v1.RadixVolumeMount) { vm.AccessMode = string(corev1.ReadWriteOnce) }),
				},
				volumes: []corev1.Volume{
					createVolume(props, func(v *corev1.Volume) {}),
				},
				existingPvcsBeforeTestRun: []corev1.PersistentVolumeClaim{
					createRandomPvc(props, props.namespace, props.componentName),
				},
				existingPvcsAfterTestRun: []corev1.PersistentVolumeClaim{
					createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {
						pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
					}),
				},
				existingStorageClassesBeforeTestRun: []storagev1.StorageClass{
					createRandomStorageClass(props, props.namespace, props.componentName),
				},
				existingStorageClassesAfterTestRun: []storagev1.StorageClass{
					createExpectedStorageClass(props, func(sc *storagev1.StorageClass) {}),
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobVolume1Storage1(nil)),
			getScenario(getPropsCsiFileVolume2Storage2(nil)),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcScProperties) deploymentVolumesTestScenario {
			return deploymentVolumesTestScenario{
				name:  "Set ReadWriteOnce volume",
				props: props,
				radixVolumeMounts: []v1.RadixVolumeMount{
					createRadixVolumeMount(props, func(vm *v1.RadixVolumeMount) { vm.AccessMode = string(corev1.ReadWriteMany) }),
				},
				volumes: []corev1.Volume{
					createVolume(props, func(v *corev1.Volume) {}),
				},
				existingPvcsBeforeTestRun: []corev1.PersistentVolumeClaim{
					createRandomPvc(props, props.namespace, props.componentName),
				},
				existingPvcsAfterTestRun: []corev1.PersistentVolumeClaim{
					createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {
						pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
					}),
				},
				existingStorageClassesBeforeTestRun: []storagev1.StorageClass{
					createRandomStorageClass(props, props.namespace, props.componentName),
				},
				existingStorageClassesAfterTestRun: []storagev1.StorageClass{
					createExpectedStorageClass(props, func(sc *storagev1.StorageClass) {}),
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobVolume1Storage1(nil)),
			getScenario(getPropsCsiFileVolume2Storage2(nil)),
		}
	}()...)

	suite.T().Run("CSI Azure volume PVCs and StorageClasses", func(t *testing.T) {
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			for _, scenario := range scenarios {
				t.Logf("Test case %s, volume type %s, component %s", scenario.name, scenario.props.radixVolumeMountType, factory.GetTargetType())
				testEnv := getTestEnv()
				deployment := getDeployment(testEnv)
				deployment.radixDeployment = buildRd(appName, environment, componentName, scenario.radixVolumeMounts)
				putExistingDeploymentVolumesScenarioDataToFakeCluster(&scenario, deployment)
				desiredDeployment := getDesiredDeployment(componentName, scenario.volumes)

				//action
				err := deployment.createOrUpdateCsiAzureVolumeResources(desiredDeployment)
				assert.Nil(t, err)

				existingPvcs, existingScs, err := getExistingPvcsAndStorageClassesFromFakeCluster(deployment)
				assert.Nil(t, err)
				equalPvcLists, err := utils.EqualPvcLists(&scenario.existingPvcsAfterTestRun, &existingPvcs, true)
				assert.Nil(t, err)
				assert.True(t, equalPvcLists)
				equalStorageClassLists, err := utils.EqualStorageClassLists(&scenario.existingStorageClassesAfterTestRun, &existingScs)
				assert.Nil(t, err)
				assert.True(t, equalStorageClassLists)
			}
		}
	})
}

func (suite *VolumeMountTestSuite) Test_CreateOrUpdateCsiAzureKeyVaultResources() {
	namespace := "some-namespace"
	environment := "some-env"
	componentName1 := "component1"
	type expectedVolumeProps struct {
		expectedVolumeNamePrefix         string
		expectedVolumeMountPath          string
		expectedNodePublishSecretRefName string
		expectedVolumeAttributePrefixes  map[string]string
	}
	scenarios := []struct {
		name                    string
		deployComponentBuilders []utils.DeployCommonComponentBuilder
		componentName           string
		azureKeyVaults          []v1.RadixAzureKeyVault
		expectedVolumeProps     []expectedVolumeProps
		radixVolumeMounts       []v1.RadixVolumeMount
	}{
		{
			name:                "No Azure Key volumes as no RadixAzureKeyVault-s",
			componentName:       componentName1,
			azureKeyVaults:      []v1.RadixAzureKeyVault{},
			expectedVolumeProps: []expectedVolumeProps{},
		},
		{
			name:           "No Azure Key volumes as no secret names in secret object",
			componentName:  componentName1,
			azureKeyVaults: []v1.RadixAzureKeyVault{{Name: "kv1"}},
		},
		{
			name:          "One Azure Key volume for one secret objects secret name",
			componentName: componentName1,
			azureKeyVaults: []v1.RadixAzureKeyVault{{
				Name:  "kv1",
				Items: []v1.RadixAzureKeyVaultItem{{Name: "secret1", EnvVar: "SECRET_REF1"}},
			}},
			expectedVolumeProps: []expectedVolumeProps{
				{
					expectedVolumeNamePrefix:         "component1-az-keyvault-opaque-kv1-",
					expectedVolumeMountPath:          "/mnt/azure-key-vault/kv1",
					expectedNodePublishSecretRefName: "component1-kv1-csiazkvcreds",
					expectedVolumeAttributePrefixes: map[string]string{
						"secretProviderClass": "component1-az-keyvault-kv1-",
					},
				},
			},
		},
		{
			name:          "Multiple Azure Key volumes for each RadixAzureKeyVault",
			componentName: componentName1,
			azureKeyVaults: []v1.RadixAzureKeyVault{
				{
					Name:  "kv1",
					Path:  utils.StringPtr("/mnt/customPath"),
					Items: []v1.RadixAzureKeyVaultItem{{Name: "secret1", EnvVar: "SECRET_REF1"}},
				},
				{
					Name:  "kv2",
					Items: []v1.RadixAzureKeyVaultItem{{Name: "secret2", EnvVar: "SECRET_REF2"}},
				},
			},
			expectedVolumeProps: []expectedVolumeProps{
				{
					expectedVolumeNamePrefix:         "component1-az-keyvault-opaque-kv1-",
					expectedVolumeMountPath:          "/mnt/customPath",
					expectedNodePublishSecretRefName: "component1-kv1-csiazkvcreds",
					expectedVolumeAttributePrefixes: map[string]string{
						"secretProviderClass": "component1-az-keyvault-kv1-",
					},
				},
				{
					expectedVolumeNamePrefix:         "component1-az-keyvault-opaque-kv2-",
					expectedVolumeMountPath:          "/mnt/azure-key-vault/kv2",
					expectedNodePublishSecretRefName: "component1-kv2-csiazkvcreds",
					expectedVolumeAttributePrefixes: map[string]string{
						"secretProviderClass": "component1-az-keyvault-kv2-",
					},
				},
			},
		},
	}
	suite.T().Run("CSI Azure Key vault volumes", func(t *testing.T) {
		t.Parallel()
		for _, scenario := range scenarios {
			testEnv := getTestEnv()
			deployment := getDeployment(testEnv)
			deployment.radixDeployment = buildRdWithComponentBuilders(appName, environment, func() []utils.DeployComponentBuilder {
				var builders []utils.DeployComponentBuilder
				builders = append(builders, utils.NewDeployComponentBuilder().
					WithName(scenario.componentName).
					WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: scenario.azureKeyVaults}))
				return builders
			})
			radixDeployComponent := deployment.radixDeployment.GetComponentByName(scenario.componentName)
			for _, azureKeyVault := range scenario.azureKeyVaults {
				spc, err := deployment.createAzureKeyVaultSecretProviderClassForRadixDeployment(namespace, appName, radixDeployComponent.GetName(), azureKeyVault)
				if err != nil {
					t.Logf(err.Error())
				} else {
					t.Logf("created secret provider class %s", spc.Name)
				}
			}
			volumes, err := GetVolumes(testEnv.kubeclient, testEnv.kubeUtil, namespace, environment, radixDeployComponent, deployment.radixDeployment.GetName())
			assert.Nil(t, err)
			assert.Len(t, volumes, len(scenario.expectedVolumeProps))
			if len(scenario.expectedVolumeProps) == 0 {
				continue
			}

			for i := 0; i < len(volumes); i++ {
				volume := volumes[i]
				assert.NotNil(t, volume.CSI)
				assert.NotNil(t, volume.CSI.VolumeAttributes)
				assert.NotNil(t, volume.CSI.NodePublishSecretRef)
				assert.Equal(t, "secrets-store.csi.k8s.io", volume.CSI.Driver)

				volumeProp := scenario.expectedVolumeProps[i]
				for attrKey, attrValue := range volumeProp.expectedVolumeAttributePrefixes {
					spcValue, exists := volume.CSI.VolumeAttributes[attrKey]
					assert.True(t, exists)
					assert.True(t, strings.HasPrefix(spcValue, attrValue))
				}
				assert.True(t, strings.Contains(volume.Name, volumeProp.expectedVolumeNamePrefix))
				assert.Equal(t, volumeProp.expectedNodePublishSecretRefName, volume.CSI.NodePublishSecretRef.Name)
			}
		}
	})

	suite.T().Run("CSI Azure Key vault volume mounts", func(t *testing.T) {
		t.Parallel()
		for _, scenario := range scenarios {
			testEnv := getTestEnv()
			deployment := getDeployment(testEnv)
			deployment.radixDeployment = buildRdWithComponentBuilders(appName, environment, func() []utils.DeployComponentBuilder {
				var builders []utils.DeployComponentBuilder
				builders = append(builders, utils.NewDeployComponentBuilder().
					WithName(scenario.componentName).
					WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: scenario.azureKeyVaults}))
				return builders
			})
			radixDeployComponent := deployment.radixDeployment.GetComponentByName(scenario.componentName)
			for _, azureKeyVault := range scenario.azureKeyVaults {
				spc, err := deployment.createAzureKeyVaultSecretProviderClassForRadixDeployment(namespace, appName, radixDeployComponent.GetName(), azureKeyVault)
				if err != nil {
					t.Logf(err.Error())
				} else {
					t.Logf("created secret provider class %s", spc.Name)
				}
			}
			volumeMounts, err := GetRadixDeployComponentVolumeMounts(radixDeployComponent, deployment.radixDeployment.GetName())
			assert.Nil(t, err)
			assert.Len(t, volumeMounts, len(scenario.expectedVolumeProps))
			if len(scenario.expectedVolumeProps) == 0 {
				continue
			}

			for i := 0; i < len(volumeMounts); i++ {
				volumeMount := volumeMounts[i]
				volumeProp := scenario.expectedVolumeProps[i]
				assert.True(t, strings.Contains(volumeMount.Name, volumeProp.expectedVolumeNamePrefix))
				assert.Equal(t, volumeProp.expectedVolumeMountPath, volumeMount.MountPath)
				assert.True(t, volumeMount.ReadOnly)
			}
		}
	})
}

func createRandomStorageClass(props expectedPvcScProperties, namespace, componentName string) storagev1.StorageClass {
	return createExpectedStorageClass(props, func(sc *storagev1.StorageClass) {
		sc.ObjectMeta.Name = utils.RandString(10)
		sc.ObjectMeta.Labels[kube.RadixNamespace] = namespace
		sc.ObjectMeta.Labels[kube.RadixComponentLabel] = componentName
	})
}

func createRandomPvc(props expectedPvcScProperties, namespace, componentName string) corev1.PersistentVolumeClaim {
	return createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {
		pvc.ObjectMeta.Name = utils.RandString(10)
		pvc.ObjectMeta.Namespace = namespace
		pvc.ObjectMeta.Labels[kube.RadixComponentLabel] = componentName
		pvc.Spec.StorageClassName = utils.StringPtr(utils.RandString(10))
	})
}

func setStorageClassMountOption(sc *storagev1.StorageClass, key, value string) {
	mountOptions := sc.MountOptions
	for i, option := range mountOptions {
		if strings.Contains(option, key) {
			mountOptions[i] = fmt.Sprintf("%s=%s", key, value)
			return
		}
	}
	fmt.Printf("MountOption %s not found for the storage class", key)
}

func getPropsCsiBlobVolume1Storage1(modify func(*expectedPvcScProperties)) expectedPvcScProperties {
	appName := "any-app"
	environment := "some-env"
	componentName := "some-component"
	props := expectedPvcScProperties{
		appName:                 appName,
		environment:             environment,
		namespace:               fmt.Sprintf("%s-%s", appName, environment),
		componentName:           componentName,
		radixVolumeMountName:    "volume1",
		radixStorageName:        "storage1",
		pvcName:                 "pvc-csi-az-blob-some-component-volume1-storage1-12345",
		storageClassName:        "sc-any-app-some-env-csi-az-blob-some-component-volume1-storage1",
		radixVolumeMountType:    v1.MountTypeBlobCsiAzure,
		requestsVolumeMountSize: "1Mi",
		volumeAccessMode:        corev1.ReadWriteMany, //default access mode
		volumeName:              "csi-az-blob-some-component-volume1-storage1",
		scProvisioner:           v1.ProvisionerBlobCsiAzure,
		scSecretName:            "some-component-volume1-csiazurecreds",
		scTmpPath:               "/tmp/any-app-some-env/csi-az-blob/some-component/volume1/storage1",
		scGid:                   "1000",
		scUid:                   "",
	}
	if modify != nil {
		modify(&props)
	}
	return props
}

func getPropsCsiFileVolume2Storage2(modify func(*expectedPvcScProperties)) expectedPvcScProperties {
	appName := "any-app"
	environment := "some-env"
	componentName := "some-component"
	props := expectedPvcScProperties{
		appName:                 appName,
		environment:             environment,
		namespace:               fmt.Sprintf("%s-%s", appName, environment),
		componentName:           componentName,
		radixVolumeMountName:    "volume2",
		radixStorageName:        "storage2",
		pvcName:                 "pvc-csi-az-file-some-component-volume2-storage2-12345",
		storageClassName:        "sc-any-app-some-env-csi-az-file-some-component-volume2-storage2",
		radixVolumeMountType:    v1.MountTypeFileCsiAzure,
		requestsVolumeMountSize: "1Mi",
		volumeAccessMode:        corev1.ReadWriteMany, //default access mode
		volumeName:              "csi-az-file-some-component-volume2-storage2",
		scProvisioner:           v1.ProvisionerFileCsiAzure,
		scSecretName:            "some-component-volume2-csiazurecreds",
		scTmpPath:               "/tmp/any-app-some-env/csi-az-file/some-component/volume2/storage2",
		scGid:                   "1000",
		scUid:                   "",
	}
	if modify != nil {
		modify(&props)
	}
	return props
}

func putExistingDeploymentVolumesScenarioDataToFakeCluster(scenario *deploymentVolumesTestScenario, deployment *Deployment) {
	for _, pvc := range scenario.existingPvcsBeforeTestRun {
		_, _ = deployment.kubeclient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(context.TODO(), &pvc, metav1.CreateOptions{})
	}
	for _, sc := range scenario.existingStorageClassesBeforeTestRun {
		_, _ = deployment.kubeclient.StorageV1().StorageClasses().Create(context.TODO(), &sc, metav1.CreateOptions{})
	}
}

func getExistingPvcsAndStorageClassesFromFakeCluster(deployment *Deployment) ([]corev1.PersistentVolumeClaim, []storagev1.StorageClass, error) {
	var pvcItems []corev1.PersistentVolumeClaim
	var scItems []storagev1.StorageClass
	pvcList, err := deployment.kubeclient.CoreV1().PersistentVolumeClaims("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return pvcItems, scItems, err
	}
	if pvcList != nil && pvcList.Items != nil {
		pvcItems = pvcList.Items
	}
	storageClassList, err := deployment.kubeclient.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return pvcItems, scItems, err
	}
	if storageClassList != nil && storageClassList.Items != nil {
		scItems = storageClassList.Items
	}
	return pvcItems, scItems, nil
}

func getDesiredDeployment(componentName string, volumes []corev1.Volume) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Labels: map[string]string{
				kube.RadixComponentLabel: componentName,
			},
			Annotations: make(map[string]string),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(DefaultReplicas),
			Selector: &metav1.LabelSelector{MatchLabels: make(map[string]string)},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: make(map[string]string), Annotations: make(map[string]string)},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: componentName}},
					Volumes:    volumes,
				},
			},
		},
	}
}

func buildRd(appName string, environment string, componentName string, radixVolumeMounts []v1.RadixVolumeMount) *v1.RadixDeployment {
	return utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(environment).
		WithComponents(utils.NewDeployComponentBuilder().
			WithName(componentName).
			WithVolumeMounts(radixVolumeMounts)).
		BuildRD()
}

func createPvc(namespace, componentName string, mountType v1.MountType, modify func(*corev1.PersistentVolumeClaim)) corev1.PersistentVolumeClaim {
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.RandString(10), //Set in test scenario
			Namespace: namespace,
			Labels: map[string]string{
				kube.RadixAppLabel:             appName,
				kube.RadixComponentLabel:       componentName,
				kube.RadixMountTypeLabel:       string(mountType),
				kube.RadixVolumeMountNameLabel: utils.RandString(10), //Set in test scenario
			},
		},
	}
	if modify != nil {
		modify(&pvc)
	}
	return pvc
}

func buildRdWithComponentBuilders(appName string, environment string, componentBuilders func() []utils.DeployComponentBuilder) *v1.RadixDeployment {
	return utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(environment).
		WithComponents(componentBuilders()...).
		BuildRD()
}

func createExpectedStorageClass(props expectedPvcScProperties, modify func(class *storagev1.StorageClass)) storagev1.StorageClass {
	mountOptions := []string{
		"--file-cache-timeout-in-seconds=120",
		"--use-attr-cache=true",
		"-o allow_other",
		"-o attr_timeout=120",
		"-o entry_timeout=120",
		"-o negative_timeout=120",
		fmt.Sprintf("--tmp-path=%s", props.scTmpPath),
	}
	idOption := getStorageClassIdMountOption(props)
	if len(idOption) > 0 {
		mountOptions = append(mountOptions, idOption)
	}
	reclaimPolicy := corev1.PersistentVolumeReclaimRetain
	bindingMode := storagev1.VolumeBindingImmediate
	sc := storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: props.storageClassName,
			Labels: map[string]string{
				kube.RadixAppLabel:             props.appName,
				kube.RadixNamespace:            props.namespace,
				kube.RadixComponentLabel:       props.componentName,
				kube.RadixMountTypeLabel:       string(props.radixVolumeMountType),
				kube.RadixVolumeMountNameLabel: props.radixVolumeMountName,
			},
		},
		Provisioner: props.scProvisioner,
		Parameters: map[string]string{
			csiStorageClassProvisionerSecretNameParameter:      props.scSecretName,
			csiStorageClassProvisionerSecretNamespaceParameter: props.namespace,
			csiStorageClassNodeStageSecretNameParameter:        props.scSecretName,
			csiStorageClassNodeStageSecretNamespaceParameter:   props.namespace,
		},
		MountOptions:      mountOptions,
		ReclaimPolicy:     &reclaimPolicy,
		VolumeBindingMode: &bindingMode,
	}
	setStorageClassStorageParameter(props.radixVolumeMountType, props.radixStorageName, &sc)
	if modify != nil {
		modify(&sc)
	}
	return sc
}

func setStorageClassStorageParameter(radixVolumeMountType v1.MountType, storageName string, sc *storagev1.StorageClass) {
	switch radixVolumeMountType {
	case v1.MountTypeBlobCsiAzure:
		sc.Parameters[csiStorageClassContainerNameParameter] = storageName
	case v1.MountTypeFileCsiAzure:
		sc.Parameters[csiStorageClassShareNameParameter] = storageName
	}
}

func getStorageClassIdMountOption(props expectedPvcScProperties) string {
	if len(props.scGid) > 0 {
		return fmt.Sprintf("-o gid=%s", props.scGid)
	}
	if len(props.scUid) > 0 {
		return fmt.Sprintf("-o uid=%s", props.scGid)
	}
	return ""
}

func createExpectedPvc(props expectedPvcScProperties, modify func(*corev1.PersistentVolumeClaim)) corev1.PersistentVolumeClaim {
	labels := map[string]string{
		kube.RadixAppLabel:             props.appName,
		kube.RadixComponentLabel:       props.componentName,
		kube.RadixMountTypeLabel:       string(props.radixVolumeMountType),
		kube.RadixVolumeMountNameLabel: props.radixVolumeMountName,
	}
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      props.pvcName,
			Namespace: props.namespace,
			Labels:    labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{props.volumeAccessMode},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(props.requestsVolumeMountSize)}, //it seems correct number is not needed for CSI driver
			},
			StorageClassName: utils.StringPtr(props.storageClassName),
		},
	}
	if modify != nil {
		modify(&pvc)
	}
	return pvc
}

func createVolume(pvcProps expectedPvcScProperties, modify func(*corev1.Volume)) corev1.Volume {
	volume := corev1.Volume{
		Name: pvcProps.volumeName,
		VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: pvcProps.pvcName,
		}},
	}
	if modify != nil {
		modify(&volume)
	}
	return volume
}

func createRadixVolumeMount(props expectedPvcScProperties, modify func(mount *v1.RadixVolumeMount)) v1.RadixVolumeMount {
	volumeMount := v1.RadixVolumeMount{
		Type:    props.radixVolumeMountType,
		Name:    props.radixVolumeMountName,
		Storage: props.radixStorageName,
		Path:    "path1",
		GID:     "1000",
	}
	if modify != nil {
		modify(&volumeMount)
	}
	return volumeMount
}
