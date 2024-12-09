package deployment

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/internal/persistentvolume"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
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
	"k8s.io/apimachinery/pkg/util/strategicpatch"
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
	kedaClient           kedav2.Interface
}

type volumeMountTestScenario struct {
	name                       string
	radixVolumeMount           v1.RadixVolumeMount
	expectedVolumeName         string
	expectedVolumeNameIsPrefix bool
	expectedError              string
	expectedPvcNamePrefix      string
}

type deploymentVolumesTestScenario struct {
	name                      string
	props                     expectedPvcPvProperties
	radixVolumeMounts         []v1.RadixVolumeMount
	volumes                   []corev1.Volume
	existingPVsBeforeTestRun  []corev1.PersistentVolume
	existingPvcsBeforeTestRun []corev1.PersistentVolumeClaim
	existingPVsAfterTestRun   []corev1.PersistentVolume
	existingPvcsAfterTestRun  []corev1.PersistentVolumeClaim
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
		kedaClient:           kedafake.NewSimpleClientset(),
		secretproviderclient: secretproviderfake.NewSimpleClientset(),
		prometheusclient:     prometheusfake.NewSimpleClientset(),
	}
	kubeUtil, _ := kube.New(testEnv.kubeclient, testEnv.radixclient, testEnv.kedaClient, testEnv.secretproviderclient)
	testEnv.kubeUtil = kubeUtil
	return testEnv
}

func getDeployment(testEnv TestEnv) *Deployment {
	return &Deployment{
		kubeclient:              testEnv.kubeclient,
		radixclient:             testEnv.radixclient,
		kubeutil:                testEnv.kubeUtil,
		prometheusperatorclient: testEnv.prometheusclient,
		config:                  &config.Config{},
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

func (suite *VolumeMountTestSuite) Test_ValidBlobCsiAzureVolumeMounts() {
	scenarios := []volumeMountTestScenario{
		{
			radixVolumeMount:   v1.RadixVolumeMount{Type: v1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume1", Storage: "storageName1", Path: "TestPath1"},
			expectedVolumeName: "csi-az-blob-app-volume1-storageName1",
		},
		{
			radixVolumeMount:   v1.RadixVolumeMount{Type: v1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume2", Storage: "storageName2", Path: "TestPath2"},
			expectedVolumeName: "csi-az-blob-app-volume2-storageName2",
		},
		{
			radixVolumeMount:           v1.RadixVolumeMount{Type: v1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume-with-long-name", Storage: "storageName-with-long-name", Path: "TestPath2"},
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
			radixVolumeMount: v1.RadixVolumeMount{Type: v1.MountTypeBlobFuse2FuseCsiAzure, Storage: "storageName1", Path: "TestPath1"},
			expectedError:    "name is empty for volume mount in the component app",
		},
		{
			name:             "Missed volume mount storage",
			radixVolumeMount: v1.RadixVolumeMount{Type: v1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume1", Path: "TestPath1"},
			expectedError:    "storage is empty for volume mount volume1 in the component app",
		},
		{
			name:             "Missed volume mount path",
			radixVolumeMount: v1.RadixVolumeMount{Type: v1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume1", Storage: "storageName1"},
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
			radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume1", Storage: "storage1", Path: "path1", GID: "1000"},
			expectedVolumeName:    "csi-az-blob-some-component-volume1-storage1",
			expectedPvcNamePrefix: "pvc-csi-az-blob-some-component-volume1-storage1",
		},
		{
			name:                       "Blob CSI Azure volume",
			radixVolumeMount:           v1.RadixVolumeMount{Type: v1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume-with-long-name", Storage: "storageName-with-long-name", Path: "path1", GID: "1000"},
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
		mounts := []v1.RadixVolumeMount{
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
	namespace := "some-namespace"
	componentName := "some-component"
	scenarios := []pvcTestScenario{
		{
			volumeMountTestScenario: volumeMountTestScenario{
				name:                  "Blob CSI Azure volume, PVS phase: Bound",
				radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume1", Storage: "storage1", Path: "path1", GID: "1000"},
				expectedVolumeName:    "csi-az-blob-some-component-volume1-storage1",
				expectedPvcNamePrefix: "existing-blob-pvc-name1",
			},
			pvc: createPvc(namespace, componentName, v1.MountTypeBlobFuse2FuseCsiAzure, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Name = "existing-blob-pvc-name1"
				pvc.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = "volume1"
				pvc.Status.Phase = corev1.ClaimBound
			}),
		},
		{
			volumeMountTestScenario: volumeMountTestScenario{
				name:                  "Blob CSI Azure volume, PVS phase: Pending",
				radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume2", Storage: "storage2", Path: "path2", GID: "1000"},
				expectedVolumeName:    "csi-az-blob-some-component-volume2-storage2",
				expectedPvcNamePrefix: "existing-blob-pvc-name2",
			},
			pvc: createPvc(namespace, componentName, v1.MountTypeBlobFuse2FuseCsiAzure, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Name = "existing-blob-pvc-name2"
				pvc.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = "volume2"
				pvc.Status.Phase = corev1.ClaimPending
			}),
		},
	}

	suite.T().Run("CSI Azure volumes with existing PVC", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		for _, scenario := range scenarios {
			t.Logf("Scenario %s for volume mount type %s, PVC status phase '%v'", scenario.name, string(GetCsiAzureVolumeMountType(&scenario.radixVolumeMount)), scenario.pvc.Status.Phase)
			_, _ = testEnv.kubeclient.CoreV1().PersistentVolumeClaims(namespace).Create(context.Background(), &scenario.pvc, metav1.CreateOptions{})

			component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts(scenario.radixVolumeMount).BuildComponent()
			volumes, err := GetVolumes(context.Background(), testEnv.kubeUtil, namespace, &component, "", nil)
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
			t.Logf("Scenario %s for volume mount type %s, PVC status phase '%v'", scenario.name, string(GetCsiAzureVolumeMountType(&scenario.radixVolumeMount)), scenario.pvc.Status.Phase)

			component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts(scenario.radixVolumeMount).BuildComponent()
			volumes, err := GetVolumes(context.Background(), testEnv.kubeUtil, namespace, &component, "", nil)
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
				radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeBlobFuse2FuseCsiAzure, Name: "blob-volume1", Storage: "storage1", Path: "path1", GID: "1000"},
				expectedVolumeName:    "csi-az-blob-some-component-blob-volume1-storage1",
				expectedPvcNamePrefix: "pvc-csi-az-blob-some-component-blob-volume1-storage1",
			},
			pvc: createPvc(namespace, componentName, v1.MountTypeBlobFuse2FuseCsiAzure, func(pvc *corev1.PersistentVolumeClaim) { pvc.Status.Phase = corev1.ClaimBound }),
		},
		{
			volumeMountTestScenario: volumeMountTestScenario{
				name:                  "Blob CSI Azure volume, Status phase: Pending",
				radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeBlobFuse2FuseCsiAzure, Name: "blob-volume2", Storage: "storage2", Path: "path2", GID: "1000"},
				expectedVolumeName:    "csi-az-blob-some-component-blob-volume2-storage2",
				expectedPvcNamePrefix: "pvc-csi-az-blob-some-component-blob-volume2-storage2",
			},
			pvc: createPvc(namespace, componentName, v1.MountTypeBlobFuse2FuseCsiAzure, func(pvc *corev1.PersistentVolumeClaim) { pvc.Status.Phase = corev1.ClaimPending }),
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

			volumes, err := GetVolumes(context.Background(), deployment.kubeutil, deployment.getNamespace(), &deployComponent, deployment.radixDeployment.GetName(), nil)

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

				volumes, err := GetVolumes(context.Background(), deployment.kubeutil, deployment.getNamespace(), &deployComponent, deployment.radixDeployment.GetName(), nil)

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
	radixVolumeMountType    v1.MountType
	requestsVolumeMountSize string
	volumeAccessMode        corev1.PersistentVolumeAccessMode
	volumeName              string
	pvProvisioner           string
	pvSecretName            string
	pvGid                   string
	pvUid                   string
	namespace               string
}

func (suite *VolumeMountTestSuite) Test_GetRadixDeployComponentVolumeMounts() {
	appName := "any-app"
	environment := "some-env"
	componentName := "some-component"
	scenarios := []volumeMountTestScenario{
		{
			name:                  "Blob CSI Azure volume, Status phase: Bound",
			radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeBlobFuse2FuseCsiAzure, Name: "blob-volume1", Storage: "storage1", Path: "path1", GID: "1000"},
			expectedVolumeName:    "csi-az-blob-some-component-blob-volume1-storage1",
			expectedPvcNamePrefix: "pvc-csi-az-blob-some-component-blob-volume1-storage1",
		},
		{
			name:                  "Blob CSI Azure volume, Status phase: Pending",
			radixVolumeMount:      v1.RadixVolumeMount{Type: v1.MountTypeBlobFuse2FuseCsiAzure, Name: "blob-volume2", Storage: "storage2", Path: "path2", GID: "1000"},
			expectedVolumeName:    "csi-az-blob-some-component-blob-volume2-storage2",
			expectedPvcNamePrefix: "pvc-csi-az-blob-some-component-blob-volume2-storage2",
		},
		{
			name:                       "Blob CSI Azure volume, Status phase: Pending",
			radixVolumeMount:           v1.RadixVolumeMount{Type: v1.MountTypeBlobFuse2FuseCsiAzure, Name: "blob-volume-with-long-name", Storage: "storage-with-long-name", Path: "path2", GID: "1000"},
			expectedVolumeName:         "csi-az-blob-some-component-blob-volume-with-long-name-sto-",
			expectedVolumeNameIsPrefix: true,
			expectedPvcNamePrefix:      "pvc-csi-az-blob-some-component-blob-volume-with-long-name-",
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
	const (
		appName       = "any-app"
		environment   = "some-env"
		componentName = "some-component"
	)
	// anotherNamespace := commonUtils.RandString(10)
	// anotherComponentName := commonUtils.RandString(10)

	var scenarios []deploymentVolumesTestScenario
	// scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
	// 	getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
	// 		return deploymentVolumesTestScenario{
	// 			name:  "Create new volume",
	// 			props: props,
	// 			radixVolumeMounts: []v1.RadixVolumeMount{
	// 				createRadixVolumeMount(props, func(vm *v1.RadixVolumeMount) {}),
	// 			},
	// 			volumes: []corev1.Volume{
	// 				createTestVolume(props, func(v *corev1.Volume) {}),
	// 			},
	// 			existingPvcsBeforeTestRun: []corev1.PersistentVolumeClaim{},
	// 			existingPvcsAfterTestRun: []corev1.PersistentVolumeClaim{
	// 				createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
	// 			},
	// 			existingPVsBeforeTestRun: []corev1.PersistentVolume{},
	// 			existingPVsAfterTestRun: []corev1.PersistentVolume{
	// 				createExpectedPv(props, func(pv *corev1.PersistentVolume) {}),
	// 			},
	// 		}
	// 	}
	// 	return []deploymentVolumesTestScenario{
	// 		getScenario(getPropsCsiBlobVolume1Storage1(nil)),
	// 	}
	// }()...)
	// scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
	// 	type scenarioProperties struct {
	// 		changedNewRadixVolumeName        string
	// 		changedNewRadixVolumeStorageName string
	// 		expectedVolumeName               string
	// 		expectedNewSecretName            string
	// 		expectedNewPvcName               string
	// 		expectedNewPvName                string
	// 	}
	// 	getScenario := func(props expectedPvcPvProperties, scenarioProps scenarioProperties) deploymentVolumesTestScenario {
	// 		return deploymentVolumesTestScenario{
	// 			name:  "Update storage in existing volume name and storage",
	// 			props: props,
	// 			radixVolumeMounts: []v1.RadixVolumeMount{
	// 				createRadixVolumeMount(props, func(vm *v1.RadixVolumeMount) {
	// 					vm.Name = scenarioProps.changedNewRadixVolumeName
	// 					vm.Storage = scenarioProps.changedNewRadixVolumeStorageName
	// 				}),
	// 			},
	// 			volumes: []corev1.Volume{
	// 				createTestVolume(props, func(v *corev1.Volume) {
	// 					v.Name = scenarioProps.expectedVolumeName
	// 				}),
	// 			},
	// 			existingPvcsBeforeTestRun: []corev1.PersistentVolumeClaim{
	// 				createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
	// 			},
	// 			existingPvcsAfterTestRun: []corev1.PersistentVolumeClaim{
	// 				createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {
	// 					pvc.ObjectMeta.Name = scenarioProps.expectedNewPvcName
	// 					pvc.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = scenarioProps.changedNewRadixVolumeName
	// 					pvc.Spec.VolumeName = scenarioProps.expectedNewPvName
	// 				}),
	// 			},
	// 			existingPVsBeforeTestRun: []corev1.PersistentVolume{
	// 				createExpectedPv(props, func(pv *corev1.PersistentVolume) {}),
	// 			},
	// 			existingPVsAfterTestRun: []corev1.PersistentVolume{
	// 				createExpectedPv(props, func(pv *corev1.PersistentVolume) {
	// 					pv.ObjectMeta.Name = scenarioProps.expectedNewPvName
	// 					pv.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = scenarioProps.changedNewRadixVolumeName
	// 					pv.ObjectMeta.Annotations[persistentvolume.CsiAnnotationProvisionerDeletionSecretName] = scenarioProps.expectedNewSecretName
	// 					setVolumeMountAttribute(pv, props.radixVolumeMountType, scenarioProps.changedNewRadixVolumeStorageName, scenarioProps.expectedNewPvcName)
	// 					pv.Spec.CSI.NodeStageSecretRef.Name = scenarioProps.expectedNewSecretName
	// 				}),
	// 			},
	// 		}
	// 	}
	// 	return []deploymentVolumesTestScenario{
	// 		getScenario(getPropsCsiBlobVolume1Storage1(nil), scenarioProperties{
	// 			changedNewRadixVolumeName:        "volume101",
	// 			changedNewRadixVolumeStorageName: "storage101",
	// 			expectedVolumeName:               "csi-az-blob-some-component-volume101-storage101",
	// 			expectedNewSecretName:            "some-component-volume101-csiazurecreds",
	// 			expectedNewPvcName:               "pvc-csi-az-blob-some-component-volume101-storage101-12345",
	// 			expectedNewPvName:                "pv-radixvolumemount-some-uuid",
	// 		}),
	// 	}
	// }()...)
	// scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
	// 	getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
	// 		pvForAnotherNamespace := createRandomPv(props, anotherNamespace, anotherComponentName)
	// 		pvForAnotherComponent := createRandomPv(props, props.namespace, anotherComponentName)
	// 		pvcForAnotherNamespace := createRandomPvc(props, anotherNamespace, anotherComponentName)
	// 		pvcForAnotherComponent := createRandomPvc(props, props.namespace, anotherComponentName)
	// 		matchPvAndPvc(pvForAnotherNamespace, pvcForAnotherNamespace)
	// 		matchPvAndPvc(pvForAnotherComponent, pvcForAnotherNamespace)
	// 		return deploymentVolumesTestScenario{
	// 			name:  "Garbage collect orphaned PVCs and PersistentVolume",
	// 			props: props,
	// 			radixVolumeMounts: []v1.RadixVolumeMount{
	// 				createRadixVolumeMount(props, func(vm *v1.RadixVolumeMount) {}),
	// 			},
	// 			volumes: []corev1.Volume{
	// 				createTestVolume(props, func(v *corev1.Volume) {}),
	// 			},
	// 			existingPvcsBeforeTestRun: []corev1.PersistentVolumeClaim{
	// 				createRandomPvc(props, props.namespace, props.componentName),
	// 				pvcForAnotherNamespace,
	// 				pvcForAnotherComponent,
	// 			},
	// 			existingPvcsAfterTestRun: []corev1.PersistentVolumeClaim{
	// 				createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
	// 				pvcForAnotherNamespace,
	// 				pvcForAnotherComponent,
	// 			},
	// 			existingPVsBeforeTestRun: []corev1.PersistentVolume{
	// 				createRandomPv(props, props.namespace, props.componentName),
	// 				pvForAnotherNamespace,
	// 				pvForAnotherComponent,
	// 			},
	// 			existingPVsAfterTestRun: []corev1.PersistentVolume{
	// 				createExpectedPv(props, func(pv *corev1.PersistentVolume) {}),
	// 				pvForAnotherNamespace,
	// 				pvForAnotherComponent,
	// 			},
	// 		}
	// 	}
	// 	return []deploymentVolumesTestScenario{
	// 		getScenario(getPropsCsiBlobVolume1Storage1(nil)),
	// 	}
	// }()...)
	// scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
	// 	getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
	// 		return deploymentVolumesTestScenario{
	// 			name:  "Set readonly volume",
	// 			props: props,
	// 			radixVolumeMounts: []v1.RadixVolumeMount{
	// 				createRadixVolumeMount(props, func(vm *v1.RadixVolumeMount) { vm.AccessMode = string(corev1.ReadOnlyMany) }),
	// 			},
	// 			volumes: []corev1.Volume{
	// 				createTestVolume(props, func(v *corev1.Volume) {}),
	// 			},
	// 			existingPvcsBeforeTestRun: []corev1.PersistentVolumeClaim{
	// 				createRandomPvc(props, props.namespace, props.componentName),
	// 			},
	// 			existingPvcsAfterTestRun: []corev1.PersistentVolumeClaim{
	// 				createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {
	// 					pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
	// 				}),
	// 			},
	// 			existingPVsBeforeTestRun: []corev1.PersistentVolume{
	// 				createRandomPv(props, props.namespace, props.componentName),
	// 			},
	// 			existingPVsAfterTestRun: []corev1.PersistentVolume{
	// 				createExpectedPv(props, func(pv *corev1.PersistentVolume) {
	// 					pv.Spec.MountOptions = append(pv.Spec.MountOptions, "-o ro")
	// 				}),
	// 			},
	// 		}
	// 	}
	// 	return []deploymentVolumesTestScenario{
	// 		getScenario(getPropsCsiBlobVolume1Storage1(nil)),
	// 	}
	// }()...)
	// scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
	// 	getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
	// 		existingPvc := createExpectedPvc(props, nil)
	// 		existingPv := createExpectedPv(props, nil)
	// 		matchPvAndPvc(&existingPv, &existingPvc)
	// 		return deploymentVolumesTestScenario{
	// 			name:  "Set ReadWriteOnce volume",
	// 			props: props,
	// 			radixVolumeMounts: []v1.RadixVolumeMount{
	// 				createRadixVolumeMount(props, func(vm *v1.RadixVolumeMount) { vm.AccessMode = string(corev1.ReadWriteOnce) }),
	// 			},
	// 			volumes: []corev1.Volume{
	// 				createTestVolume(props, func(v *corev1.Volume) {}),
	// 			},
	// 			existingPvcsBeforeTestRun: []corev1.PersistentVolumeClaim{
	// 				existingPvc,
	// 			},
	// 			existingPvcsAfterTestRun: []corev1.PersistentVolumeClaim{
	// 				modifyPvc(existingPvc, func(pvc *corev1.PersistentVolumeClaim) {
	// 					pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	// 				}),
	// 			},
	// 			existingPVsBeforeTestRun: []corev1.PersistentVolume{
	// 				existingPv,
	// 			},
	// 			existingPVsAfterTestRun: []corev1.PersistentVolume{
	// 				modifyPv(existingPv, func(pv *corev1.PersistentVolume) {
	// 					pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	// 					pv.Spec.MountOptions = getMountOptions(props, false)
	// 				}),
	// 			},
	// 		}
	// 	}
	// 	return []deploymentVolumesTestScenario{
	// 		getScenario(getPropsCsiBlobVolume1Storage1(nil)),
	// 	}
	// }()...)
	// scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
	// 	getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
	// 		existingPvc := createExpectedPvc(props, nil)
	// 		existingPv := createExpectedPv(props, nil)
	// 		matchPvAndPvc(&existingPv, &existingPvc)
	// 		return deploymentVolumesTestScenario{
	// 			name:  "Set ReadWriteMany volume",
	// 			props: props,
	// 			radixVolumeMounts: []v1.RadixVolumeMount{
	// 				createRadixVolumeMount(props, func(vm *v1.RadixVolumeMount) { vm.AccessMode = string(corev1.ReadWriteMany) }),
	// 			},
	// 			volumes: []corev1.Volume{
	// 				createTestVolume(props, func(v *corev1.Volume) {}),
	// 			},
	// 			existingPvcsBeforeTestRun: []corev1.PersistentVolumeClaim{
	// 				existingPvc,
	// 			},
	// 			existingPvcsAfterTestRun: []corev1.PersistentVolumeClaim{
	// 				modifyPvc(existingPvc, func(pvc *corev1.PersistentVolumeClaim) {
	// 					pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
	// 				}),
	// 			},
	// 			existingPVsBeforeTestRun: []corev1.PersistentVolume{
	// 				existingPv,
	// 			},
	// 			existingPVsAfterTestRun: []corev1.PersistentVolume{
	// 				modifyPv(existingPv, func(pv *corev1.PersistentVolume) {
	// 					pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
	// 					pv.Spec.MountOptions = getMountOptions(props, false)
	// 				}),
	// 			},
	// 		}
	// 	}
	// 	return []deploymentVolumesTestScenario{
	// 		getScenario(getPropsCsiBlobVolume1Storage1(nil)),
	// 	}
	// }()...)
	// scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
	// 	getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
	// 		existingPvc := createExpectedPvc(props, nil)
	// 		existingPv := createExpectedPv(props, nil)
	// 		matchPvAndPvc(&existingPv, &existingPvc)
	// 		return deploymentVolumesTestScenario{
	// 			name:  "Set ReadOnlyMany volume",
	// 			props: props,
	// 			radixVolumeMounts: []v1.RadixVolumeMount{
	// 				createRadixVolumeMount(props, func(vm *v1.RadixVolumeMount) { vm.AccessMode = string(corev1.ReadOnlyMany) }),
	// 			},
	// 			volumes: []corev1.Volume{
	// 				createTestVolume(props, func(v *corev1.Volume) {}),
	// 			},
	// 			existingPvcsBeforeTestRun: []corev1.PersistentVolumeClaim{
	// 				modifyPvc(existingPvc, func(pvc *corev1.PersistentVolumeClaim) {
	// 					pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
	// 				}),
	// 			},
	// 			existingPvcsAfterTestRun: []corev1.PersistentVolumeClaim{
	// 				modifyPvc(existingPvc, func(pvc *corev1.PersistentVolumeClaim) {
	// 					pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
	// 				}),
	// 			},
	// 			existingPVsBeforeTestRun: []corev1.PersistentVolume{
	// 				modifyPv(existingPv, func(pv *corev1.PersistentVolume) {
	// 					pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
	// 					pv.Spec.MountOptions = getMountOptions(props, false)
	// 				}),
	// 			},
	// 			existingPVsAfterTestRun: []corev1.PersistentVolume{
	// 				modifyPv(existingPv, func(pv *corev1.PersistentVolume) {
	// 					pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
	// 					pv.Spec.MountOptions = getMountOptions(props, true)
	// 				}),
	// 			},
	// 		}
	// 	}
	// 	return []deploymentVolumesTestScenario{
	// 		getScenario(getPropsCsiBlobVolume1Storage1(nil)),
	// 	}
	// }()...)
	// scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
	// 	getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
	// 		return deploymentVolumesTestScenario{
	// 			name:  "Create new BlobFuse2 volume has streaming by default and streaming options not set",
	// 			props: props,
	// 			radixVolumeMounts: []v1.RadixVolumeMount{
	// 				createBlobFuse2RadixVolumeMount(props, func(vm *v1.RadixVolumeMount) {}),
	// 			},
	// 			volumes: []corev1.Volume{
	// 				createTestVolume(props, func(v *corev1.Volume) {}),
	// 			},
	// 			existingPvcsBeforeTestRun: []corev1.PersistentVolumeClaim{},
	// 			existingPvcsAfterTestRun: []corev1.PersistentVolumeClaim{
	// 				createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
	// 			},
	// 			existingPVsBeforeTestRun: []corev1.PersistentVolume{},
	// 			existingPVsAfterTestRun: []corev1.PersistentVolume{
	// 				createExpectedPv(props, func(pv *corev1.PersistentVolume) {
	// 					pv.Spec.MountOptions = getMountOptions(props, true, "--streaming=true", "--use-adls=false")
	// 				}),
	// 			},
	// 		}
	// 	}
	// 	return []deploymentVolumesTestScenario{
	// 		getScenario(getPropsCsiBlobFuse2Volume1Storage1(nil)),
	// 	}
	// }()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			return deploymentVolumesTestScenario{
				name:  "Create new BlobFuse2 volume has implicit streaming by default and streaming options set",
				props: props,
				radixVolumeMounts: []v1.RadixVolumeMount{
					createBlobFuse2RadixVolumeMount(props, func(vm *v1.RadixVolumeMount) {
						vm.BlobFuse2.Streaming = &v1.RadixVolumeMountStreaming{
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
				existingPvcsBeforeTestRun: []corev1.PersistentVolumeClaim{},
				existingPvcsAfterTestRun: []corev1.PersistentVolumeClaim{
					createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
				},
				existingPVsBeforeTestRun: []corev1.PersistentVolume{},
				existingPVsAfterTestRun: []corev1.PersistentVolume{
					createExpectedPv(props, func(pv *corev1.PersistentVolume) {
						pv.Spec.MountOptions = getMountOptions(props, true,
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
	//
	// scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
	// 	getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
	// 		return deploymentVolumesTestScenario{
	// 			name:  "Create new BlobFuse2 volume has disabled streaming",
	// 			props: props,
	// 			radixVolumeMounts: []v1.RadixVolumeMount{
	// 				createBlobFuse2RadixVolumeMount(props, func(vm *v1.RadixVolumeMount) {
	// 					vm.BlobFuse2.Streaming = &v1.RadixVolumeMountStreaming{
	// 						Enabled:          pointers.Ptr(false),
	// 						StreamCache:      pointers.Ptr(uint64(101)),
	// 						BlockSize:        pointers.Ptr(uint64(102)),
	// 						BufferSize:       pointers.Ptr(uint64(103)),
	// 						MaxBuffers:       pointers.Ptr(uint64(104)),
	// 						MaxBlocksPerFile: pointers.Ptr(uint64(105)),
	// 					}
	// 				}),
	// 			},
	// 			volumes: []corev1.Volume{
	// 				createTestVolume(props, func(v *corev1.Volume) {}),
	// 			},
	// 			existingPvcsBeforeTestRun: []corev1.PersistentVolumeClaim{},
	// 			existingPvcsAfterTestRun: []corev1.PersistentVolumeClaim{
	// 				createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
	// 			},
	// 			existingPVsBeforeTestRun: []corev1.PersistentVolume{},
	// 			existingPVsAfterTestRun: []corev1.PersistentVolume{
	// 				createExpectedPv(props, func(pv *corev1.PersistentVolume) {
	// 					pv.Spec.MountOptions = []string{
	// 						"--file-cache-timeout-in-seconds=120",
	// 						"--use-attr-cache=true",
	// 						"--cancel-list-on-mount-seconds=0",
	// 						"-o allow_other",
	// 						"-o attr_timeout=120",
	// 						"-o entry_timeout=120",
	// 						"-o negative_timeout=120",
	// 						"-o gid=1000",
	// 						"--use-adls=false",
	// 					}
	// 				}),
	// 			},
	// 		}
	// 	}
	// 	return []deploymentVolumesTestScenario{
	// 		getScenario(getPropsCsiBlobFuse2Volume1Storage1(nil)),
	// 	}
	// }()...)

	suite.T().Run("CSI Azure volume PVCs and PersistentVolume", func(t *testing.T) {
		// t.Parallel()
		for _, factory := range suite.radixCommonDeployComponentFactories[:1] {
			for _, scenario := range scenarios {
				t.Logf("Test case %s, volume type %s, component %s", scenario.name, scenario.props.radixVolumeMountType, factory.GetTargetType())
				testEnv := getTestEnv()
				deployment := getDeployment(testEnv)
				deployment.radixDeployment = buildRd(appName, environment, componentName, scenario.radixVolumeMounts)
				putExistingDeploymentVolumesScenarioDataToFakeCluster(&scenario, deployment)
				desiredDeployment := getDesiredDeployment(componentName, scenario.volumes)

				// action
				deployComponent := deployment.radixDeployment.Spec.Components[0]
				err := deployment.createOrUpdateCsiAzureVolumeResources(context.Background(), &deployComponent, desiredDeployment)
				assert.Nil(t, err)

				existingPvcs, existingPvs, err := getExistingPvcsAndPersistentVolumeFromFakeCluster(deployment)
				assert.Nil(t, err)
				assert.True(t, equalPersistentVolumeClaims(&scenario.existingPvcsAfterTestRun, &existingPvcs), "PVC-s are not equal")
				assert.True(t, equalPersistentVolumes(&scenario.existingPVsAfterTestRun, &existingPvs), "PV-s are not equal")
			}
		}
	})
}

func matchPvAndPvc(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
	pv.Spec.CSI.VolumeAttributes[persistentvolume.CsiVolumeMountAttributePvcName] = pvc.Name
	pvc.Spec.VolumeName = pv.Name
}

func (suite *VolumeMountTestSuite) Test_CreateOrUpdateCsiAzureKeyVaultResources() {
	appName := "app"
	namespace := "some-namespace"
	environment := "some-env"
	componentName1, componentNameLong := "component1", "a-very-long-component-name-that-exceeds-63-kubernetes-volume-name-limit"
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
		{
			name:          "Volume name should be trimmed when exceeding 63 chars",
			componentName: componentNameLong,
			azureKeyVaults: []v1.RadixAzureKeyVault{{
				Name:  "kv1",
				Items: []v1.RadixAzureKeyVaultItem{{Name: "secret1", EnvVar: "SECRET_REF1"}},
			}},
			expectedVolumeProps: []expectedVolumeProps{
				{
					expectedVolumeNamePrefix:         "a-very-long-component-name-that-exceeds-63-kubernetes-vol",
					expectedVolumeMountPath:          "/mnt/azure-key-vault/kv1",
					expectedNodePublishSecretRefName: "a-very-long-component-name-that-exceeds-63-kubernetes-volume-name-limit-kv1-csiazkvcreds",
					expectedVolumeAttributePrefixes: map[string]string{
						"secretProviderClass": "a-very-long-component-name-that-exceeds-63-kubernetes-volume-name-limit-az-keyvault-kv1-",
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
				spc, err := deployment.createAzureKeyVaultSecretProviderClassForRadixDeployment(context.Background(), namespace, appName, radixDeployComponent.GetName(), azureKeyVault)
				if err != nil {
					t.Log(err.Error())
				} else {
					t.Logf("created secret provider class %s", spc.Name)
				}
			}
			volumes, err := GetVolumes(context.Background(), testEnv.kubeUtil, namespace, radixDeployComponent, deployment.radixDeployment.GetName(), nil)
			assert.Nil(t, err)
			assert.Len(t, volumes, len(scenario.expectedVolumeProps))
			if len(scenario.expectedVolumeProps) == 0 {
				continue
			}

			for i := 0; i < len(volumes); i++ {
				volume := volumes[i]
				assert.Less(t, len(volume.Name), 64, "volume name is too long")
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
				spc, err := deployment.createAzureKeyVaultSecretProviderClassForRadixDeployment(context.Background(), namespace, appName, radixDeployComponent.GetName(), azureKeyVault)
				if err != nil {
					t.Log(err.Error())
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
				assert.Less(t, len(volumeMount.Name), 64, "volumemount name is too long")
				assert.True(t, strings.Contains(volumeMount.Name, volumeProp.expectedVolumeNamePrefix))
				assert.Equal(t, volumeProp.expectedVolumeMountPath, volumeMount.MountPath)
				assert.True(t, volumeMount.ReadOnly)
			}
		}
	})
}

func Test_EmptyDir(t *testing.T) {
	appName, envName, compName := "anyapp", "anyenv", "anycomp"

	tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	builder := utils.NewDeploymentBuilder().
		WithRadixApplication(utils.NewRadixApplicationBuilder().WithAppName(appName).WithRadixRegistration(utils.NewRegistrationBuilder().WithName(appName))).
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponents(
			utils.NewDeployComponentBuilder().WithName(compName).WithVolumeMounts(
				v1.RadixVolumeMount{Name: "cache", Path: "/cache", EmptyDir: &v1.RadixEmptyDirVolumeMount{SizeLimit: resource.MustParse("50M")}},
				v1.RadixVolumeMount{Name: "log", Path: "/log", EmptyDir: &v1.RadixEmptyDirVolumeMount{SizeLimit: resource.MustParse("100M")}},
			),
		)

	rd, err := ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, builder)
	require.NoError(t, err)
	assert.NotNil(t, rd)

	deployment, err := kubeclient.AppsV1().Deployments(utils.GetEnvironmentNamespace(appName, envName)).Get(context.Background(), compName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Len(t, deployment.Spec.Template.Spec.Containers[0].VolumeMounts, 2)
	assert.Len(t, deployment.Spec.Template.Spec.Volumes, 2)
}

func createRandomPv(props expectedPvcPvProperties, namespace, componentName string) corev1.PersistentVolume {
	return createExpectedPv(props, func(pv *corev1.PersistentVolume) {
		pvName := getCsiAzurePersistentVolumeName()
		pv.ObjectMeta.Name = pvName
		pv.ObjectMeta.Labels[kube.RadixNamespace] = namespace
		pv.ObjectMeta.Labels[kube.RadixComponentLabel] = componentName
		pv.Spec.CSI.VolumeAttributes[persistentvolume.CsiVolumeMountAttributePvName] = pvName
	})
}

func createRandomPvc(props expectedPvcPvProperties, namespace, componentName string) corev1.PersistentVolumeClaim {
	return createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {
		pvcName, err := getCsiAzurePersistentVolumeClaimName(componentName, &v1.RadixVolumeMount{Name: props.radixVolumeMountName, Type: v1.MountTypeBlobFuse2FuseCsiAzure, Storage: props.radixStorageName, Path: "/tmp"})
		if err != nil {
			panic(err)
		}
		pvName := getCsiAzurePersistentVolumeName()
		pvc.ObjectMeta.Name = pvcName
		pvc.ObjectMeta.Namespace = namespace
		pvc.ObjectMeta.Labels[kube.RadixComponentLabel] = componentName
		pvc.Spec.VolumeName = pvName
	})
}

func getPropsCsiBlobVolume1Storage1(modify func(*expectedPvcPvProperties)) expectedPvcPvProperties {
	appName := "any-app"
	environment := "some-env"
	componentName := "some-component"
	props := expectedPvcPvProperties{
		appName:                 appName,
		environment:             environment,
		namespace:               fmt.Sprintf("%s-%s", appName, environment),
		componentName:           componentName,
		radixVolumeMountName:    "volume1",
		radixStorageName:        "storage1",
		pvcName:                 "pvc-csi-az-blob-some-component-volume1-storage1-12345",
		persistentVolumeName:    "pv-radixvolumemount-some-uuid",
		radixVolumeMountType:    v1.MountTypeBlobFuse2FuseCsiAzure,
		requestsVolumeMountSize: "1Mi",
		volumeAccessMode:        corev1.ReadOnlyMany, // default access mode
		volumeName:              "csi-az-blob-some-component-volume1-storage1",
		pvProvisioner:           provisionerBlobCsiAzure,
		pvSecretName:            "some-component-volume1-csiazurecreds",
		pvGid:                   "1000",
		pvUid:                   "",
	}
	if modify != nil {
		modify(&props)
	}
	return props
}

func getPropsCsiBlobFuse2Volume1Storage1(modify func(*expectedPvcPvProperties)) expectedPvcPvProperties {
	appName := "any-app"
	environment := "some-env"
	componentName := "some-component"
	props := expectedPvcPvProperties{
		appName:                 appName,
		environment:             environment,
		namespace:               fmt.Sprintf("%s-%s", appName, environment),
		componentName:           componentName,
		radixVolumeMountName:    "volume1",
		radixStorageName:        "storage1",
		pvcName:                 "pvc-csi-blobfuse2-fuse2-some-component-volume1-storage1-12345",
		persistentVolumeName:    "pv-radixvolumemount-some-uuid",
		radixVolumeMountType:    v1.MountTypeBlobFuse2Fuse2CsiAzure,
		requestsVolumeMountSize: "1Mi",
		volumeAccessMode:        corev1.ReadOnlyMany, // default access mode
		volumeName:              "csi-blobfuse2-fuse2-some-component-volume1-storage1",
		pvProvisioner:           provisionerBlobCsiAzure,
		pvSecretName:            "some-component-volume1-csiazurecreds",
		pvGid:                   "1000",
		pvUid:                   "",
	}
	if modify != nil {
		modify(&props)
	}
	return props
}

func putExistingDeploymentVolumesScenarioDataToFakeCluster(scenario *deploymentVolumesTestScenario, deployment *Deployment) {
	for _, pvc := range scenario.existingPvcsBeforeTestRun {
		_, _ = deployment.kubeclient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(context.Background(), &pvc, metav1.CreateOptions{})
	}
	for _, pv := range scenario.existingPVsBeforeTestRun {
		_, _ = deployment.kubeclient.CoreV1().PersistentVolumes().Create(context.Background(), &pv, metav1.CreateOptions{})
	}
}

func getExistingPvcsAndPersistentVolumeFromFakeCluster(deployment *Deployment) ([]corev1.PersistentVolumeClaim, []corev1.PersistentVolume, error) {
	var pvcItems []corev1.PersistentVolumeClaim
	var pvItems []corev1.PersistentVolume
	pvcList, err := deployment.kubeclient.CoreV1().PersistentVolumeClaims("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	if pvcList != nil && pvcList.Items != nil {
		pvcItems = pvcList.Items
	}
	pvList, err := deployment.kubeclient.CoreV1().PersistentVolumes().List(context.Background(), metav1.ListOptions{})
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
			Replicas: pointers.Ptr(DefaultReplicas),
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
			WithVolumeMounts(radixVolumeMounts...)).
		BuildRD()
}

func createPvc(namespace, componentName string, mountType v1.MountType, modify func(*corev1.PersistentVolumeClaim)) corev1.PersistentVolumeClaim {
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

func buildRdWithComponentBuilders(appName string, environment string, componentBuilders func() []utils.DeployComponentBuilder) *v1.RadixDeployment {
	return utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(environment).
		WithComponents(componentBuilders()...).
		BuildRD()
}

func createExpectedPv(props expectedPvcPvProperties, modify func(pv *corev1.PersistentVolume)) corev1.PersistentVolume {
	mountOptions := getMountOptions(props, true)
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
				Namespace:  props.namespace,
				Name:       props.pvcName,
				APIVersion: "v1",
				Kind:       "PersistentVolumeClaim",
			},
			StorageClassName:              "",
			MountOptions:                  mountOptions,
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
	}
	setVolumeMountAttribute(&pv, props.radixVolumeMountType, props.radixStorageName, props.pvcName)
	if modify != nil {
		modify(&pv)
	}
	return pv
}

func getMountOptions(props expectedPvcPvProperties, readOnly bool, extraOptions ...string) []string {
	options := []string{
		"--file-cache-timeout-in-seconds=120",
		"--use-attr-cache=true",
		"--cancel-list-on-mount-seconds=0",
		"-o allow_other",
		"-o attr_timeout=120",
		"-o entry_timeout=120",
		"-o negative_timeout=120",
	}
	if readOnly {
		options = append(options, "-o ro")
	}
	idOption := getPersistentVolumeIdMountOption(props)
	if len(idOption) > 0 {
		options = append(options, idOption)
	}
	return append(options, extraOptions...)
}

func setVolumeMountAttribute(pv *corev1.PersistentVolume, radixVolumeMountType v1.MountType, containerName, pvcName string) {
	pv.Spec.CSI.VolumeAttributes[persistentvolume.CsiVolumeMountAttributeContainerName] = containerName
	pv.Spec.CSI.VolumeAttributes[persistentvolume.CsiVolumeMountAttributePvcName] = pvcName
	switch radixVolumeMountType {
	case v1.MountTypeBlobFuse2FuseCsiAzure:
		pv.Spec.CSI.VolumeAttributes[persistentvolume.CsiVolumeMountAttributeProtocol] = persistentvolume.CsiVolumeAttributeProtocolParameterFuse
	case v1.MountTypeBlobFuse2Fuse2CsiAzure:
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
		},
		Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
	}
	if modify != nil {
		modify(&pvc)
	}
	return pvc
}

func createTestVolume(pvcProps expectedPvcPvProperties, modify func(*corev1.Volume)) corev1.Volume {
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

func createRadixVolumeMount(props expectedPvcPvProperties, modify func(mount *v1.RadixVolumeMount)) v1.RadixVolumeMount {
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
func createBlobFuse2RadixVolumeMount(props expectedPvcPvProperties, modify func(mount *v1.RadixVolumeMount)) v1.RadixVolumeMount {
	volumeMount := v1.RadixVolumeMount{
		Name: props.radixVolumeMountName,
		Path: "path1",
		BlobFuse2: &v1.RadixBlobFuse2VolumeMount{
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
			if persistentvolume.EqualPersistentVolumesForTest(&expectedPv, &actualPv) {
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

func equalPersistentVolumeClaims(pvcList1, pvcList2 *[]corev1.PersistentVolumeClaim) bool {
	if len(*pvcList1) != len(*pvcList2) {
		return false
	}
	for _, pvc1 := range *pvcList1 {
		var hasEqualPvc bool
		for _, pvc2 := range *pvcList2 {
			if equalNamePrefix(pvc1, pvc2) &&
				equalVolumeNamePrefix(pvc1, pvc2) &&
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

func equalVolumeNamePrefix(pvc1 corev1.PersistentVolumeClaim, pvc2 corev1.PersistentVolumeClaim) bool {
	return pvc1.Spec.VolumeName[:20] == pvc2.Spec.VolumeName[:20]
}

func equalNamePrefix(pvc1 corev1.PersistentVolumeClaim, pvc2 corev1.PersistentVolumeClaim) bool {
	return pvc1.GetName()[:len(pvc1.GetName())-5] == pvc2.GetName()[:len(pvc2.GetName())-5]
}

func getPvcCopyWithLabels(pvc *corev1.PersistentVolumeClaim, ignoreRandomPostfixInName bool) (*corev1.PersistentVolumeClaim, map[string]string) {
	pvcCopy := pvc.DeepCopy()
	pvcCopy.ObjectMeta.ManagedFields = nil // HACK: to avoid ManagedFields comparison
	if ignoreRandomPostfixInName {
		pvcCopy.ObjectMeta.Name = commonUtils.ShortenString(pvcCopy.ObjectMeta.Name, 6)
	}
	// to avoid label order variations
	labels := pvcCopy.ObjectMeta.Labels
	pvcCopy.ObjectMeta.Labels = map[string]string{}
	return pvcCopy, labels
}

func getPvcPatch(pvc1, pvc2 *corev1.PersistentVolumeClaim) ([]byte, error) {
	json1, err := json.Marshal(pvc1)
	if err != nil {
		return nil, err
	}
	json2, err := json.Marshal(pvc2)
	if err != nil {
		return nil, err
	}
	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(json1, json2, corev1.PersistentVolumeClaim{})
	if err != nil {
		return nil, err
	}
	return patchBytes, nil
}
