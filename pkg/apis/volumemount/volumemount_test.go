//nolint:staticcheck
package volumemount

import (
	"context"
	"fmt"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/internal"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type volumeMountTestSuite struct {
	testSuite
}

func TestVolumeMountTestSuite(t *testing.T) {
	suite.Run(t, new(volumeMountTestSuite))
}

func (s *volumeMountTestSuite) Test_NoVolumeMounts() {
	s.T().Run("app", func(t *testing.T) {
		t.Parallel()
		for _, factory := range s.radixCommonDeployComponentFactories {

			component := utils.NewDeployCommonComponentBuilder(factory).
				WithName("app").
				BuildComponent()

			volumeMounts, _ := GetRadixDeployComponentVolumeMounts(component, "")
			assert.Equal(t, 0, len(volumeMounts))
		}
	})
}

func (s *volumeMountTestSuite) Test_ValidBlobCsiAzureVolumeMounts() {
	scenarios := []volumeMountTestScenario{
		{
			radixVolumeMount:   radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume1", Storage: "storagename1", Path: "TestPath1"},
			expectedVolumeName: "csi-az-blob-app-volume1-storagename1",
		},
		{
			radixVolumeMount:   radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume2", Storage: "storagename2", Path: "TestPath2"},
			expectedVolumeName: "csi-az-blob-app-volume2-storagename2",
		},
		{
			radixVolumeMount:   radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume-with-long-name", Storage: "blobstoragename-with-long-name", Path: "TestPath2"},
			expectedVolumeName: "csi-az-blob-app-volume-with-long-name-blobstoragename-wit-12345",
		},
	}
	s.T().Run("One Blob CSI Azure volume mount ", func(t *testing.T) {
		t.Parallel()
		for _, factory := range s.radixCommonDeployComponentFactories {
			t.Logf("Test case %s for component %s", scenarios[0].name, factory.GetTargetType())
			component := utils.NewDeployCommonComponentBuilder(factory).WithName("app").
				WithVolumeMounts(scenarios[0].radixVolumeMount).
				BuildComponent()

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(component, "")
			assert.Nil(t, err)
			assert.Equal(t, 1, len(volumeMounts), "Unexpected volume count")
			if len(volumeMounts) > 0 {
				mount := volumeMounts[0]
				assert.Less(t, len(volumeMounts[0].Name), 64, "Volume name is too long")
				assert.Equal(t, scenarios[0].expectedVolumeName, mount.Name, "Mismatching volume names")
				assert.Equal(t, scenarios[0].radixVolumeMount.Path, mount.MountPath, "Mismatching volume paths")
			}
		}
	})
	s.T().Run("Multiple Blob CSI Azure volume mount ", func(t *testing.T) {
		t.Parallel()
		for _, factory := range s.radixCommonDeployComponentFactories {
			t.Logf("Test case %s for component %s", scenarios[0].name, factory.GetTargetType())
			component := utils.NewDeployCommonComponentBuilder(factory).
				WithName("app").
				WithVolumeMounts(scenarios[0].radixVolumeMount, scenarios[1].radixVolumeMount, scenarios[2].radixVolumeMount).
				BuildComponent()

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(component, "")
			assert.Equal(t, 3, len(volumeMounts), "Unexpected volume count")
			assert.Nil(t, err)
			for idx, testCase := range scenarios {
				if len(volumeMounts) > 0 {
					volumeMountName := volumeMounts[idx].Name
					assert.Less(t, len(volumeMountName), 64)
					if len(volumeMountName) > 60 {
						assert.True(t, internal.EqualTillPostfix(testCase.expectedVolumeName, volumeMountName, nameRandPartLength), "Mismatching volume name prefixes %s and %s", volumeMountName, testCase.expectedVolumeName)
					} else {
						assert.Equal(t, testCase.expectedVolumeName, volumeMountName, "Mismatching volume names")
					}
					assert.Equal(t, testCase.radixVolumeMount.Path, volumeMounts[idx].MountPath, "Mismatching volume paths")
				}
			}
		}
	})
}

func (s *volumeMountTestSuite) Test_FailBlobCsiAzureVolumeMounts() {
	scenarios := []volumeMountTestScenario{
		{
			name:             "Missed volume mount name",
			radixVolumeMount: radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Storage: "storagename1", Path: "TestPath1"},
			expectedError:    "name is empty for volume mount in the component app",
		},
		{
			name:             "Missed volume mount storage",
			radixVolumeMount: radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume1", Path: "TestPath1"},
			expectedError:    "storage is empty for volume mount volume1 in the component app",
		},
		{
			name:             "Missed volume mount path",
			radixVolumeMount: radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume1", Storage: "storagename1"},
			expectedError:    "path is empty for volume mount volume1 in the component app",
		},
	}
	s.T().Run("Failing Blob CSI Azure volume mount", func(t *testing.T) {
		t.Parallel()
		for _, factory := range s.radixCommonDeployComponentFactories {
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

func (s *volumeMountTestSuite) Test_GetNewVolumes() {
	namespace := "some-namespace"
	componentName := "some-component"
	s.T().Run("No volumes in component", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts().BuildComponent()
		volumes, err := GetVolumes(context.Background(), testEnv.kubeUtil, namespace, &component, "", nil)
		assert.Nil(t, err)
		assert.Len(t, volumes, 0)
	})
	scenarios := []volumeMountTestScenario{
		{
			name:               "Blob CSI Azure volume",
			radixVolumeMount:   radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume1", Storage: "storage1", Path: "path1", GID: "1000"},
			expectedVolumeName: "csi-az-blob-some-component-volume1-storage1",
			expectedPvcName:    "pvc-csi-az-blob-some-component-volume1-storage1-12345",
		},
		{
			name:               "Blob CSI Azure volume with long names",
			radixVolumeMount:   radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "volume-with-long-name", Storage: "blobstoragesame-with-long-name", Path: "path1", GID: "1000"},
			expectedVolumeName: "csi-az-blob-some-component-volume-with-long-name-blobstor-12345",
			expectedPvcName:    "pvc-csi-az-blob-some-component-volume-with-long-name-blobstor-einhp-12345",
		},
	}
	s.T().Run("CSI Azure volumes", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		for _, scenario := range scenarios {
			t.Logf("Scenario %s", scenario.name)
			component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts(scenario.radixVolumeMount).BuildComponent()
			volumes, err := GetVolumes(context.Background(), testEnv.kubeUtil, namespace, &component, "", nil)
			assert.Nil(t, err)
			assert.Len(t, volumes, 1, "Unexpected volume count")
			volume := volumes[0]
			if len(volume.Name) > 60 {
				assert.True(t, internal.EqualTillPostfix(scenario.expectedVolumeName, volume.Name, nameRandPartLength), "Mismatching volume name prefixes %s and %s", scenario.expectedVolumeName, volume.Name)
			} else {
				assert.Equal(t, scenario.expectedVolumeName, volume.Name, "Mismatching volume names")
			}
			assert.Less(t, len(volume.Name), 64, "Volume name is too long")
			assert.NotNil(t, volume.PersistentVolumeClaim, "PVC is nil")
			assert.True(t, internal.EqualTillPostfix(scenario.expectedPvcName, volume.PersistentVolumeClaim.ClaimName, nameRandPartLength), "Mismatching PVC name prefixes %s and %s", scenario.expectedPvcName, volume.PersistentVolumeClaim.ClaimName)
		}
	})
	s.T().Run("Unsupported volume type", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		mounts := []radixv1.RadixVolumeMount{
			{Type: "unsupported-type", Name: "volume1", Container: "storage1", Path: "path1"},
		}
		component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts(mounts...).BuildComponent()
		volumes, err := GetVolumes(context.Background(), testEnv.kubeUtil, namespace, &component, "", nil)
		assert.Len(t, volumes, 0, "Unexpected volume count")
		assert.NotNil(t, err)
		assert.Equal(t, "unsupported volume type unsupported-type", err.Error())
	})
}

func (s *volumeMountTestSuite) Test_GetCsiVolumesWithExistingPvcs() {
	const (
		namespace     = "any-app-some-env"
		componentName = "some-component"
	)
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
		{
			volumeMountTestScenario: volumeMountTestScenario{
				name:               "Changed container name",
				radixVolumeMount:   radixv1.RadixVolumeMount{Name: "volume1", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Container: "storage2", GID: "1000"}, Path: "path1"},
				expectedVolumeName: "csi-blobfuse2-fuse2-some-component-volume1-storage2",
			},
			pvc: createExpectedPvc(modifyProps(props, func(props *expectedPvcPvProperties) {
				props.blobStorageName = "storage2"
				props.pvcName = "pvc-csi-blobfuse2-fuse2-some-component-volume1-storage2-12345"
			}), func(pvc *corev1.PersistentVolumeClaim) {}),
			pv: createExpectedPv(modifyProps(props, func(props *expectedPvcPvProperties) {
				props.blobStorageName = "storage2"
			}), func(pv *corev1.PersistentVolume) {}),
		},
	}

	s.T().Run("CSI Azure volumes with existing PVC", func(t *testing.T) {
		t.Parallel()
		for _, scenario := range scenarios {
			t.Logf("Scenario %s for volume mount type %s, PVC status phase '%v'", scenario.name, string(GetCsiAzureVolumeMountType(&scenario.radixVolumeMount)), scenario.pvc.Status.Phase)
			testEnv := getTestEnv()
			_, err := testEnv.kubeClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})
			require.NoError(t, err)
			_, err = testEnv.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Create(context.Background(), &scenario.pvc, metav1.CreateOptions{})
			require.NoError(t, err)
			_, err = testEnv.kubeClient.CoreV1().PersistentVolumes().Create(context.Background(), &scenario.pv, metav1.CreateOptions{})
			require.NoError(t, err)

			component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts(scenario.radixVolumeMount).BuildComponent()
			volumes, err := GetVolumes(context.Background(), testEnv.kubeUtil, namespace, &component, "", []corev1.Volume{{
				Name:         props.volumeName,
				VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: scenario.pvc.Name}},
			}})
			assert.Nil(t, err)
			assert.Len(t, volumes, 1)
			assert.Equal(t, scenario.expectedVolumeName, volumes[0].Name, "Mismatching volume names")
			assert.NotNil(t, volumes[0].PersistentVolumeClaim, "PVC is nil")
			assert.True(t, internal.EqualTillPostfix(scenario.pvc.Name, volumes[0].PersistentVolumeClaim.ClaimName, nameRandPartLength), "Mismatching PVC name prefixes %s and %s", scenario.pvc.Name, volumes[0].PersistentVolumeClaim.ClaimName)
		}
	})

	s.T().Run("CSI Azure volumes with no existing PVC", func(t *testing.T) {
		t.Parallel()
		for _, scenario := range scenarios {
			t.Logf("Scenario %s for volume mount type %s, PVC status phase '%v'", scenario.name, string(GetCsiAzureVolumeMountType(&scenario.radixVolumeMount)), scenario.pvc.Status.Phase)
			testEnv := getTestEnv()
			_, err := testEnv.kubeClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})
			require.NoError(t, err)
			component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts(scenario.radixVolumeMount).BuildComponent()
			volumes, err := GetVolumes(context.Background(), testEnv.kubeUtil, namespace, &component, "", nil)
			assert.Nil(t, err)
			assert.Len(t, volumes, 1, "Unexpected volume count")
			assert.Equal(t, scenario.expectedVolumeName, volumes[0].Name, "Mismatching volume names")
			assert.NotNil(t, volumes[0].PersistentVolumeClaim, "Unexpected PVC")
			assert.True(t, internal.EqualTillPostfix(scenario.pvc.Name, volumes[0].PersistentVolumeClaim.ClaimName, nameRandPartLength), "Matching PVC name prefixes %s and %s", scenario.pvc.Name, volumes[0].PersistentVolumeClaim.ClaimName)
		}
	})
}

func (s *volumeMountTestSuite) Test_GetVolumesForComponent() {
	const (
		appName       = "any-app"
		environment   = "some-env"
		componentName = "some-component"
	)
	namespace := fmt.Sprintf("%s-%s", appName, environment)
	scenarios := []pvcTestScenario{
		{
			volumeMountTestScenario: volumeMountTestScenario{
				name:               "Blob CSI Azure volume, Status phase: Bound",
				radixVolumeMount:   radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "blob-volume1", Storage: "storage1", Path: "path1", GID: "1000"},
				expectedVolumeName: "csi-az-blob-some-component-blob-volume1-storage1",
				expectedPvcName:    "pvc-csi-az-blob-some-component-blob-volume1-storage1-12345",
			},
			pvc: createPvc(namespace, componentName, radixv1.MountTypeBlobFuse2FuseCsiAzure, func(pvc *corev1.PersistentVolumeClaim) { pvc.Status.Phase = corev1.ClaimBound }),
		},
		{
			volumeMountTestScenario: volumeMountTestScenario{
				name:               "Blob CSI Azure volume, Status phase: Pending",
				radixVolumeMount:   radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "blob-volume2", Storage: "storage2", Path: "path2", GID: "1000"},
				expectedVolumeName: "csi-az-blob-some-component-blob-volume2-storage2",
				expectedPvcName:    "pvc-csi-az-blob-some-component-blob-volume2-storage2-12345",
			},
			pvc: createPvc(namespace, componentName, radixv1.MountTypeBlobFuse2FuseCsiAzure, func(pvc *corev1.PersistentVolumeClaim) { pvc.Status.Phase = corev1.ClaimPending }),
		},
	}

	s.T().Run("No volumes", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		for _, factory := range s.radixCommonDeployComponentFactories {
			t.Logf("Test case for component %s", factory.GetTargetType())

			radixDeployment := buildRd(appName, environment, componentName, "", []radixv1.RadixVolumeMount{})
			deployComponent := radixDeployment.Spec.Components[0]

			volumes, err := GetVolumes(context.Background(), testEnv.kubeUtil, radixDeployment.GetNamespace(), &deployComponent, radixDeployment.GetName(), nil)

			assert.Nil(t, err)
			assert.Len(t, volumes, 0, "No volumes should be returned")
		}
	})
	s.T().Run("Exists volume", func(t *testing.T) {
		t.Parallel()
		testEnv := getTestEnv()
		for _, factory := range s.radixCommonDeployComponentFactories {
			for _, scenario := range scenarios {
				t.Logf("Test case %s for component %s", scenario.name, factory.GetTargetType())

				radixDeployment := buildRd(appName, environment, componentName, "", []radixv1.RadixVolumeMount{scenario.radixVolumeMount})
				deployComponent := radixDeployment.Spec.Components[0]

				volumes, err := GetVolumes(context.Background(), testEnv.kubeUtil, radixDeployment.GetNamespace(), &deployComponent, radixDeployment.GetName(), nil)

				assert.Nil(t, err)
				assert.Len(t, volumes, 1, "Unexpected volume count")
				assert.Equal(t, scenario.expectedVolumeName, volumes[0].Name, "Mismatching volume names")
				assert.NotNil(t, volumes[0].PersistentVolumeClaim, "PVC is nil")
				assert.True(t, internal.EqualTillPostfix(scenario.expectedPvcName, volumes[0].PersistentVolumeClaim.ClaimName, nameRandPartLength), "Mismatching PVC name prefixes %s and %s", scenario.expectedPvcName, volumes[0].PersistentVolumeClaim.ClaimName)
			}
		}
	})
}

func (s *volumeMountTestSuite) Test_GetRadixDeployComponentVolumeMounts() {
	const (
		appName       = "any-app"
		environment   = "some-env"
		componentName = "some-component"
	)
	scenarios := []volumeMountTestScenario{
		{
			name:               "Blob CSI Azure volume, Status phase: Bound",
			radixVolumeMount:   radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "blob-volume1", Storage: "storage1", Path: "path1", GID: "1000"},
			expectedVolumeName: "csi-az-blob-some-component-blob-volume1-storage1",
			expectedPvcName:    "pvc-csi-az-blob-some-component-blob-volume1-storage1-12345",
		},
		{
			name:               "Blob CSI Azure volume, Status phase: Pending",
			radixVolumeMount:   radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "blob-volume2", Storage: "storage2", Path: "path2", GID: "1000"},
			expectedVolumeName: "csi-az-blob-some-component-blob-volume2-storage2",
			expectedPvcName:    "pvc-csi-az-blob-some-component-blob-volume2-storage2-12345",
		},
		{
			name:               "Blob CSI Azure volume, Status phase: Pending",
			radixVolumeMount:   radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "blob-volume-with-long-name", Storage: "storage-with-long-name", Path: "path2", GID: "1000"},
			expectedVolumeName: "csi-az-blob-some-component-blob-volume-with-long-name-sto-12345",
			expectedPvcName:    "pvc-csi-az-blob-some-component-blob-volume-with-long-name-12345",
		},
	}

	s.T().Run("No volumes", func(t *testing.T) {
		t.Parallel()
		for _, factory := range s.radixCommonDeployComponentFactories {
			t.Logf("Test case for component %s", factory.GetTargetType())

			radixDeployment := buildRd(appName, environment, componentName, "", []radixv1.RadixVolumeMount{})
			deployComponent := radixDeployment.Spec.Components[0]

			volumes, err := GetRadixDeployComponentVolumeMounts(&deployComponent, "")

			assert.Nil(t, err)
			assert.Len(t, volumes, 0, "No volumes should be returned")
		}
	})
	s.T().Run("Exists volume", func(t *testing.T) {
		t.Parallel()
		for _, factory := range s.radixCommonDeployComponentFactories {
			for _, scenario := range scenarios {
				t.Logf("Test case %s for component %s", scenario.name, factory.GetTargetType())

				radixDeployment := buildRd(appName, environment, componentName, "", []radixv1.RadixVolumeMount{scenario.radixVolumeMount})
				deployComponent := radixDeployment.Spec.Components[0]

				volumeMounts, err := GetRadixDeployComponentVolumeMounts(&deployComponent, "")

				assert.Nil(t, err)
				assert.Len(t, volumeMounts, 1)
				volumeMountName := volumeMounts[0].Name
				if len(volumeMountName) > 60 {
					assert.True(t, internal.EqualTillPostfix(scenario.expectedVolumeName, volumeMountName, nameRandPartLength), "Mismatching volume name prefixes %s and %s", scenario.expectedVolumeName, volumeMountName)
				} else {
					assert.Equal(t, scenario.expectedVolumeName, volumeMountName)
				}
				assert.Less(t, len(volumeMountName), 64, "Volume name is too long")
				assert.Equal(t, scenario.radixVolumeMount.Path, volumeMounts[0].MountPath, "Mismatching volume paths")
			}
		}
	})
}

func (s *volumeMountTestSuite) Test_CreateOrUpdateCsiAzureResources() {
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
						setVolumeMountAttribute(pv, props.radixVolumeMountType, scenarioProps.changedNewRadixVolumeStorageName)
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
				pv.Spec.MountOptions = getMountOptions(modifyProps(props, func(props *expectedPvcPvProperties) { props.readOnly = false }))
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
						pv.Spec.MountOptions = getMountOptions(modifyProps(props, func(props *expectedPvcPvProperties) { props.readOnly = true }))
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
						pv.Spec.MountOptions = getMountOptions(props, "--streaming=true", "--block-cache-pool-size=750", "--use-adls=false")
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
						vm.BlobFuse2.StreamingOptions = &radixv1.BlobFuse2StreamingOptions{
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
							"--block-cache-pool-size=101",
							"--use-adls=false",
						)
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
						vm.BlobFuse2.StreamingOptions = &radixv1.BlobFuse2StreamingOptions{
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
			expectedPvc := createRandomPvc(props, props.namespace, componentName1)
			expectedPv := createRandomPv(props, props.namespace, componentName1)
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
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			existingPvc := createRandomAutoProvisionedPvcWithStorageClass(props, props.namespace, props.componentName, props.radixVolumeMountName)
			existingPvc.Spec.VolumeName = "" //auto-provisioned PVCs have empty volume name
			expectedPvc := createRandomPvc(props, props.namespace, componentName1)
			expectedPv := createRandomPv(props, props.namespace, componentName1)
			matchPvAndPvc(&expectedPv, &expectedPvc)
			return deploymentVolumesTestScenario{
				name:  "Create PV for existing PVC without PV name",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {}),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, func(v *corev1.Volume) {
						v.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: existingPvc.Name,
						}
					}),
				},
				existingPvcs: []corev1.PersistentVolumeClaim{
					existingPvc,
				},
				expectedPvcs: []corev1.PersistentVolumeClaim{
					existingPvc,
					expectedPvc,
				},
				existingPvs: []corev1.PersistentVolume{},
				expectedPvs: []corev1.PersistentVolume{
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
			expectedPvc := createRandomPvc(props, props.namespace, componentName1)
			expectedPv := createExpectedPvWithIdentity(props, nil)
			matchPvAndPvc(&expectedPv, &expectedPvc)
			return deploymentVolumesTestScenario{
				name:  "Create PV and PVC with useAzureIdentity",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {
						vm.BlobFuse2.UseAzureIdentity = pointers.Ptr(true)
						vm.BlobFuse2.StorageAccount = props.storageAccountName
						vm.BlobFuse2.SubscriptionId = props.subscriptionId
						vm.BlobFuse2.ResourceGroup = props.resourceGroup
						vm.BlobFuse2.UseAdls = nil
						vm.BlobFuse2.StreamingOptions = &radixv1.BlobFuse2StreamingOptions{Enabled: pointers.Ptr(false)}
					}),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, func(v *corev1.Volume) {}),
				},
				existingPvcs: []corev1.PersistentVolumeClaim{},
				expectedPvcs: []corev1.PersistentVolumeClaim{
					expectedPvc,
				},
				existingPvs: []corev1.PersistentVolume{},
				expectedPvs: []corev1.PersistentVolume{
					expectedPv,
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobVolume1Storage1(func(props *expectedPvcPvProperties) {
				setPropsForBlob2Fuse2AzureIdentity(props)
			})),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			existingPvc := createRandomPvc(props, props.namespace, componentName1)
			existingPv := createExpectedPvWithIdentity(props, nil)
			matchPvAndPvc(&existingPv, &existingPvc)
			expectedPvc := createRandomPvc(props, props.namespace, componentName1)
			expectedPv := createExpectedPvWithIdentity(props, nil)
			matchPvAndPvc(&expectedPv, &expectedPvc)
			return deploymentVolumesTestScenario{
				name:  "Not changed PV and PVC with useAzureIdentity",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {
						setVolumeMountPropsForBlob2Fuse2AzureIdentity(props, vm)
					}),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, func(v *corev1.Volume) {
						v.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: existingPvc.Name,
						}
					}),
				},
				existingPvcs: []corev1.PersistentVolumeClaim{
					existingPvc,
				},
				expectedPvcs: []corev1.PersistentVolumeClaim{
					expectedPvc,
				},
				existingPvs: []corev1.PersistentVolume{
					existingPv,
				},
				expectedPvs: []corev1.PersistentVolume{
					expectedPv,
				},
			}
		}
		return []deploymentVolumesTestScenario{
			getScenario(getPropsCsiBlobVolume1Storage1(func(props *expectedPvcPvProperties) {
				setPropsForBlob2Fuse2AzureIdentity(props)
			})),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			existingPvc := createRandomPvc(props, props.namespace, componentName1)
			existingPv := createExpectedPvWithIdentity(props, nil)
			matchPvAndPvc(&existingPv, &existingPvc)
			changedProps := modifyProps(props, func(props *expectedPvcPvProperties) {
				props.storageAccountName = testChangedStorageAccountName
			})
			expectedPvc := createRandomPvc(changedProps, props.namespace, componentName1)
			expectedPv := createExpectedPvWithIdentity(changedProps, nil)
			matchPvAndPvc(&expectedPv, &expectedPvc)
			return deploymentVolumesTestScenario{
				name:  "Created PV and PVC with useAzureIdentity on changed storage account",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {
						setVolumeMountPropsForBlob2Fuse2AzureIdentity(props, vm)
						vm.BlobFuse2.StorageAccount = testChangedStorageAccountName
					}),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, func(v *corev1.Volume) {
						v.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: existingPvc.Name,
						}
					}),
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
				setPropsForBlob2Fuse2AzureIdentity(props)
			})),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			existingPvc := createRandomPvc(props, props.namespace, componentName1)
			existingPv := createExpectedPvWithIdentity(props, nil)
			matchPvAndPvc(&existingPv, &existingPvc)
			changedProps := modifyProps(props, func(props *expectedPvcPvProperties) {
				props.resourceGroup = testChangedResourceGroup
			})
			expectedPvc := createRandomPvc(changedProps, props.namespace, componentName1)
			expectedPv := createExpectedPvWithIdentity(changedProps, nil)
			matchPvAndPvc(&expectedPv, &expectedPvc)
			return deploymentVolumesTestScenario{
				name:  "Created PV and PVC with useAzureIdentity on changed resource group",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {
						setVolumeMountPropsForBlob2Fuse2AzureIdentity(props, vm)
						vm.BlobFuse2.ResourceGroup = testChangedResourceGroup
					}),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, func(v *corev1.Volume) {
						v.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: existingPvc.Name,
						}
					}),
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
				setPropsForBlob2Fuse2AzureIdentity(props)
			})),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			existingPvc := createRandomPvc(props, props.namespace, componentName1)
			existingPv := createExpectedPvWithIdentity(props, nil)
			matchPvAndPvc(&existingPv, &existingPvc)
			changedProps := modifyProps(props, func(props *expectedPvcPvProperties) {
				props.subscriptionId = testChangedSubscriptionId
			})
			expectedPvc := createRandomPvc(changedProps, props.namespace, componentName1)
			expectedPv := createExpectedPvWithIdentity(changedProps, nil)
			matchPvAndPvc(&expectedPv, &expectedPvc)
			return deploymentVolumesTestScenario{
				name:  "Created PV and PVC with useAzureIdentity on changed subscription id",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {
						setVolumeMountPropsForBlob2Fuse2AzureIdentity(props, vm)
						vm.BlobFuse2.SubscriptionId = testChangedSubscriptionId
					}),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, func(v *corev1.Volume) {
						v.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: existingPvc.Name,
						}
					}),
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
				setPropsForBlob2Fuse2AzureIdentity(props)
			})),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			existingPvc := createRandomPvc(props, props.namespace, componentName1)
			existingPv := createExpectedPvWithIdentity(props, nil)
			matchPvAndPvc(&existingPv, &existingPvc)
			changedProps := modifyProps(props, func(props *expectedPvcPvProperties) {
				props.tenantId = testChangedTenantId
			})
			expectedPvc := createRandomPvc(changedProps, props.namespace, componentName1)
			expectedPv := createExpectedPvWithIdentity(changedProps, nil)
			matchPvAndPvc(&expectedPv, &expectedPvc)
			return deploymentVolumesTestScenario{
				name:  "Created PV and PVC with useAzureIdentity on changed tenant id",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {
						setVolumeMountPropsForBlob2Fuse2AzureIdentity(props, vm)
						vm.BlobFuse2.TenantId = testChangedTenantId
					}),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, func(v *corev1.Volume) {
						v.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: existingPvc.Name,
						}
					}),
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
				setPropsForBlob2Fuse2AzureIdentity(props)
				props.tenantId = testTenantId
			})),
		}
	}()...)
	scenarios = append(scenarios, func() []deploymentVolumesTestScenario {
		getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
			existingPvc := createRandomPvc(props, props.namespace, componentName1)
			existingPv := createExpectedPvWithIdentity(props, nil)
			matchPvAndPvc(&existingPv, &existingPvc)
			changedProps := modifyProps(props, func(props *expectedPvcPvProperties) {
				props.tenantId = testChangedTenantId
			})
			expectedPvc := createRandomPvc(changedProps, props.namespace, componentName1)
			expectedPv := createExpectedPv(changedProps, nil)
			matchPvAndPvc(&expectedPv, &expectedPvc)
			return deploymentVolumesTestScenario{
				name:  "Changed PV and PVC from useAzureIdentity to use a Client Secret",
				props: props,
				radixVolumeMounts: []radixv1.RadixVolumeMount{
					createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {}),
				},
				volumes: []corev1.Volume{
					createTestVolume(props, func(v *corev1.Volume) {
						v.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: existingPvc.Name,
						}
					}),
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
				setPropsForBlob2Fuse2AzureIdentity(props)
			})),
		}
	}()...)

	s.T().Run("CSI Azure volume PVCs and PersistentVolume", func(t *testing.T) {
		for _, factory := range s.radixCommonDeployComponentFactories {
			for _, scenario := range scenarios {
				t.Logf("Test case %s, volume type %s, component %s", scenario.name, scenario.props.radixVolumeMountType, factory.GetTargetType())
				testEnv := getTestEnv()
				radixDeployment := buildRd(appName1, envName1, componentName1, testClientId, scenario.radixVolumeMounts)
				putExistingDeploymentVolumesScenarioDataToFakeCluster(testEnv.kubeUtil.KubeClient(), &scenario)
				desiredVolumes := getDesiredDeployment(componentName1, scenario.volumes).Spec.Template.Spec.Volumes

				deployComponent := radixDeployment.Spec.Components[0]
				actualVolumes, err := CreateOrUpdateCsiAzureVolumeResourcesForDeployComponent(context.Background(), testEnv.kubeUtil.KubeClient(), radixDeployment, utils.GetEnvironmentNamespace(appName1, envName1), &deployComponent, desiredVolumes)
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

func setVolumeMountPropsForBlob2Fuse2AzureIdentity(props expectedPvcPvProperties, vm *radixv1.RadixVolumeMount) {
	vm.BlobFuse2.UseAzureIdentity = pointers.Ptr(true)
	vm.BlobFuse2.StorageAccount = props.storageAccountName
	vm.BlobFuse2.SubscriptionId = props.subscriptionId
	vm.BlobFuse2.ResourceGroup = props.resourceGroup
	vm.BlobFuse2.UseAdls = nil
	vm.BlobFuse2.StreamingOptions = &radixv1.BlobFuse2StreamingOptions{Enabled: pointers.Ptr(false)}
}

func setPropsForBlob2Fuse2AzureIdentity(props *expectedPvcPvProperties) {
	props.radixVolumeMountType = radixv1.MountTypeBlobFuse2Fuse2CsiAzure
	props.volumeName = "csi-blobfuse2-fuse2-some-component-volume1-storage1"
	props.clientId = testClientId
	props.subscriptionId = testSubscriptionId
	props.resourceGroup = testResourceGroup
	props.storageAccountName = testStorageAccountName
}
