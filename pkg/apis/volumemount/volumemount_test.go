package volumemount

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/internal"
	"github.com/equinor/radix-operator/pkg/apis/internal/persistentvolume"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/google/uuid"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type VolumeMountTestSuite struct {
	suite.Suite
	radixCommonDeployComponentFactories []radixv1.RadixCommonDeployComponentFactory
}

type TestEnv struct {
	kubeclient           kubernetes.Interface
	radixclient          radixclient.Interface
	secretproviderclient secretProviderClient.Interface
	prometheusclient     prometheusclient.Interface
	kubeUtil             *kube.Kube
	kedaClient           kedav2.Interface
}

type volumeMountTestScenario struct {
	name                       string
	radixVolumeMount           radixv1.RadixVolumeMount
	expectedVolumeName         string
	expectedVolumeNameIsPrefix bool
	expectedError              string
	expectedPvcNamePrefix      string
}

type deploymentVolumesTestScenario struct {
	name              string
	props             expectedPvcPvProperties
	radixVolumeMounts []radixv1.RadixVolumeMount
	volumes           []corev1.Volume
	existingPvs       []corev1.PersistentVolume
	existingPvcs      []corev1.PersistentVolumeClaim
	expectedPvs       []corev1.PersistentVolume
	expectedPvcs      []corev1.PersistentVolumeClaim
}

type pvcTestScenario struct {
	volumeMountTestScenario
	pv  corev1.PersistentVolume
	pvc corev1.PersistentVolumeClaim
}

const (
	appName       = "any-app"
	environment   = "some-env"
	componentName = "some-component"
)

var (
	anotherNamespace       = strings.ToLower(commonUtils.RandString(10))
	anotherComponentName   = strings.ToLower(commonUtils.RandString(10))
	anotherVolumeMountName = strings.ToLower(commonUtils.RandString(10))
)

func TestVolumeMountTestSuite(t *testing.T) {
	suite.Run(t, new(VolumeMountTestSuite))
}

func (suite *VolumeMountTestSuite) SetupSuite() {
	suite.radixCommonDeployComponentFactories = []radixv1.RadixCommonDeployComponentFactory{
		radixv1.RadixDeployComponentFactory{},
		radixv1.RadixDeployJobComponentFactory{},
	}
}

func getTestEnv() TestEnv {
	testEnv := TestEnv{
		kubeclient:           kubefake.NewSimpleClientset(),
		radixclient:          radixfake.NewSimpleClientset(),
		kedaClient:           kedafake.NewSimpleClientset(),
		secretproviderclient: secretproviderfake.NewSimpleClientset(),
		prometheusclient:     prometheusfake.NewSimpleClientset(),
	}
	kubeUtil, _ := kube.New(testEnv.kubeclient, testEnv.radixclient, testEnv.kedaClient, testEnv.secretproviderclient)
	testEnv.kubeUtil = kubeUtil
	return testEnv
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

func (suite *VolumeMountTestSuite) Test_ValidBlobCsiAzureVolumeMounts() {
	scenarios := []volumeMountTestScenario{
		{
			radixVolumeMount:   radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume1", Storage: "storageName1", Path: "TestPath1"},
			expectedVolumeName: "csi-az-blob-app-volume1-storageName1",
		},
		{
			radixVolumeMount:   radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume2", Storage: "storageName2", Path: "TestPath2"},
			expectedVolumeName: "csi-az-blob-app-volume2-storageName2",
		},
		{
			radixVolumeMount:           radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume-with-long-name", Storage: "storageName-with-long-name", Path: "TestPath2"},
			expectedVolumeName:         "csi-az-blob-app-volume-with-long-name-storageName-with-lo-",
			expectedVolumeNameIsPrefix: true,
		},
	}
	suite.T().Run("One Blob CSI Azure volume mount ", func(t *testing.T) {
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			t.Logf("Test case %s for component %s", scenarios[0].name, factory.GetTargetType())
			component := utils.NewDeployCommonComponentBuilder(factory).WithName("app").
				WithVolumeMounts(scenarios[0].radixVolumeMount).
				BuildComponent()

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(component, "")
			assert.Nil(t, err)
			assert.Equal(t, 1, len(volumeMounts))
			if len(volumeMounts) > 0 {
				mount := volumeMounts[0]
				assert.Less(t, len(volumeMounts[0].Name), 64)
				assert.Equal(t, scenarios[0].expectedVolumeName, mount.Name)
				assert.Equal(t, scenarios[0].radixVolumeMount.Path, mount.MountPath)
			}
		}
	})
	suite.T().Run("Multiple Blob CSI Azure volume mount ", func(t *testing.T) {
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			t.Logf("Test case %s for component %s", scenarios[0].name, factory.GetTargetType())
			component := utils.NewDeployCommonComponentBuilder(factory).
				WithName("app").
				WithVolumeMounts(scenarios[0].radixVolumeMount, scenarios[1].radixVolumeMount, scenarios[2].radixVolumeMount).
				BuildComponent()

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(component, "")
			assert.Equal(t, 3, len(volumeMounts))
			assert.Nil(t, err)
			for idx, testCase := range scenarios {
				if len(volumeMounts) > 0 {
					assert.Less(t, len(volumeMounts[idx].Name), 64)
					if testCase.expectedVolumeNameIsPrefix {
						assert.True(t, strings.HasPrefix(volumeMounts[idx].Name, testCase.expectedVolumeName))
					} else {
						assert.Equal(t, testCase.expectedVolumeName, volumeMounts[idx].Name)
					}
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
			radixVolumeMount: radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Storage: "storageName1", Path: "TestPath1"},
			expectedError:    "name is empty for volume mount in the component app",
		},
		{
			name:             "Missed volume mount storage",
			radixVolumeMount: radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume1", Path: "TestPath1"},
			expectedError:    "storage is empty for volume mount volume1 in the component app",
		},
		{
			name:             "Missed volume mount path",
			radixVolumeMount: radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume1", Storage: "storageName1"},
			expectedError:    "path is empty for volume mount volume1 in the component app",
		},
	}
	suite.T().Run("Failing Blob CSI Azure volume mount", func(t *testing.T) {
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			for _, testCase := range scenarios {
				t.Logf("Test case %s for component %s", testCase.name, factory.GetTargetType())
				component := utils.NewDeployCommonComponentBuilder(factory).
					WithName("app").
					WithVolumeMounts(testCase.radixVolumeMount).
					BuildComponent()

				_, err := GetRadixDeployComponentVolumeMounts(component, "")
				assert.NotNil(t, err)
				assert.Equal(t, testCase.expectedError, err.Error())
			}
		}
	})
}

func (suite *VolumeMountTestSuite) Test_GetNewVolumes() {
	namespace := "some-namespace"
	componentName := "some-component"
	suite.T().Run("No volumes in component", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts().BuildComponent()
		volumes, err := GetVolumes(context.Background(), testEnv.kubeUtil, namespace, &component, "", nil)
		assert.Nil(t, err)
		assert.Len(t, volumes, 0)
	})
	scenarios := []volumeMountTestScenario{
		{
			name:                  "Blob CSI Azure volume",
			radixVolumeMount:      radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume1", Storage: "storage1", Path: "path1", GID: "1000"},
			expectedVolumeName:    "csi-az-blob-some-component-volume1-storage1",
			expectedPvcNamePrefix: "pvc-csi-az-blob-some-component-volume1-storage1",
		},
		{
			name:                       "Blob CSI Azure volume",
			radixVolumeMount:           radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume-with-long-name", Storage: "storageName-with-long-name", Path: "path1", GID: "1000"},
			expectedVolumeName:         "csi-az-blob-some-component-volume-with-long-name-storageN-",
			expectedVolumeNameIsPrefix: true,
			expectedPvcNamePrefix:      "pvc-csi-az-blob-some-component-volume-with-long-name-storageN-",
		},
	}
	suite.T().Run("CSI Azure volumes", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		for _, scenario := range scenarios {
			t.Logf("Scenario %s", scenario.name)
			component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts(scenario.radixVolumeMount).BuildComponent()
			volumes, err := GetVolumes(context.Background(), testEnv.kubeUtil, namespace, &component, "", nil)
			assert.Nil(t, err)
			assert.Len(t, volumes, 1)
			volume := volumes[0]
			if scenario.expectedVolumeNameIsPrefix {
				assert.True(t, strings.HasPrefix(volume.Name, scenario.expectedVolumeName))
			} else {
				assert.Equal(t, scenario.expectedVolumeName, volume.Name)
			}
			assert.Less(t, len(volume.Name), 64)
			assert.NotNil(t, volume.PersistentVolumeClaim)
			assert.Contains(t, volume.PersistentVolumeClaim.ClaimName, scenario.expectedPvcNamePrefix)
		}
	})
	suite.T().Run("Unsupported volume type", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		mounts := []radixv1.RadixVolumeMount{
			{Type: "unsupported-type", Name: "volume1", Container: "storage1", Path: "path1"},
		}
		component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts(mounts...).BuildComponent()
		volumes, err := GetVolumes(context.Background(), testEnv.kubeUtil, namespace, &component, "", nil)
		assert.Len(t, volumes, 0)
		assert.NotNil(t, err)
		assert.Equal(t, "unsupported volume type unsupported-type", err.Error())
	})
}

func (suite *VolumeMountTestSuite) Test_GetCsiVolumesWithExistingPvcs() {
	namespace := "any-app-some-env"
	componentName := "some-component"
	props := getPropsCsiBlobFuse2Volume1Storage1(nil)
	scenarios := []pvcTestScenario{
		{
			volumeMountTestScenario: volumeMountTestScenario{
				name:               "Blob CSI Azure BlobFuse2 Fuse2 volume",
				radixVolumeMount:   radixv1.RadixVolumeMount{Name: "volume1", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Container: "storage1", GID: "1000"}, Path: "path1"},
				expectedVolumeName: "csi-blobfuse2-fuse2-some-component-volume1-storage1",
			},
			pvc: createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
			pv:  createExpectedPv(props, func(pv *corev1.PersistentVolume) {}),
		},
	}

	suite.T().Run("CSI Azure volumes with existing PVC", func(t *testing.T) {
		t.Parallel()
		for _, scenario := range scenarios {
			t.Logf("Scenario %s for volume mount type %s, PVC status phase '%v'", scenario.name, string(GetCsiAzureVolumeMountType(&scenario.radixVolumeMount)), scenario.pvc.Status.Phase)
			testEnv := getTestEnv()
			_, err := testEnv.kubeclient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})
			require.NoError(t, err)
			_, err = testEnv.kubeclient.CoreV1().PersistentVolumeClaims(namespace).Create(context.Background(), &scenario.pvc, metav1.CreateOptions{})
			require.NoError(t, err)
			_, err = testEnv.kubeclient.CoreV1().PersistentVolumes().Create(context.Background(), &scenario.pv, metav1.CreateOptions{})
			require.NoError(t, err)

			component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts(scenario.radixVolumeMount).BuildComponent()
			volumes, err := GetVolumes(context.Background(), testEnv.kubeUtil, namespace, &component, "", []corev1.Volume{{
				Name:         props.volumeName,
				VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: scenario.pvc.Name}},
			}})
			assert.Nil(t, err)
			assert.Len(t, volumes, 1)
			assert.Equal(t, scenario.expectedVolumeName, volumes[0].Name)
			assert.NotNil(t, volumes[0].PersistentVolumeClaim)
			assert.Equal(t, scenario.pvc.Name, volumes[0].PersistentVolumeClaim.ClaimName)
		}
	})

	suite.T().Run("CSI Azure volumes with no existing PVC", func(t *testing.T) {
		t.Parallel()
		for _, scenario := range scenarios {
			t.Logf("Scenario %s for volume mount type %s, PVC status phase '%v'", scenario.name, string(GetCsiAzureVolumeMountType(&scenario.radixVolumeMount)), scenario.pvc.Status.Phase)
			testEnv := getTestEnv()
			_, err := testEnv.kubeclient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})
			require.NoError(t, err)
			component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts(scenario.radixVolumeMount).BuildComponent()
			volumes, err := GetVolumes(context.Background(), testEnv.kubeUtil, namespace, &component, "", nil)
			assert.Nil(t, err)
			assert.Len(t, volumes, 1)
			assert.Equal(t, scenario.expectedVolumeName, volumes[0].Name)
			assert.NotNil(t, volumes[0].PersistentVolumeClaim)
			assert.NotEqual(t, scenario.pvc.Name, volumes[0].PersistentVolumeClaim.ClaimName)
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
				radixVolumeMount:      radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "blob-volume1", Storage: "storage1", Path: "path1", GID: "1000"},
				expectedVolumeName:    "csi-az-blob-some-component-blob-volume1-storage1",
				expectedPvcNamePrefix: "pvc-csi-az-blob-some-component-blob-volume1-storage1",
			},
			pvc: createPvc(namespace, componentName, radixv1.MountTypeBlobFuse2FuseCsiAzure, func(pvc *corev1.PersistentVolumeClaim) { pvc.Status.Phase = corev1.ClaimBound }),
		},
		{
			volumeMountTestScenario: volumeMountTestScenario{
				name:                  "Blob CSI Azure volume, Status phase: Pending",
				radixVolumeMount:      radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "blob-volume2", Storage: "storage2", Path: "path2", GID: "1000"},
				expectedVolumeName:    "csi-az-blob-some-component-blob-volume2-storage2",
				expectedPvcNamePrefix: "pvc-csi-az-blob-some-component-blob-volume2-storage2",
			},
			pvc: createPvc(namespace, componentName, radixv1.MountTypeBlobFuse2FuseCsiAzure, func(pvc *corev1.PersistentVolumeClaim) { pvc.Status.Phase = corev1.ClaimPending }),
		},
	}

	suite.T().Run("No volumes", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			t.Logf("Test case for component %s", factory.GetTargetType())

			radixDeployment := buildRd(appName, environment, componentName, []radixv1.RadixVolumeMount{})
			deployComponent := radixDeployment.Spec.Components[0]

			volumes, err := GetVolumes(context.Background(), testEnv.kubeUtil, radixDeployment.GetNamespace(), &deployComponent, radixDeployment.GetName(), nil)

			assert.Nil(t, err)
			assert.Len(t, volumes, 0)
		}
	})
	suite.T().Run("Exists volume", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			for _, scenario := range scenarios {
				t.Logf("Test case %s for component %s", scenario.name, factory.GetTargetType())

				radixDeployment := buildRd(appName, environment, componentName, []radixv1.RadixVolumeMount{scenario.radixVolumeMount})
				deployComponent := radixDeployment.Spec.Components[0]

				volumes, err := GetVolumes(context.Background(), testEnv.kubeUtil, radixDeployment.GetNamespace(), &deployComponent, radixDeployment.GetName(), nil)

				assert.Nil(t, err)
				assert.Len(t, volumes, 1)
				assert.Equal(t, scenario.expectedVolumeName, volumes[0].Name)
				assert.NotNil(t, volumes[0].PersistentVolumeClaim)
				assert.Contains(t, volumes[0].PersistentVolumeClaim.ClaimName, scenario.expectedPvcNamePrefix)
			}
		}
	})
}

type expectedPvcPvProperties struct {
	appName                 string
	environment             string
	componentName           string
	radixVolumeMountName    string
	radixStorageName        string
	pvcName                 string
	persistentVolumeName    string
	radixVolumeMountType    radixv1.MountType
	requestsVolumeMountSize string
	volumeAccessMode        corev1.PersistentVolumeAccessMode
	volumeName              string
	pvProvisioner           string
	pvSecretName            string
	pvGid                   string
	pvUid                   string
	namespace               string
	readOnly                bool
}

func (suite *VolumeMountTestSuite) Test_GetRadixDeployComponentVolumeMounts() {
	appName := "any-app"
	environment := "some-env"
	componentName := "some-component"
	scenarios := []volumeMountTestScenario{
		{
			name:                  "Blob CSI Azure volume, Status phase: Bound",
			radixVolumeMount:      radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "blob-volume1", Storage: "storage1", Path: "path1", GID: "1000"},
			expectedVolumeName:    "csi-az-blob-some-component-blob-volume1-storage1",
			expectedPvcNamePrefix: "pvc-csi-az-blob-some-component-blob-volume1-storage1",
		},
		{
			name:                  "Blob CSI Azure volume, Status phase: Pending",
			radixVolumeMount:      radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "blob-volume2", Storage: "storage2", Path: "path2", GID: "1000"},
			expectedVolumeName:    "csi-az-blob-some-component-blob-volume2-storage2",
			expectedPvcNamePrefix: "pvc-csi-az-blob-some-component-blob-volume2-storage2",
		},
		{
			name:                       "Blob CSI Azure volume, Status phase: Pending",
			radixVolumeMount:           radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "blob-volume-with-long-name", Storage: "storage-with-long-name", Path: "path2", GID: "1000"},
			expectedVolumeName:         "csi-az-blob-some-component-blob-volume-with-long-name-sto-",
			expectedVolumeNameIsPrefix: true,
			expectedPvcNamePrefix:      "pvc-csi-az-blob-some-component-blob-volume-with-long-name-",
		},
	}

	suite.T().Run("No volumes", func(t *testing.T) {
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			t.Logf("Test case for component %s", factory.GetTargetType())

			radixDeployment := buildRd(appName, environment, componentName, []radixv1.RadixVolumeMount{})
			deployComponent := radixDeployment.Spec.Components[0]

			volumes, err := GetRadixDeployComponentVolumeMounts(&deployComponent, "")

			assert.Nil(t, err)
			assert.Len(t, volumes, 0)
		}
	})
	suite.T().Run("Exists volume", func(t *testing.T) {
		t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories {
			for _, scenario := range scenarios {
				t.Logf("Test case %s for component %s", scenario.name, factory.GetTargetType())

				radixDeployment := buildRd(appName, environment, componentName, []radixv1.RadixVolumeMount{scenario.radixVolumeMount})
				deployComponent := radixDeployment.Spec.Components[0]

				volumeMounts, err := GetRadixDeployComponentVolumeMounts(&deployComponent, "")

				assert.Nil(t, err)
				assert.Len(t, volumeMounts, 1)
				if scenario.expectedVolumeNameIsPrefix {
					assert.True(t, strings.HasPrefix(volumeMounts[0].Name, scenario.expectedVolumeName))
				} else {
					assert.Equal(t, scenario.expectedVolumeName, volumeMounts[0].Name)
				}
				assert.Less(t, len(volumeMounts[0].Name), 64)
				assert.Equal(t, scenario.radixVolumeMount.Path, volumeMounts[0].MountPath)
			}
		}
	})
}

func modifyPvc(pvc corev1.PersistentVolumeClaim, modify func(pvc *corev1.PersistentVolumeClaim)) corev1.PersistentVolumeClaim {
	modify(&pvc)
	return pvc
}
func modifyPv(pv corev1.PersistentVolume, modify func(pv *corev1.PersistentVolume)) corev1.PersistentVolume {
	modify(&pv)
	return pv
}

func (suite *VolumeMountTestSuite) Test_CreateOrUpdateCsiAzureResources() {
	var scenarios []deploymentVolumesTestScenario
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			return deploymentVolumesTestScenario{
				name:  "Create new volume",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {}),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, nil),
				},
				existingPvcs: []corev1.PersistentVolumeClaim{},
				expectedPvcs: []corev1.PersistentVolumeClaim{
					createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
				},
				existingPvs: []corev1.PersistentVolume{},
				expectedPvs: []corev1.PersistentVolume{
					createExpectedPv(props, func(pv *corev1.PersistentVolume) {}),
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobVolume1Storage1(nil)),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		type scenarioProperties struct {
			changedNewRadixVolumeName        string
			changedNewRadixVolumeStorageName string
			expectedVolumeName               string
			expectedNewSecretName            string
			expectedNewPvcName               string
			expectedNewPvName                string
		}
		getScenario := func(props expectedPvcPvProperties, scenarioProps scenarioProperties) deploymentVolumesTestScenario {
			existingPv := createExpectedPv(props, func(pv *corev1.PersistentVolume) {})
			existingPvc := createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {})
			return deploymentVolumesTestScenario{
				name:  "Update storage in existing volume name and storage",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {
						vm.Name = scenarioProps.changedNewRadixVolumeName
						vm.Storage = scenarioProps.changedNewRadixVolumeStorageName
					}),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, func(v *corev1.Volume) {
						v.Name = scenarioProps.expectedVolumeName
					}),
				},
				existingPvcs: []corev1.PersistentVolumeClaim{
					existingPvc,
				},
				expectedPvcs: []corev1.PersistentVolumeClaim{
					existingPvc,
					createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {
						pvc.ObjectMeta.Name = scenarioProps.expectedNewPvcName
						pvc.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = scenarioProps.changedNewRadixVolumeName
						pvc.Spec.VolumeName = scenarioProps.expectedNewPvName
					}),
				},
				existingPvs: []corev1.PersistentVolume{
					existingPv,
				},
				expectedPvs: []corev1.PersistentVolume{
					existingPv,
					createExpectedPv(props, func(pv *corev1.PersistentVolume) {
						pv.ObjectMeta.Name = scenarioProps.expectedNewPvName
						pv.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = scenarioProps.changedNewRadixVolumeName
						pv.ObjectMeta.Annotations[persistentvolume.CsiAnnotationProvisionerDeletionSecretName] = scenarioProps.expectedNewSecretName
						setVolumeMountAttribute(pv, props.radixVolumeMountType, scenarioProps.changedNewRadixVolumeStorageName, scenarioProps.expectedNewPvcName)
						pv.Spec.ClaimRef.Name = scenarioProps.expectedNewPvcName
						pv.Spec.CSI.NodeStageSecretRef.Name = scenarioProps.expectedNewSecretName
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
				expectedNewPvName:                "pv-radixvolumemount-some-uuid",
			}),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			existingPvc := createRandomPvc(props, props.namespace, props.componentName)
			expectedPvc := createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
			})
			existingPv := createRandomPv(props, props.namespace, props.componentName)
			expectedPv := createExpectedPv(props, nil)
			return deploymentVolumesTestScenario{
				name:  "Set readonly volume",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) { vm.AccessMode = string(corev1.ReadOnlyMany) }),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, func(v *corev1.Volume) {}),
				},
				existingPvcs: []corev1.PersistentVolumeClaim{
					existingPvc,
				},
				expectedPvcs: []corev1.PersistentVolumeClaim{
					existingPvc,
					expectedPvc,
				},
				existingPvs: []corev1.PersistentVolume{
					existingPv,
				},
				expectedPvs: []corev1.PersistentVolume{
					existingPv,
					expectedPv,
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobVolume1Storage1(func(props *expectedPvcPvProperties) {
				props.readOnly = false
			})),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			existingPvc := createExpectedPvc(props, nil)
			existingPv := createExpectedPv(props, nil)
			matchPvAndPvc(&existingPv, &existingPvc)
			expectedPv := modifyPv(existingPv, func(pv *corev1.PersistentVolume) {
				pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
				pv.Spec.MountOptions = getMountOptions(props)
			})
			expectedPvc := modifyPvc(existingPvc, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
			})
			return deploymentVolumesTestScenario{
				name:  "Set ReadWriteOnce volume",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) { vm.AccessMode = string(corev1.ReadWriteOnce) }),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, func(v *corev1.Volume) {}),
				},
				existingPvcs: []corev1.PersistentVolumeClaim{
					existingPvc,
				},
				expectedPvcs: []corev1.PersistentVolumeClaim{
					existingPvc,
					expectedPvc,
				},
				existingPvs: []corev1.PersistentVolume{
					existingPv,
				},
				expectedPvs: []corev1.PersistentVolume{
					existingPv,
					expectedPv,
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobVolume1Storage1(nil)),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			existingPvc := createExpectedPvc(props, nil)
			existingPv := createExpectedPv(props, nil)
			matchPvAndPvc(&existingPv, &existingPvc)
			return deploymentVolumesTestScenario{
				name:  "Set ReadWriteMany volume",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) { vm.AccessMode = string(corev1.ReadWriteMany) }),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, func(v *corev1.Volume) {}),
				},
				existingPvcs: []corev1.PersistentVolumeClaim{
					existingPvc,
				},
				expectedPvcs: []corev1.PersistentVolumeClaim{
					existingPvc,
					modifyPvc(existingPvc, func(pvc *corev1.PersistentVolumeClaim) {
						pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
					}),
				},
				existingPvs: []corev1.PersistentVolume{
					existingPv,
				},
				expectedPvs: []corev1.PersistentVolume{
					existingPv,
					modifyPv(existingPv, func(pv *corev1.PersistentVolume) {
						pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
						pv.Spec.MountOptions = getMountOptions(modifyProps(props, func(props *expectedPvcPvProperties) { props.readOnly = false }))
					}),
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobVolume1Storage1(nil)),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			existingPvc := createExpectedPvc(props, nil)
			existingPv := createExpectedPv(props, nil)
			matchPvAndPvc(&existingPv, &existingPvc)
			existingPv = modifyPv(existingPv, func(pv *corev1.PersistentVolume) {
				pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
				pv.Spec.MountOptions = getMountOptions(modifyProps(props, func(props *expectedPvcPvProperties) { props.readOnly = false }))
			})
			existingPvc = modifyPvc(existingPvc, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
			})
			return deploymentVolumesTestScenario{
				name:  "Set ReadOnlyMany volume",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) { vm.AccessMode = string(corev1.ReadOnlyMany) }),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, func(v *corev1.Volume) {}),
				},
				existingPvcs: []corev1.PersistentVolumeClaim{
					existingPvc,
				},
				expectedPvcs: []corev1.PersistentVolumeClaim{
					existingPvc,
					modifyPvc(existingPvc, func(pvc *corev1.PersistentVolumeClaim) {
						pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
					}),
				},
				existingPvs: []corev1.PersistentVolume{
					existingPv,
				},
				expectedPvs: []corev1.PersistentVolume{
					existingPv,
					modifyPv(existingPv, func(pv *corev1.PersistentVolume) {
						pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
					}),
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobVolume1Storage1(nil)),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			return deploymentVolumesTestScenario{
				name:  "Create new BlobFuse2 volume has streaming by default and streaming options not set",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {}),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, func(v *corev1.Volume) {}),
				},
				existingPvcs: []corev1.PersistentVolumeClaim{},
				expectedPvcs: []corev1.PersistentVolumeClaim{
					createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
				},
				existingPvs: []corev1.PersistentVolume{},
				expectedPvs: []corev1.PersistentVolume{
					createExpectedPv(props, func(pv *corev1.PersistentVolume) {
						pv.Spec.MountOptions = getMountOptions(props, "--streaming=true", "--use-adls=false")
					}),
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobFuse2Volume1Storage1(nil)),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			return deploymentVolumesTestScenario{
				name:  "Create new BlobFuse2 volume has implicit streaming by default and streaming options set",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {
						vm.BlobFuse2.Streaming = &radixv1.RadixVolumeMountStreaming{
							StreamCache:      pointers.Ptr(uint64(101)),
							BlockSize:        pointers.Ptr(uint64(102)),
							BufferSize:       pointers.Ptr(uint64(103)),
							MaxBuffers:       pointers.Ptr(uint64(104)),
							MaxBlocksPerFile: pointers.Ptr(uint64(105)),
						}
					}),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, func(v *corev1.Volume) {}),
				},
				existingPvcs: []corev1.PersistentVolumeClaim{},
				expectedPvcs: []corev1.PersistentVolumeClaim{
					createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
				},
				existingPvs: []corev1.PersistentVolume{},
				expectedPvs: []corev1.PersistentVolume{
					createExpectedPv(props, func(pv *corev1.PersistentVolume) {
						pv.Spec.MountOptions = getMountOptions(props,
							"--streaming=true",
							"--stream-cache-mb=101",
							"--block-size-mb=102",
							"--buffer-size-mb=103",
							"--max-buffers=104",
							"--max-blocks-per-file=105",
							"--use-adls=false")
					}),
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobFuse2Volume1Storage1(nil)),
		}
	}()...)

	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			return deploymentVolumesTestScenario{
				name:  "Create new BlobFuse2 volume has disabled streaming",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {
						vm.BlobFuse2.Streaming = &radixv1.RadixVolumeMountStreaming{
							Enabled:          pointers.Ptr(false),
							StreamCache:      pointers.Ptr(uint64(101)),
							BlockSize:        pointers.Ptr(uint64(102)),
							BufferSize:       pointers.Ptr(uint64(103)),
							MaxBuffers:       pointers.Ptr(uint64(104)),
							MaxBlocksPerFile: pointers.Ptr(uint64(105)),
						}
					}),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, func(v *corev1.Volume) {}),
				},
				existingPvcs: []corev1.PersistentVolumeClaim{},
				expectedPvcs: []corev1.PersistentVolumeClaim{
					createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
				},
				existingPvs: []corev1.PersistentVolume{},
				expectedPvs: []corev1.PersistentVolume{
					createExpectedPv(props, func(pv *corev1.PersistentVolume) {
						pv.Spec.MountOptions = getMountOptions(props,
							"--use-adls=false")
					}),
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobFuse2Volume1Storage1(nil)),
		}
	}()...)

	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			pvForAnotherComponent := createRandomAutoProvisionedPvWithStorageClass(props, props.namespace, anotherComponentName, anotherVolumeMountName)
			pvcForAnotherComponent := createRandomAutoProvisionedPvcWithStorageClass(props, props.namespace, anotherComponentName, anotherVolumeMountName)
			matchPvAndPvc(&pvForAnotherComponent, &pvcForAnotherComponent)
			volume := createTestVolume(props, func(v *corev1.Volume) {})
			existingPv := createAutoProvisionedPvWithStorageClass(props, func(pv *corev1.PersistentVolume) { pv.Spec.ClaimRef.Name = volume.PersistentVolumeClaim.ClaimName })
			expectedPvc := createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {})
			expectedPv := createExpectedPv(props, func(pv *corev1.PersistentVolume) {})
			matchPvAndPvc(&expectedPv, &expectedPvc)
			return deploymentVolumesTestScenario{
				name:  "Do not change existing PersistentVolume with class name, when creating new PVC",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createRandomVolumeMount(func(vm *radixv1.RadixVolumeMount) { vm.Name = anotherVolumeMountName }),
					createRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {}),
				},
				volumes: []corev1.Volume{
					volume,
				},
				existingPvcs: []corev1.PersistentVolumeClaim{
					pvcForAnotherComponent,
				},
				expectedPvcs: []corev1.PersistentVolumeClaim{
					expectedPvc,
					pvcForAnotherComponent,
				},
				existingPvs: []corev1.PersistentVolume{
					existingPv,
					pvForAnotherComponent,
				},
				expectedPvs: []corev1.PersistentVolume{
					expectedPv,
					existingPv,
					pvForAnotherComponent,
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobVolume1Storage1(nil)),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			pvForAnotherComponent := createRandomPv(props, props.namespace, anotherComponentName)
			pvcForAnotherComponent := createRandomPvc(props, props.namespace, anotherComponentName)
			matchPvAndPvc(&pvForAnotherComponent, &pvcForAnotherComponent)
			existingPv := createExpectedPv(props, func(pv *corev1.PersistentVolume) {})
			return deploymentVolumesTestScenario{
				name:  "Do not change existing PersistentVolume without class name, when creating new PVC",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {}),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, func(v *corev1.Volume) {}),
				},
				existingPvcs: []corev1.PersistentVolumeClaim{
					pvcForAnotherComponent,
				},
				expectedPvcs: []corev1.PersistentVolumeClaim{
					createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
					pvcForAnotherComponent,
				},
				existingPvs: []corev1.PersistentVolume{
					existingPv,
					pvForAnotherComponent,
				},
				expectedPvs: []corev1.PersistentVolume{
					existingPv,
					pvForAnotherComponent,
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobVolume1Storage1(nil)),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			pvForAnotherComponent := createRandomAutoProvisionedPvWithStorageClass(props, props.namespace, anotherComponentName, anotherVolumeMountName)
			pvcForAnotherComponent := createRandomAutoProvisionedPvcWithStorageClass(props, props.namespace, anotherComponentName, anotherVolumeMountName)
			matchPvAndPvc(&pvForAnotherComponent, &pvcForAnotherComponent)
			existingPvc := createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {})
			expectedPvc := createRandomPvc(props, props.namespace, componentName)
			expectedPv := createRandomPv(props, props.namespace, componentName)
			matchPvAndPvc(&expectedPv, &expectedPvc)
			return deploymentVolumesTestScenario{
				name:  "Do not change existing PVC with class name, when creating new PersistentVolume",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {}),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, func(v *corev1.Volume) {}),
				},
				existingPvcs: []corev1.PersistentVolumeClaim{
					pvcForAnotherComponent,
					existingPvc,
				},
				expectedPvcs: []corev1.PersistentVolumeClaim{
					pvcForAnotherComponent,
					expectedPvc,
				},
				existingPvs: []corev1.PersistentVolume{
					pvForAnotherComponent,
				},
				expectedPvs: []corev1.PersistentVolume{
					pvForAnotherComponent,
					expectedPv,
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobVolume1Storage1(nil)),
		}
	}()...)

	suite.T().Run("CSI Azure volume PVCs and PersistentVolume", func(t *testing.T) {
		for _, factory := range suite.radixCommonDeployComponentFactories[:1] {
			for _, scenario := range scenarios {
				t.Logf("Test case %s, volume type %s, component %s", scenario.name, scenario.props.radixVolumeMountType, factory.GetTargetType())
				testEnv := getTestEnv()
				radixDeployment := buildRd(appName, environment, componentName, scenario.radixVolumeMounts)
				putExistingDeploymentVolumesScenarioDataToFakeCluster(testEnv.kubeUtil.KubeClient(), &scenario)
				desiredVolumes := getDesiredDeployment(componentName, scenario.volumes).Spec.Template.Spec.Volumes

				deployComponent := radixDeployment.Spec.Components[0]
				actualVolumes, err := CreateOrUpdateCsiAzureVolumeResourcesForDeployComponent(context.Background(), testEnv.kubeUtil.KubeClient(), radixDeployment, utils.GetEnvironmentNamespace(appName, environment), &deployComponent, desiredVolumes)
				require.NoError(t, err)
				assert.Equal(t, len(scenario.volumes), len(actualVolumes), "Number of volumes is not equal")

				existingPvcs, existingPvs, err := getExistingPvcsAndPersistentVolumeFromFakeCluster(testEnv.kubeUtil.KubeClient())
				require.NoError(t, err)
				assert.Len(t, existingPvcs, len(scenario.expectedPvcs), "PVC-s count is not equal")
				assert.True(t, equalPersistentVolumeClaims(&scenario.expectedPvcs, &existingPvcs), "PVC-s are not equal")
				assert.Len(t, existingPvs, len(scenario.expectedPvs), "PV-s count is not equal")
				assert.True(t, equalPersistentVolumes(&scenario.expectedPvs, &existingPvs), "PV-s are not equal")
			}
		}
	})
}

func createRandomVolumeMount(modify func(mount *radixv1.RadixVolumeMount)) radixv1.RadixVolumeMount {
	vm := radixv1.RadixVolumeMount{
		Name: strings.ToLower(utils.RandString(10)),
		Path: "/tmp/" + strings.ToLower(utils.RandString(10)),
		BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
			Protocol:  radixv1.BlobFuse2ProtocolFuse2,
			Container: strings.ToLower(utils.RandString(10)),
		},
	}
	modify(&vm)
	return vm
}

func matchPvAndPvc(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
	pv.Spec.CSI.VolumeAttributes[persistentvolume.CsiVolumeMountAttributePvcName] = pvc.GetName()
	pv.Spec.CSI.VolumeAttributes[persistentvolume.CsiVolumeMountAttributePvcNamespace] = pvc.GetNamespace()
	pv.Spec.ClaimRef = &corev1.ObjectReference{
		APIVersion: "radixv1",
		Kind:       k8s.KindPersistentVolumeClaim,
		Name:       pvc.GetName(),
		Namespace:  pvc.GetNamespace(),
	}
	pvc.Spec.VolumeName = pv.Name
}

func modifyProps(props expectedPvcPvProperties, modify func(props *expectedPvcPvProperties)) expectedPvcPvProperties {
	modify(&props)
	return props
}

func createRandomPv(props expectedPvcPvProperties, namespace, componentName string) corev1.PersistentVolume {
	return createExpectedPv(props, func(pv *corev1.PersistentVolume) {
		pvName := getCsiAzurePvName()
		pv.ObjectMeta.Name = pvName
		pv.ObjectMeta.Labels[kube.RadixNamespace] = namespace
		pv.ObjectMeta.Labels[kube.RadixComponentLabel] = componentName
		pv.Spec.CSI.VolumeAttributes[persistentvolume.CsiVolumeMountAttributePvName] = pvName
	})
}

func createRandomPvc(props expectedPvcPvProperties, namespace, componentName string) corev1.PersistentVolumeClaim {
	return createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {
		pvcName, err := getCsiAzurePvcName(componentName, &radixv1.RadixVolumeMount{Name: props.radixVolumeMountName, Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Storage: props.radixStorageName, Path: "/tmp"})
		if err != nil {
			panic(err)
		}
		pvName := getCsiAzurePvName()
		pvc.ObjectMeta.Name = pvcName
		pvc.ObjectMeta.Namespace = namespace
		pvc.ObjectMeta.Labels[kube.RadixComponentLabel] = componentName
		pvc.Spec.VolumeName = pvName
	})
}

func createRandomAutoProvisionedPvWithStorageClass(props expectedPvcPvProperties, namespace, componentName, anotherVolumeMountName string) corev1.PersistentVolume {
	return createAutoProvisionedPvWithStorageClass(props, func(pv *corev1.PersistentVolume) {
		pvName := "pvc-" + uuid.NewString()
		pv.ObjectMeta.Name = pvName
		if pv.ObjectMeta.Labels == nil {
			pv.ObjectMeta.Labels = make(map[string]string)
		}
		pv.ObjectMeta.Labels[kube.RadixNamespace] = namespace
		pv.ObjectMeta.Labels[kube.RadixComponentLabel] = componentName
		pv.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = anotherVolumeMountName
		pv.Spec.CSI.VolumeAttributes[persistentvolume.CsiVolumeMountAttributePvName] = pvName
	})
}

func createRandomAutoProvisionedPvcWithStorageClass(props expectedPvcPvProperties, namespace, componentName, anotherVolumeMountName string) corev1.PersistentVolumeClaim {
	return createExpectedAutoProvisionedPvcWithStorageClass(props, func(pvc *corev1.PersistentVolumeClaim) {
		pvcName, err := getCsiAzurePvcName(componentName, &radixv1.RadixVolumeMount{Name: props.radixVolumeMountName, Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Storage: props.radixStorageName, Path: "/tmp"})
		if err != nil {
			panic(err)
		}
		pvName := getCsiAzurePvName()
		pvc.ObjectMeta.Name = pvcName
		pvc.ObjectMeta.Namespace = namespace
		pvc.ObjectMeta.Labels[kube.RadixComponentLabel] = componentName
		pvc.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = anotherVolumeMountName
		pvc.Spec.VolumeName = pvName
	})
}

func getPropsCsiBlobVolume1Storage1(modify func(*expectedPvcPvProperties)) expectedPvcPvProperties {
	props := expectedPvcPvProperties{
		appName:                 appName,
		environment:             environment,
		namespace:               utils.GetEnvironmentNamespace(appName, environment),
		componentName:           componentName,
		radixVolumeMountName:    "volume1",
		radixStorageName:        "storage1",
		pvcName:                 "pvc-csi-az-blob-some-component-volume1-storage1-12345",
		persistentVolumeName:    "pv-radixvolumemount-some-uuid",
		radixVolumeMountType:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
		requestsVolumeMountSize: "1Mi",
		volumeAccessMode:        corev1.ReadOnlyMany, // default access mode
		volumeName:              "csi-az-blob-some-component-volume1-storage1",
		pvProvisioner:           provisionerBlobCsiAzure,
		pvSecretName:            "some-component-volume1-csiazurecreds",
		pvGid:                   "1000",
		pvUid:                   "",
		readOnly:                true,
	}
	if modify != nil {
		modify(&props)
	}
	return props
}

func getPropsCsiBlobFuse2Volume1Storage1(modify func(*expectedPvcPvProperties)) expectedPvcPvProperties {
	props := expectedPvcPvProperties{
		appName:                 appName,
		environment:             environment,
		namespace:               fmt.Sprintf("%s-%s", appName, environment),
		componentName:           componentName,
		radixVolumeMountName:    "volume1",
		radixStorageName:        "storage1",
		pvcName:                 "pvc-csi-blobfuse2-fuse2-some-component-volume1-storage1-12345",
		persistentVolumeName:    "pv-radixvolumemount-some-uuid",
		radixVolumeMountType:    radixv1.MountTypeBlobFuse2Fuse2CsiAzure,
		requestsVolumeMountSize: "1Mi",
		volumeAccessMode:        corev1.ReadOnlyMany, // default access mode
		volumeName:              "csi-blobfuse2-fuse2-some-component-volume1-storage1",
		pvProvisioner:           provisionerBlobCsiAzure,
		pvSecretName:            "some-component-volume1-csiazurecreds",
		pvGid:                   "1000",
		pvUid:                   "",
		readOnly:                true,
	}
	if modify != nil {
		modify(&props)
	}
	return props
}

func putExistingDeploymentVolumesScenarioDataToFakeCluster(kubeClient kubernetes.Interface, scenario *deploymentVolumesTestScenario) {
	for _, pvc := range scenario.existingPvcs {
		_, _ = kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(context.Background(), &pvc, metav1.CreateOptions{})
	}
	for _, pv := range scenario.existingPvs {
		_, _ = kubeClient.CoreV1().PersistentVolumes().Create(context.Background(), &pv, metav1.CreateOptions{})
	}
}

func getExistingPvcsAndPersistentVolumeFromFakeCluster(kubeClient kubernetes.Interface) ([]corev1.PersistentVolumeClaim, []corev1.PersistentVolume, error) {
	var pvcItems []corev1.PersistentVolumeClaim
	var pvItems []corev1.PersistentVolume
	pvcList, err := kubeClient.CoreV1().PersistentVolumeClaims("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	if pvcList != nil && pvcList.Items != nil {
		pvcItems = pvcList.Items
	}
	pvList, err := kubeClient.CoreV1().PersistentVolumes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	if pvList != nil && pvList.Items != nil {
		pvItems = pvList.Items
	}
	return pvcItems, pvItems, nil
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
			Replicas: pointers.Ptr(defaults.DefaultReplicas),
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

func buildRd(appName string, environment string, componentName string, radixVolumeMounts []radixv1.RadixVolumeMount) *radixv1.RadixDeployment {
	return utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(environment).
		WithComponents(utils.NewDeployComponentBuilder().
			WithName(componentName).
			WithVolumeMounts(radixVolumeMounts...)).
		BuildRD()
}

func createPvc(namespace, componentName string, mountType radixv1.MountType, modify func(*corev1.PersistentVolumeClaim)) corev1.PersistentVolumeClaim {
	appName := "app"
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      commonUtils.RandString(10), // Set in test scenario
			Namespace: namespace,
			Labels: map[string]string{
				kube.RadixAppLabel:             appName,
				kube.RadixComponentLabel:       componentName,
				kube.RadixMountTypeLabel:       string(mountType),
				kube.RadixVolumeMountNameLabel: commonUtils.RandString(10), // Set in test scenario
			},
		},
	}
	if modify != nil {
		modify(&pvc)
	}
	return pvc
}

func createExpectedPv(props expectedPvcPvProperties, modify func(pv *corev1.PersistentVolume)) corev1.PersistentVolume {
	mountOptions := getMountOptions(props)
	pv := corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: props.persistentVolumeName,
			Labels: map[string]string{
				kube.RadixAppLabel:             props.appName,
				kube.RadixNamespace:            props.namespace,
				kube.RadixComponentLabel:       props.componentName,
				kube.RadixVolumeMountNameLabel: props.radixVolumeMountName,
			},
			Annotations: map[string]string{
				persistentvolume.CsiAnnotationProvisionedBy:                      provisionerBlobCsiAzure,
				persistentvolume.CsiAnnotationProvisionerDeletionSecretName:      props.pvSecretName,
				persistentvolume.CsiAnnotationProvisionerDeletionSecretNamespace: props.namespace,
			},
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(props.requestsVolumeMountSize)},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       provisionerBlobCsiAzure,
					VolumeHandle: getVolumeHandle(props.namespace, props.componentName, props.persistentVolumeName, props.radixStorageName),
					VolumeAttributes: map[string]string{
						persistentvolume.CsiVolumeMountAttributeContainerName:   props.radixStorageName,
						persistentvolume.CsiVolumeMountAttributeProtocol:        persistentvolume.CsiVolumeAttributeProtocolParameterFuse2,
						persistentvolume.CsiVolumeMountAttributePvName:          props.persistentVolumeName,
						persistentvolume.CsiVolumeMountAttributePvcName:         props.pvcName,
						persistentvolume.CsiVolumeMountAttributePvcNamespace:    props.namespace,
						persistentvolume.CsiVolumeMountAttributeSecretNamespace: props.namespace,
						// skip auto-created by the provisioner "storage.kubernetes.io/csiProvisionerIdentity": "1732528668611-2190-blob.csi.azure.com"
					},
					NodeStageSecretRef: &corev1.SecretReference{
						Name:      props.pvSecretName,
						Namespace: props.namespace,
					},
				},
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{props.volumeAccessMode},
			ClaimRef: &corev1.ObjectReference{
				APIVersion: "radixv1",
				Kind:       k8s.KindPersistentVolumeClaim,
				Namespace:  props.namespace,
				Name:       props.pvcName,
			},
			StorageClassName:              "",
			MountOptions:                  mountOptions,
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			VolumeMode:                    pointers.Ptr(corev1.PersistentVolumeFilesystem),
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
	}
	setVolumeMountAttribute(&pv, props.radixVolumeMountType, props.radixStorageName, props.pvcName)
	if modify != nil {
		modify(&pv)
	}
	return pv
}

func createAutoProvisionedPvWithStorageClass(props expectedPvcPvProperties, modify func(pv *corev1.PersistentVolume)) corev1.PersistentVolume {
	mountOptions := getMountOptionsInRandomOrder(props)
	pv := corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: props.persistentVolumeName,
			Annotations: map[string]string{
				persistentvolume.CsiAnnotationProvisionedBy:                      provisionerBlobCsiAzure,
				persistentvolume.CsiAnnotationProvisionerDeletionSecretName:      props.pvSecretName,
				persistentvolume.CsiAnnotationProvisionerDeletionSecretNamespace: props.namespace,
			},
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(props.requestsVolumeMountSize)},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       provisionerBlobCsiAzure,
					VolumeHandle: "MC_clusters_ABC_northeurope##testdata#pvc-681b9ffc-66cc-4e09-90b2-872688b792542#some-app-namespace#",
					VolumeAttributes: map[string]string{
						persistentvolume.CsiVolumeMountAttributeContainerName:       props.radixStorageName,
						persistentvolume.CsiVolumeMountAttributeProtocol:            persistentvolume.CsiVolumeAttributeProtocolParameterFuse2,
						persistentvolume.CsiVolumeMountAttributePvName:              props.persistentVolumeName,
						persistentvolume.CsiVolumeMountAttributePvcName:             props.pvcName,
						persistentvolume.CsiVolumeMountAttributePvcNamespace:        props.namespace,
						persistentvolume.CsiVolumeMountAttributeSecretNamespace:     props.namespace,
						persistentvolume.CsiVolumeMountAttributeProvisionerIdentity: "6540128941979-5154-blob.csi.azure.com",
					},
					NodeStageSecretRef: &corev1.SecretReference{
						Name:      props.pvSecretName,
						Namespace: props.namespace,
					},
				},
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{props.volumeAccessMode},
			ClaimRef: &corev1.ObjectReference{
				APIVersion: "radixv1",
				Kind:       k8s.KindPersistentVolumeClaim,
				Namespace:  props.namespace,
				Name:       props.pvcName,
			},
			StorageClassName:              "some-storage-class",
			MountOptions:                  mountOptions,
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			VolumeMode:                    pointers.Ptr(corev1.PersistentVolumeFilesystem),
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound, LastPhaseTransitionTime: pointers.Ptr(metav1.Time{Time: time.Now()})},
	}
	setVolumeMountAttribute(&pv, props.radixVolumeMountType, props.radixStorageName, props.pvcName)
	if modify != nil {
		modify(&pv)
	}
	return pv
}

func getMountOptions(props expectedPvcPvProperties, extraOptions ...string) []string {
	options := []string{
		"--file-cache-timeout-in-seconds=120",
		"--use-attr-cache=true",
		"--cancel-list-on-mount-seconds=0",
		"-o allow_other",
		"-o attr_timeout=120",
		"-o entry_timeout=120",
		"-o negative_timeout=120",
	}
	if props.readOnly {
		options = append(options, ReadOnlyMountOption)
	}
	idOption := getPersistentVolumeIdMountOption(props)
	if len(idOption) > 0 {
		options = append(options, idOption)
	}
	return append(options, extraOptions...)
}

func getMountOptionsInRandomOrder(props expectedPvcPvProperties, extraOptions ...string) []string {
	options := []string{
		"--file-cache-timeout-in-seconds=120",
		"--use-attr-cache=true",
		"-o allow_other",
		"--cancel-list-on-mount-seconds=0",
		"-o negative_timeout=120",
		"-o entry_timeout=120",
		"-o attr_timeout=120",
	}
	idOption := getPersistentVolumeIdMountOption(props)
	if len(idOption) > 0 {
		options = append(options, idOption)
	}
	if props.readOnly {
		options = append(options, ReadOnlyMountOption)
	}
	return append(options, extraOptions...)
}

func setVolumeMountAttribute(pv *corev1.PersistentVolume, radixVolumeMountType radixv1.MountType, containerName, pvcName string) {
	pv.Spec.CSI.VolumeAttributes[persistentvolume.CsiVolumeMountAttributeContainerName] = containerName
	pv.Spec.CSI.VolumeAttributes[persistentvolume.CsiVolumeMountAttributePvcName] = pvcName
	switch radixVolumeMountType {
	case radixv1.MountTypeBlobFuse2FuseCsiAzure:
		pv.Spec.CSI.VolumeAttributes[persistentvolume.CsiVolumeMountAttributeProtocol] = persistentvolume.CsiVolumeAttributeProtocolParameterFuse
	case radixv1.MountTypeBlobFuse2Fuse2CsiAzure:
		pv.Spec.CSI.VolumeAttributes[persistentvolume.CsiVolumeMountAttributeProtocol] = persistentvolume.CsiVolumeAttributeProtocolParameterFuse2
	}
}

func getPersistentVolumeIdMountOption(props expectedPvcPvProperties) string {
	if len(props.pvGid) > 0 {
		return fmt.Sprintf("-o gid=%s", props.pvGid)
	}
	if len(props.pvUid) > 0 {
		return fmt.Sprintf("-o uid=%s", props.pvGid)
	}
	return ""
}

func createExpectedPvc(props expectedPvcPvProperties, modify func(*corev1.PersistentVolumeClaim)) corev1.PersistentVolumeClaim {
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
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(props.requestsVolumeMountSize)}, // it seems correct number is not needed for CSI driver
			},
			VolumeName: props.persistentVolumeName,
			VolumeMode: pointers.Ptr(corev1.PersistentVolumeFilesystem),
		},
		Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
	}
	if modify != nil {
		modify(&pvc)
	}
	return pvc
}

func createExpectedAutoProvisionedPvcWithStorageClass(props expectedPvcPvProperties, modify func(*corev1.PersistentVolumeClaim)) corev1.PersistentVolumeClaim {
	labels := map[string]string{
		kube.RadixAppLabel:             props.appName,
		kube.RadixComponentLabel:       props.componentName,
		kube.RadixMountTypeLabel:       string(props.radixVolumeMountType),
		kube.RadixVolumeMountNameLabel: props.radixVolumeMountName,
	}
	annotations := map[string]string{
		"pv.kubernetes.io/bind-completed":               "yes",
		"pv.kubernetes.io/bound-by-controller":          "yes",
		"volume.beta.kubernetes.io/storage-provisioner": provisionerBlobCsiAzure,
		"volume.kubernetes.io/storage-provisioner":      provisionerBlobCsiAzure,
	}
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:            props.pvcName,
			Namespace:       props.namespace,
			Labels:          labels,
			Annotations:     annotations,
			Finalizers:      []string{"kubernetes.io/pvc-protection"},
			ResourceVersion: "630363277",
			UID:             types.UID("681b9ffc-66cc-4e09-90b2-872688b792542"),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{props.volumeAccessMode},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(props.requestsVolumeMountSize)}, // it seems correct number is not needed for CSI driver
			},
			VolumeName:       props.persistentVolumeName,
			StorageClassName: pointers.Ptr("some-storage-class"),
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase:       corev1.ClaimBound,
			AccessModes: []corev1.PersistentVolumeAccessMode{props.volumeAccessMode},
			Capacity:    map[corev1.ResourceName]resource.Quantity{corev1.ResourceStorage: resource.MustParse(props.requestsVolumeMountSize)},
		},
	}
	if modify != nil {
		modify(&pvc)
	}
	return pvc
}

func createTestVolume(pvcProps expectedPvcPvProperties, modify func(v *corev1.Volume)) corev1.Volume {
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

func createRandomVolume(modify func(*corev1.Volume)) corev1.Volume {
	volume := corev1.Volume{
		Name: strings.ToLower(utils.RandString(10)),
		VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: strings.ToLower(utils.RandString(10)),
		}},
	}
	if modify != nil {
		modify(&volume)
	}
	return volume
}

func createRadixVolumeMount(props expectedPvcPvProperties, modify func(mount *radixv1.RadixVolumeMount)) radixv1.RadixVolumeMount {
	volumeMount := radixv1.RadixVolumeMount{
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
func createBlobFuse2RadixVolumeMount(props expectedPvcPvProperties, modify func(mount *radixv1.RadixVolumeMount)) radixv1.RadixVolumeMount {
	volumeMount := radixv1.RadixVolumeMount{
		Name: props.radixVolumeMountName,
		Path: "path1",
		BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
			Container: props.radixStorageName,
			GID:       "1000",
		},
	}
	if modify != nil {
		modify(&volumeMount)
	}
	return volumeMount
}

func equalPersistentVolumes(expectedPvs, actualPvs *[]corev1.PersistentVolume) bool {
	if len(*expectedPvs) != len(*actualPvs) {
		return false
	}
	for _, expectedPv := range *expectedPvs {
		var hasEqualPv bool
		for _, actualPv := range *actualPvs {
			if persistentvolume.EqualPersistentVolumes(&expectedPv, &actualPv) {
				hasEqualPv = true
				break
			}
		}
		if !hasEqualPv {
			return false
		}
	}
	return true
}

func equalPersistentVolumeClaims(list1, list2 *[]corev1.PersistentVolumeClaim) bool {
	if len(*list1) != len(*list2) {
		return false
	}
	for _, pvc1 := range *list1 {
		var hasEqualPvc bool
		for _, pvc2 := range *list2 {
			if internal.EqualTillPostfix(pvc1.GetName(), pvc2.GetName(), 5) &&
				equalPrefix(pvc1.Spec.VolumeName, pvc2.Spec.VolumeName, 20) &&
				persistentvolume.EqualPersistentVolumeClaims(&pvc1, &pvc2) {
				hasEqualPvc = true
				break
			}
		}
		if !hasEqualPvc {
			return false
		}
	}
	return true
}

func equalPrefix(value1, value2 string, prefixLength int) bool {
	if len(value1) < prefixLength || len(value2) < prefixLength {
		return false
	}
	return value1[:prefixLength] == value2[:prefixLength]
}
