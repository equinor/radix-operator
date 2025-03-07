//nolint:staticcheck
package volumemount

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/internal"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

type deployCommonComponentBuilder struct {
	name         string
	volumeMounts []radixv1.RadixVolumeMount
	factory      radixCommonDeployComponentFactory
}

func (dcb *deployCommonComponentBuilder) WithName(name string) *deployCommonComponentBuilder {
	dcb.name = name
	return dcb
}

func (dcb *deployCommonComponentBuilder) WithVolumeMounts(volumeMounts ...radixv1.RadixVolumeMount) *deployCommonComponentBuilder {
	dcb.volumeMounts = volumeMounts
	return dcb
}

func (dcb *deployCommonComponentBuilder) BuildComponent() radixv1.RadixCommonDeployComponent {
	component := dcb.factory.Create()
	component.SetName(dcb.name)
	component.SetVolumeMounts(dcb.volumeMounts)
	return component
}

func newDeployCommonComponentBuilder(factory radixCommonDeployComponentFactory) *deployCommonComponentBuilder {
	return &deployCommonComponentBuilder{factory: factory}
}

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

			component := newDeployCommonComponentBuilder(factory).
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
			component := newDeployCommonComponentBuilder(factory).WithName("app").
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
			component := newDeployCommonComponentBuilder(factory).
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
	}
	s.T().Run("Failing Blob CSI Azure volume mount", func(t *testing.T) {
		t.Parallel()
		for _, factory := range s.radixCommonDeployComponentFactories {
			for _, testCase := range scenarios {
				t.Logf("Test case %s for component %s", testCase.name, factory.GetTargetType())
				component := newDeployCommonComponentBuilder(factory).
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
			t.Logf("Scenario %s for volume mount type %s, PVC status phase '%v'", scenario.name, string(scenario.radixVolumeMount.GetVolumeMountType()), scenario.pvc.Status.Phase)
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
			t.Logf("Scenario %s for volume mount type %s, PVC status phase '%v'", scenario.name, string(scenario.radixVolumeMount.GetVolumeMountType()), scenario.pvc.Status.Phase)
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

func (s *volumeMountTestSuite) Test_RadixVolumeMountPVCAndPVBinding() {
	tests := map[string]struct {
		volumeMount radixv1.RadixVolumeMount
	}{
		"deprecated volume": {
			volumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "anyname",
				Path:    "anypath",
				Storage: "anystorage",
			},
		},
		"blofuse2": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyname",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
				},
			},
		},
	}

	for testName, test := range tests {
		s.Run(testName, func() {
			const (
				appName       = "anyapp"
				envName       = "anyenv"
				namespace     = "anyns"
				componentName = "anycomp"
				pvcName       = "anypvc"
			)
			kubeClient := kubefake.NewClientset()
			rd := &radixv1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "anyrd", Namespace: namespace},
				Spec:       radixv1.RadixDeploymentSpec{AppName: appName, Environment: envName},
			}
			component := &radixv1.RadixDeployComponent{Name: componentName, VolumeMounts: []radixv1.RadixVolumeMount{test.volumeMount}}
			volumeName, err := GetVolumeMountVolumeName(&test.volumeMount, componentName)
			s.Require().NoError(err)
			desiredVolumes := []corev1.Volume{{Name: volumeName, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}}}
			actualVolumes, err := CreateOrUpdateCsiAzureVolumeResourcesForDeployComponent(context.Background(), kubeClient, rd, component, desiredVolumes)
			s.Require().NoError(err)
			s.Require().Len(actualVolumes, 1)
			s.Equal(pvcName, actualVolumes[0].VolumeSource.PersistentVolumeClaim.ClaimName)
			pvc, err := kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
			s.Require().NoError(err)
			_, err = kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
			s.Require().NoError(err)
		})
	}
}

func (s *volumeMountTestSuite) Test_RadixVolumeMountPVCSpec() {
	tests := map[string]struct {
		volumeMount     radixv1.RadixVolumeMount
		expectedPVCSpec corev1.PersistentVolumeClaimSpec
	}{
		"deprecated volume: default settings": {
			volumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "anyname",
				Path:    "anypath",
				Storage: "anystorage",
			},
			expectedPVCSpec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")}},
				StorageClassName: pointers.Ptr(""),
				VolumeMode:       pointers.Ptr(corev1.PersistentVolumeFilesystem),
			},
		},
		"deprecated volume: AccessMode=ReadOnlyMany": {
			volumeMount: radixv1.RadixVolumeMount{
				Type:       radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:       "anyname",
				Path:       "anypath",
				Storage:    "anystorage",
				AccessMode: "ReadOnlyMany",
			},
			expectedPVCSpec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")}},
				StorageClassName: pointers.Ptr(""),
				VolumeMode:       pointers.Ptr(corev1.PersistentVolumeFilesystem),
			},
		},
		"deprecated volume: AccessMode=ReadWriteOnce": {
			volumeMount: radixv1.RadixVolumeMount{
				Type:       radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:       "anyname",
				Path:       "anypath",
				Storage:    "anystorage",
				AccessMode: "ReadWriteOnce",
			},
			expectedPVCSpec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")}},
				StorageClassName: pointers.Ptr(""),
				VolumeMode:       pointers.Ptr(corev1.PersistentVolumeFilesystem),
			},
		},
		"deprecated volume: AccessMode=ReadWriteMany": {
			volumeMount: radixv1.RadixVolumeMount{
				Type:       radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:       "anyname",
				Path:       "anypath",
				Storage:    "anystorage",
				AccessMode: "ReadWriteMany",
			},
			expectedPVCSpec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
				Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")}},
				StorageClassName: pointers.Ptr(""),
				VolumeMode:       pointers.Ptr(corev1.PersistentVolumeFilesystem),
			},
		},
		"deprecated volume: RequestsStorage": {
			volumeMount: radixv1.RadixVolumeMount{
				Type:            radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:            "anyname",
				Path:            "anypath",
				Storage:         "anystorage",
				RequestsStorage: resource.MustParse("123G"),
			},
			expectedPVCSpec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("123G")}},
				StorageClassName: pointers.Ptr(""),
				VolumeMode:       pointers.Ptr(corev1.PersistentVolumeFilesystem),
			},
		},
		"blofuse2: default settings": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyname",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
				},
			},
			expectedPVCSpec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")}},
				StorageClassName: pointers.Ptr(""),
				VolumeMode:       pointers.Ptr(corev1.PersistentVolumeFilesystem),
			},
		},
		"blofuse2: AccessMode ReadOnlyMany": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyname",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					AccessMode: "ReadOnlyMany",
					Container:  "anycontainer",
				},
			},
			expectedPVCSpec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")}},
				StorageClassName: pointers.Ptr(""),
				VolumeMode:       pointers.Ptr(corev1.PersistentVolumeFilesystem),
			},
		},
		"blofuse2: AccessMode ReadWriteOnce": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyname",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					AccessMode: "ReadWriteOnce",
					Container:  "anycontainer",
				},
			},
			expectedPVCSpec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")}},
				StorageClassName: pointers.Ptr(""),
				VolumeMode:       pointers.Ptr(corev1.PersistentVolumeFilesystem),
			},
		},
		"blofuse2: AccessMode ReadWriteMany": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyname",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					AccessMode: "ReadWriteMany",
					Container:  "anycontainer",
				},
			},
			expectedPVCSpec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
				Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")}},
				StorageClassName: pointers.Ptr(""),
				VolumeMode:       pointers.Ptr(corev1.PersistentVolumeFilesystem),
			},
		},
		"blofuse2: AccessMode RequestsStorage": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyname",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					RequestsStorage: resource.MustParse("123G"),
					Container:       "anycontainer",
				},
			},
			expectedPVCSpec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("123G")}},
				StorageClassName: pointers.Ptr(""),
				VolumeMode:       pointers.Ptr(corev1.PersistentVolumeFilesystem),
			},
		},
	}

	for testName, test := range tests {
		s.Run(testName, func() {
			const (
				appName       = "anyapp"
				envName       = "anyenv"
				namespace     = "anyns"
				componentName = "anycomp"
				pvcName       = "anypvc"
			)
			kubeClient := kubefake.NewClientset()
			rd := &radixv1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "anyrd", Namespace: namespace},
				Spec:       radixv1.RadixDeploymentSpec{AppName: appName, Environment: envName},
			}
			component := &radixv1.RadixDeployComponent{Name: componentName, VolumeMounts: []radixv1.RadixVolumeMount{test.volumeMount}}
			volumeName, err := GetVolumeMountVolumeName(&test.volumeMount, componentName)
			s.Require().NoError(err)
			desiredVolumes := []corev1.Volume{{Name: volumeName, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}}}
			_, err = CreateOrUpdateCsiAzureVolumeResourcesForDeployComponent(context.Background(), kubeClient, rd, component, desiredVolumes)
			s.Require().NoError(err)
			actualPVC, err := kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
			s.Require().NoError(err)

			compareResourceList := func(expected, actual corev1.ResourceList) bool {
				if len(expected) != len(actual) {
					return false
				}
				for k, v := range actual {
					if !v.Equal(expected[k]) {
						return false
					}
				}
				return true
			}
			s.True(compareResourceList(test.expectedPVCSpec.Resources.Requests, actualPVC.Spec.Resources.Requests))
			s.True(compareResourceList(test.expectedPVCSpec.Resources.Limits, actualPVC.Spec.Resources.Limits))
			s.ElementsMatch(test.expectedPVCSpec.AccessModes, actualPVC.Spec.AccessModes)
			s.Equal(test.expectedPVCSpec.Selector, actualPVC.Spec.Selector)
			s.Equal(test.expectedPVCSpec.VolumeMode, actualPVC.Spec.VolumeMode)
			s.Equal(test.expectedPVCSpec.DataSource, actualPVC.Spec.DataSource)
			s.Equal(test.expectedPVCSpec.DataSourceRef, actualPVC.Spec.DataSourceRef)
			s.Equal(test.expectedPVCSpec.VolumeAttributesClassName, actualPVC.Spec.VolumeAttributesClassName)
			// VolumeName cannot be determined in advance since it is non-deterministic and generated upon PVC creation, so no need to compare
		})
	}
}

func (s *volumeMountTestSuite) Test_RadixVolumeMountPVCLabels() {
	tests := map[string]struct {
		volumeMount    radixv1.RadixVolumeMount
		appName        string
		componentName  string
		expectedLabels map[string]string
	}{
		"deprecated volume": {
			volumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "vol1",
				Path:    "anypath",
				Storage: "anystorage",
			},
			appName:       "app1",
			componentName: "comp1",
			expectedLabels: map[string]string{
				kube.RadixAppLabel:             "app1",
				kube.RadixComponentLabel:       "comp1",
				kube.RadixVolumeMountNameLabel: "vol1",
				kube.RadixMountTypeLabel:       string(radixv1.MountTypeBlobFuse2FuseCsiAzure),
			},
		},
		"blofuse2": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "vol2",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
				},
			},
			appName:       "app2",
			componentName: "comp2",
			expectedLabels: map[string]string{
				kube.RadixAppLabel:             "app2",
				kube.RadixComponentLabel:       "comp2",
				kube.RadixVolumeMountNameLabel: "vol2",
				kube.RadixMountTypeLabel:       string(radixv1.MountTypeBlobFuse2Fuse2CsiAzure),
			},
		},
	}

	for testName, test := range tests {
		s.Run(testName, func() {
			const (
				envName   = "anyenv"
				namespace = "anyns"
				pvcName   = "anypvc"
			)
			kubeClient := kubefake.NewClientset()
			rd := &radixv1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "anyrd", Namespace: namespace},
				Spec:       radixv1.RadixDeploymentSpec{AppName: test.appName, Environment: envName},
			}
			component := &radixv1.RadixDeployComponent{Name: test.componentName, VolumeMounts: []radixv1.RadixVolumeMount{test.volumeMount}}
			volumeName, err := GetVolumeMountVolumeName(&test.volumeMount, test.componentName)
			s.Require().NoError(err)
			desiredVolumes := []corev1.Volume{{Name: volumeName, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}}}
			_, err = CreateOrUpdateCsiAzureVolumeResourcesForDeployComponent(context.Background(), kubeClient, rd, component, desiredVolumes)
			s.Require().NoError(err)
			pvc, err := kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
			s.Require().NoError(err)
			s.Equal(test.expectedLabels, pvc.GetLabels())
		})
	}
}

func (s *volumeMountTestSuite) Test_RadixVolumeMountPVSpec_ExcludingMountOptions() {
	tests := map[string]struct {
		appName        string
		envName        string
		componentName  string
		clientId       string
		volumeMount    radixv1.RadixVolumeMount
		expectedPVSpec corev1.PersistentVolumeSpec
	}{
		"deprecated volume: default settings": {
			appName:       "app1",
			envName:       "env1",
			componentName: "comp1",
			volumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "vol1",
				Path:    "anypath",
				Storage: "storage1",
			},
			expectedPVSpec: corev1.PersistentVolumeSpec{
				Capacity:    corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "blob.csi.azure.com",
						NodeStageSecretRef: &corev1.SecretReference{
							Namespace: "app1-env1",
							Name:      defaults.GetCsiAzureVolumeMountCredsSecretName("comp1", "vol1"),
						},
						VolumeAttributes: map[string]string{
							"protocol":        "fuse",
							"containerName":   "storage1",
							"secretnamespace": "app1-env1",
						},
					},
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			},
		},
		"deprecated volume: AccessMode=ReadOnlyMany": {
			appName:       "app1",
			envName:       "env1",
			componentName: "comp1",
			volumeMount: radixv1.RadixVolumeMount{
				Type:       radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:       "vol1",
				Path:       "anypath",
				Storage:    "storage1",
				AccessMode: "ReadOnlyMany",
			},
			expectedPVSpec: corev1.PersistentVolumeSpec{
				Capacity:    corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "blob.csi.azure.com",
						NodeStageSecretRef: &corev1.SecretReference{
							Namespace: "app1-env1",
							Name:      defaults.GetCsiAzureVolumeMountCredsSecretName("comp1", "vol1"),
						},
						VolumeAttributes: map[string]string{
							"protocol":        "fuse",
							"containerName":   "storage1",
							"secretnamespace": "app1-env1",
						},
					},
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			},
		},
		"deprecated volume: AccessMode=ReadWriteOnce": {
			appName:       "app1",
			envName:       "env1",
			componentName: "comp1",
			volumeMount: radixv1.RadixVolumeMount{
				Type:       radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:       "vol1",
				Path:       "anypath",
				Storage:    "storage1",
				AccessMode: "ReadWriteOnce",
			},
			expectedPVSpec: corev1.PersistentVolumeSpec{
				Capacity:    corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "blob.csi.azure.com",
						NodeStageSecretRef: &corev1.SecretReference{
							Namespace: "app1-env1",
							Name:      defaults.GetCsiAzureVolumeMountCredsSecretName("comp1", "vol1"),
						},
						VolumeAttributes: map[string]string{
							"protocol":        "fuse",
							"containerName":   "storage1",
							"secretnamespace": "app1-env1",
						},
					},
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			},
		},
		"deprecated volume: AccessMode=ReadWriteMany": {
			appName:       "app1",
			envName:       "env1",
			componentName: "comp1",
			volumeMount: radixv1.RadixVolumeMount{
				Type:       radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:       "vol1",
				Path:       "anypath",
				Storage:    "storage1",
				AccessMode: "ReadWriteMany",
			},
			expectedPVSpec: corev1.PersistentVolumeSpec{
				Capacity:    corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "blob.csi.azure.com",
						NodeStageSecretRef: &corev1.SecretReference{
							Namespace: "app1-env1",
							Name:      defaults.GetCsiAzureVolumeMountCredsSecretName("comp1", "vol1"),
						},
						VolumeAttributes: map[string]string{
							"protocol":        "fuse",
							"containerName":   "storage1",
							"secretnamespace": "app1-env1",
						},
					},
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			},
		},
		"deprecated volume: RequestsStorage": {
			appName:       "app1",
			envName:       "env1",
			componentName: "comp1",
			volumeMount: radixv1.RadixVolumeMount{
				Type:            radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:            "vol1",
				Path:            "anypath",
				Storage:         "storage1",
				RequestsStorage: resource.MustParse("123G"),
			},
			expectedPVSpec: corev1.PersistentVolumeSpec{
				Capacity:    corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("123G")},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "blob.csi.azure.com",
						NodeStageSecretRef: &corev1.SecretReference{
							Namespace: "app1-env1",
							Name:      defaults.GetCsiAzureVolumeMountCredsSecretName("comp1", "vol1"),
						},
						VolumeAttributes: map[string]string{
							"protocol":        "fuse",
							"containerName":   "storage1",
							"secretnamespace": "app1-env1",
						},
					},
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			},
		},
		"blofuse2: default settings": {
			appName:       "app2",
			envName:       "env2",
			componentName: "comp2",
			volumeMount: radixv1.RadixVolumeMount{
				Name: "vol2",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "container2",
				},
			},
			expectedPVSpec: corev1.PersistentVolumeSpec{
				Capacity:    corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "blob.csi.azure.com",
						NodeStageSecretRef: &corev1.SecretReference{
							Namespace: "app2-env2",
							Name:      defaults.GetCsiAzureVolumeMountCredsSecretName("comp2", "vol2"),
						},
						VolumeAttributes: map[string]string{
							"protocol":        "fuse2",
							"containerName":   "container2",
							"secretnamespace": "app2-env2",
						},
					},
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			},
		},
		"blofuse2: AccessMode=ReadOnlyMany": {
			appName:       "app2",
			envName:       "env2",
			componentName: "comp2",
			volumeMount: radixv1.RadixVolumeMount{
				Name: "vol2",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:  "container2",
					AccessMode: "ReadOnlyMany",
				},
			},
			expectedPVSpec: corev1.PersistentVolumeSpec{
				Capacity:    corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "blob.csi.azure.com",
						NodeStageSecretRef: &corev1.SecretReference{
							Namespace: "app2-env2",
							Name:      defaults.GetCsiAzureVolumeMountCredsSecretName("comp2", "vol2"),
						},
						VolumeAttributes: map[string]string{
							"protocol":        "fuse2",
							"containerName":   "container2",
							"secretnamespace": "app2-env2",
						},
					},
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			},
		},
		"blofuse2: AccessMode=ReadWriteOnce": {
			appName:       "app2",
			envName:       "env2",
			componentName: "comp2",
			volumeMount: radixv1.RadixVolumeMount{
				Name: "vol2",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:  "container2",
					AccessMode: "ReadWriteOnce",
				},
			},
			expectedPVSpec: corev1.PersistentVolumeSpec{
				Capacity:    corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "blob.csi.azure.com",
						NodeStageSecretRef: &corev1.SecretReference{
							Namespace: "app2-env2",
							Name:      defaults.GetCsiAzureVolumeMountCredsSecretName("comp2", "vol2"),
						},
						VolumeAttributes: map[string]string{
							"protocol":        "fuse2",
							"containerName":   "container2",
							"secretnamespace": "app2-env2",
						},
					},
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			},
		},
		"blofuse2: AccessMode=ReadWriteMany": {
			appName:       "app2",
			envName:       "env2",
			componentName: "comp2",
			volumeMount: radixv1.RadixVolumeMount{
				Name: "vol2",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:  "container2",
					AccessMode: "ReadWriteMany",
				},
			},
			expectedPVSpec: corev1.PersistentVolumeSpec{
				Capacity:    corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "blob.csi.azure.com",
						NodeStageSecretRef: &corev1.SecretReference{
							Namespace: "app2-env2",
							Name:      defaults.GetCsiAzureVolumeMountCredsSecretName("comp2", "vol2"),
						},
						VolumeAttributes: map[string]string{
							"protocol":        "fuse2",
							"containerName":   "container2",
							"secretnamespace": "app2-env2",
						},
					},
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			},
		},
		"blofuse2: RequestsStorage": {
			appName:       "app2",
			envName:       "env2",
			componentName: "comp2",
			volumeMount: radixv1.RadixVolumeMount{
				Name: "vol2",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:       "container2",
					RequestsStorage: resource.MustParse("123G"),
				},
			},
			expectedPVSpec: corev1.PersistentVolumeSpec{
				Capacity:    corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("123G")},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "blob.csi.azure.com",
						NodeStageSecretRef: &corev1.SecretReference{
							Namespace: "app2-env2",
							Name:      defaults.GetCsiAzureVolumeMountCredsSecretName("comp2", "vol2"),
						},
						VolumeAttributes: map[string]string{
							"protocol":        "fuse2",
							"containerName":   "container2",
							"secretnamespace": "app2-env2",
						},
					},
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			},
		},
		"blofuse2: StorageAccount": {
			appName:       "app2",
			envName:       "env2",
			componentName: "comp2",
			volumeMount: radixv1.RadixVolumeMount{
				Name: "vol2",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:      "container2",
					StorageAccount: "storage2",
				},
			},
			expectedPVSpec: corev1.PersistentVolumeSpec{
				Capacity:    corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "blob.csi.azure.com",
						NodeStageSecretRef: &corev1.SecretReference{
							Namespace: "app2-env2",
							Name:      defaults.GetCsiAzureVolumeMountCredsSecretName("comp2", "vol2"),
						},
						VolumeAttributes: map[string]string{
							"protocol":        "fuse2",
							"containerName":   "container2",
							"secretnamespace": "app2-env2",
							"storageAccount":  "storage2",
						},
					},
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			},
		},
		"blofuse2: UseAzureIdentity - credentials secret not set": {
			appName:       "app2",
			envName:       "env2",
			componentName: "comp2",
			volumeMount: radixv1.RadixVolumeMount{
				Name: "vol2",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "container2",
					UseAzureIdentity: pointers.Ptr(true),
				},
			},
			expectedPVSpec: corev1.PersistentVolumeSpec{
				Capacity:    corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "blob.csi.azure.com",
						VolumeAttributes: map[string]string{
							"protocol":      "fuse2",
							"containerName": "container2",
							"clientID":      "",
							"resourcegroup": "",
						},
					},
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			},
		},
		"blofuse2: UseAzureIdentity specific volume attributes": {
			appName:       "app2",
			envName:       "env2",
			componentName: "comp2",
			clientId:      "anyclientid",
			volumeMount: radixv1.RadixVolumeMount{
				Name: "vol2",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "container2",
					UseAzureIdentity: pointers.Ptr(true),
					ResourceGroup:    "anyresourcegroup",
					SubscriptionId:   "anysubscription",
					TenantId:         "anytenant",
				},
			},
			expectedPVSpec: corev1.PersistentVolumeSpec{
				Capacity:    corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "blob.csi.azure.com",
						VolumeAttributes: map[string]string{
							"protocol":       "fuse2",
							"containerName":  "container2",
							"clientID":       "anyclientid",
							"resourcegroup":  "anyresourcegroup",
							"subscriptionid": "anysubscription",
							"tenantID":       "anytenant",
						},
					},
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			},
		},
		"blofuse2: UseAzureIdentity specific volume attributes ignored when false": {
			appName:       "app2",
			envName:       "env2",
			componentName: "comp2",
			clientId:      "anyclientid",
			volumeMount: radixv1.RadixVolumeMount{
				Name: "vol2",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "container2",
					UseAzureIdentity: pointers.Ptr(false),
					ResourceGroup:    "anyresourcegroup",
					SubscriptionId:   "anysubscription",
					TenantId:         "anytenant",
				},
			},
			expectedPVSpec: corev1.PersistentVolumeSpec{
				Capacity:    corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "blob.csi.azure.com",
						VolumeAttributes: map[string]string{
							"protocol":        "fuse2",
							"containerName":   "container2",
							"secretnamespace": "app2-env2",
						},
						NodeStageSecretRef: &corev1.SecretReference{
							Namespace: "app2-env2",
							Name:      defaults.GetCsiAzureVolumeMountCredsSecretName("comp2", "vol2"),
						},
					},
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			},
		},
	}

	for testName, test := range tests {
		s.Run(testName, func() {
			const (
				pvcName = "anypvc"
			)
			namespace := utils.GetEnvironmentNamespace(test.appName, test.envName)
			kubeClient := kubefake.NewClientset()
			rd := &radixv1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "anyrd", Namespace: namespace},
				Spec:       radixv1.RadixDeploymentSpec{AppName: test.appName, Environment: test.envName},
			}
			component := &radixv1.RadixDeployComponent{
				Name:         test.componentName,
				VolumeMounts: []radixv1.RadixVolumeMount{test.volumeMount},
				Identity:     &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: test.clientId}},
			}
			volumeName, err := GetVolumeMountVolumeName(&test.volumeMount, test.componentName)
			s.Require().NoError(err)
			desiredVolumes := []corev1.Volume{{Name: volumeName, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}}}
			_, err = CreateOrUpdateCsiAzureVolumeResourcesForDeployComponent(context.Background(), kubeClient, rd, component, desiredVolumes)
			s.Require().NoError(err)
			pvc, err := kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
			s.Require().NoError(err)
			actualPV, err := kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
			s.Require().NoError(err)

			compareResourceList := func(expected, actual corev1.ResourceList) bool {
				if len(expected) != len(actual) {
					return false
				}
				for k, v := range actual {
					if !v.Equal(expected[k]) {
						return false
					}
				}
				return true
			}
			s.True(compareResourceList(test.expectedPVSpec.Capacity, actualPV.Spec.Capacity))
			s.ElementsMatch(test.expectedPVSpec.AccessModes, actualPV.Spec.AccessModes)
			s.Equal(test.expectedPVSpec.PersistentVolumeReclaimPolicy, actualPV.Spec.PersistentVolumeReclaimPolicy)
			s.Equal(test.expectedPVSpec.StorageClassName, actualPV.Spec.StorageClassName)
			s.Equal(test.expectedPVSpec.VolumeMode, actualPV.Spec.VolumeMode)
			s.Equal(test.expectedPVSpec.NodeAffinity, actualPV.Spec.NodeAffinity)
			s.Equal(test.expectedPVSpec.VolumeAttributesClassName, actualPV.Spec.VolumeAttributesClassName)

			// Test that actual and expected PersistentVolumeSource fields are nil or not
			expectedVal := reflect.ValueOf(test.expectedPVSpec.PersistentVolumeSource)
			expectedType := reflect.TypeOf(test.expectedPVSpec.PersistentVolumeSource)
			actualVal := reflect.ValueOf(actualPV.Spec.PersistentVolumeSource)
			for i := range expectedVal.NumField() {
				ef := expectedVal.Field(i)
				n := expectedType.Field(i).Name
				af := actualVal.FieldByName(n)
				s.Equal(ef.IsNil(), af.IsNil(), "field PersistentVolumeSource.%s", n)
			}

			if expectedCSI, actualCSI := test.expectedPVSpec.PersistentVolumeSource.CSI, actualPV.Spec.PersistentVolumeSource.CSI; expectedCSI != nil {
				s.Equal(expectedCSI.Driver, actualCSI.Driver)
				s.NotEmpty(actualCSI.VolumeHandle)
				s.Equal(expectedCSI.ReadOnly, actualCSI.ReadOnly)
				s.Equal(expectedCSI.FSType, actualCSI.FSType)
				s.Equal(expectedCSI.VolumeAttributes, actualCSI.VolumeAttributes)
				s.Equal(expectedCSI.ControllerPublishSecretRef, actualCSI.ControllerPublishSecretRef)
				s.Equal(expectedCSI.NodeStageSecretRef, actualCSI.NodeStageSecretRef)
				s.Equal(expectedCSI.NodePublishSecretRef, actualCSI.NodePublishSecretRef)
				s.Equal(expectedCSI.ControllerExpandSecretRef, actualCSI.ControllerExpandSecretRef)
				s.Equal(expectedCSI.NodeExpandSecretRef, actualCSI.NodeExpandSecretRef)
			}
		})
	}
}

func (s *volumeMountTestSuite) Test_RadixVolumeMountPVSpec_MountOptions() {
	tests := map[string]struct {
		volumeMount          radixv1.RadixVolumeMount
		expectedMountOptions []string
	}{
		"deprecated volume: default settings": {
			volumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "anyvol",
				Path:    "anypath",
				Storage: "anystorage",
			},
			expectedMountOptions: []string{
				"--file-cache-timeout-in-seconds=120",
				"--cancel-list-on-mount-seconds=0",
				"--attr-cache-timeout=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
				"--read-only=true",
			},
		},
		"deprecated volume: AccessMode=ReadOnlyMany": {
			volumeMount: radixv1.RadixVolumeMount{
				Type:       radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:       "anyvol",
				Path:       "anypath",
				Storage:    "anystorage",
				AccessMode: "ReadOnlyMany",
			},
			expectedMountOptions: []string{
				"--file-cache-timeout-in-seconds=120",
				"--cancel-list-on-mount-seconds=0",
				"--attr-cache-timeout=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
				"--read-only=true",
			},
		},
		"deprecated volume: AccessMode=ReadWriteOnce": {
			volumeMount: radixv1.RadixVolumeMount{
				Type:       radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:       "anyvol",
				Path:       "anypath",
				Storage:    "anystorage",
				AccessMode: "ReadWriteOnce",
			},
			expectedMountOptions: []string{
				"--file-cache-timeout-in-seconds=120",
				"--cancel-list-on-mount-seconds=0",
				"--attr-cache-timeout=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
			},
		},
		"deprecated volume: AccessMode=ReadWriteMany": {
			volumeMount: radixv1.RadixVolumeMount{
				Type:       radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:       "anyvol",
				Path:       "anypath",
				Storage:    "anystorage",
				AccessMode: "ReadWriteOnce",
			},
			expectedMountOptions: []string{
				"--file-cache-timeout-in-seconds=120",
				"--cancel-list-on-mount-seconds=0",
				"--attr-cache-timeout=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
			},
		},
		"deprecated volume: GID set": {
			volumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "anyvol",
				Path:    "anypath",
				Storage: "anystorage",
				GID:     "1337",
			},
			expectedMountOptions: []string{
				"--file-cache-timeout-in-seconds=120",
				"--cancel-list-on-mount-seconds=0",
				"--attr-cache-timeout=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
				"--read-only=true",
				"-o gid=1337",
			},
		},
		"deprecated volume: UID set": {
			volumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "anyvol",
				Path:    "anypath",
				Storage: "anystorage",
				UID:     "1337",
			},
			expectedMountOptions: []string{
				"--file-cache-timeout-in-seconds=120",
				"--cancel-list-on-mount-seconds=0",
				"--attr-cache-timeout=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
				"--read-only=true",
				"-o uid=1337",
			},
		},
		"blobfuse2: default settings": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyvol",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
				},
			},
			expectedMountOptions: []string{
				"--disable-writeback-cache=true",
				"--cancel-list-on-mount-seconds=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
				"--use-adls=false",
				"--read-only=true",
				"--attr-cache-timeout=0",
				"--block-cache",
				"--block-cache-block-size=4",
				"--block-cache-pool-size=44",
				"--block-cache-prefetch=11",
				"--block-cache-prefetch-on-open=false",
				"--block-cache-parallelism=8",
			},
		},
		"blobfuse2: AttributeCacheOptions": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyvol",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
					AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{
						Timeout: pointers.Ptr[uint32](1337),
					},
				},
			},
			expectedMountOptions: []string{
				"--disable-writeback-cache=true",
				"--cancel-list-on-mount-seconds=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
				"--use-adls=false",
				"--read-only=true",
				"--block-cache",
				"--block-cache-block-size=4",
				"--block-cache-pool-size=44",
				"--block-cache-prefetch=11",
				"--block-cache-prefetch-on-open=false",
				"--block-cache-parallelism=8",
				"--attr-cache-timeout=1337",
			},
		},
		"blobfuse2: streaming enabled=false should use file cache": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyvol",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
					StreamingOptions: &radixv1.BlobFuse2StreamingOptions{
						Enabled: pointers.Ptr(false),
					},
				},
			},
			expectedMountOptions: []string{
				"--disable-writeback-cache=true",
				"--cancel-list-on-mount-seconds=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
				"--use-adls=false",
				"--read-only=true",
				"--attr-cache-timeout=0",
				"--file-cache-timeout=120",
			},
		},
		"blobfuse2: CacheMode=DirectIO": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyvol",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeDirectIO),
				},
			},
			expectedMountOptions: []string{
				"--disable-writeback-cache=true",
				"--cancel-list-on-mount-seconds=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
				"--use-adls=false",
				"--read-only=true",
				"--attr-cache-timeout=0",
				"-o direct_io",
			},
		},
		"blobfuse2: CacheMode=File": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyvol",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeFile),
				},
			},
			expectedMountOptions: []string{
				"--disable-writeback-cache=true",
				"--cancel-list-on-mount-seconds=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
				"--use-adls=false",
				"--read-only=true",
				"--attr-cache-timeout=0",
				"--file-cache-timeout=120",
			},
		},
		"blobfuse2: CacheMode=File, set Timeout": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyvol",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeFile),
					FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{
						Timeout: pointers.Ptr[uint32](1337),
					},
				},
			},
			expectedMountOptions: []string{
				"--disable-writeback-cache=true",
				"--cancel-list-on-mount-seconds=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
				"--use-adls=false",
				"--read-only=true",
				"--attr-cache-timeout=0",
				"--file-cache-timeout=1337",
			},
		},
		"blobfuse2: CacheMode=Block default settings": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyvol",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
				},
			},
			expectedMountOptions: []string{
				"--disable-writeback-cache=true",
				"--cancel-list-on-mount-seconds=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
				"--use-adls=false",
				"--read-only=true",
				"--attr-cache-timeout=0",
				"--block-cache",
				"--block-cache-block-size=4",
				"--block-cache-pool-size=44",
				"--block-cache-prefetch=11",
				"--block-cache-prefetch-on-open=false",
				"--block-cache-parallelism=8",
			},
		},
		"blobfuse2: CacheMode=Block, set BlockSize and PrefetchCount affects default block-cache-pool-size": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyvol",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](16),
						PrefetchCount: pointers.Ptr[uint32](20),
					},
				},
			},
			expectedMountOptions: []string{
				"--disable-writeback-cache=true",
				"--cancel-list-on-mount-seconds=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
				"--use-adls=false",
				"--read-only=true",
				"--attr-cache-timeout=0",
				"--block-cache",
				"--block-cache-prefetch-on-open=false",
				"--block-cache-parallelism=8",
				"--block-cache-block-size=16",
				"--block-cache-pool-size=320",
				"--block-cache-prefetch=20",
			},
		},
		"blobfuse2: CacheMode=Block, setting PoolSize below minimum required should use calclated min value": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyvol",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](16),
						PoolSize:      pointers.Ptr[uint32](319),
						PrefetchCount: pointers.Ptr[uint32](20),
					},
				},
			},
			expectedMountOptions: []string{
				"--disable-writeback-cache=true",
				"--cancel-list-on-mount-seconds=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
				"--use-adls=false",
				"--read-only=true",
				"--attr-cache-timeout=0",
				"--block-cache",
				"--block-cache-prefetch-on-open=false",
				"--block-cache-parallelism=8",
				"--block-cache-block-size=16",
				"--block-cache-pool-size=320",
				"--block-cache-prefetch=20",
			},
		},
		"blobfuse2: CacheMode=Block, setting PoolSize above minimum required": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyvol",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](16),
						PoolSize:      pointers.Ptr[uint32](321),
						PrefetchCount: pointers.Ptr[uint32](20),
					},
				},
			},
			expectedMountOptions: []string{
				"--disable-writeback-cache=true",
				"--cancel-list-on-mount-seconds=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
				"--use-adls=false",
				"--read-only=true",
				"--attr-cache-timeout=0",
				"--block-cache",
				"--block-cache-prefetch-on-open=false",
				"--block-cache-parallelism=8",
				"--block-cache-block-size=16",
				"--block-cache-pool-size=321",
				"--block-cache-prefetch=20",
			},
		},
		"blobfuse2: CacheMode=Block, setting PrefetchCount to 0 should set pool size=block size": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyvol",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](16),
						PrefetchCount: pointers.Ptr[uint32](0),
					},
				},
			},
			expectedMountOptions: []string{
				"--disable-writeback-cache=true",
				"--cancel-list-on-mount-seconds=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
				"--use-adls=false",
				"--read-only=true",
				"--attr-cache-timeout=0",
				"--block-cache",
				"--block-cache-prefetch-on-open=false",
				"--block-cache-parallelism=8",
				"--block-cache-block-size=16",
				"--block-cache-pool-size=16",
				"--block-cache-prefetch=0",
			},
		},
		"blobfuse2: CacheMode=Block, set PrefetchOnOpen": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyvol",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						PrefetchOnOpen: pointers.Ptr(true),
					},
				},
			},
			expectedMountOptions: []string{
				"--disable-writeback-cache=true",
				"--cancel-list-on-mount-seconds=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
				"--use-adls=false",
				"--read-only=true",
				"--attr-cache-timeout=0",
				"--block-cache",
				"--block-cache-block-size=4",
				"--block-cache-pool-size=44",
				"--block-cache-prefetch=11",
				"--block-cache-parallelism=8",
				"--block-cache-prefetch-on-open=true",
			},
		},
		"blobfuse2: CacheMode=Block, set PrefetchOnOpenx": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyvol",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						Parallelism: pointers.Ptr[uint32](1337),
					},
				},
			},
			expectedMountOptions: []string{
				"--disable-writeback-cache=true",
				"--cancel-list-on-mount-seconds=0",
				"--allow-other",
				"--attr-timeout=0",
				"--entry-timeout=0",
				"--negative-timeout=0",
				"--use-adls=false",
				"--read-only=true",
				"--attr-cache-timeout=0",
				"--block-cache",
				"--block-cache-block-size=4",
				"--block-cache-pool-size=44",
				"--block-cache-prefetch=11",
				"--block-cache-prefetch-on-open=false",
				"--block-cache-parallelism=1337",
			},
		},
		// TODO: Add test for blobfuse2, blockcache disk setting. Testing of disk path (random value) must be performed some other way
	}

	for testName, test := range tests {
		s.Run(testName, func() {
			const (
				appName       = "anyapp"
				envName       = "anyenv"
				componentName = "anycomp"
				namespace     = "anyns"
				pvcName       = "anypvc"
			)
			kubeClient := kubefake.NewClientset()
			rd := &radixv1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "anyrd", Namespace: namespace},
				Spec:       radixv1.RadixDeploymentSpec{AppName: appName, Environment: envName},
			}
			component := &radixv1.RadixDeployComponent{
				Name:         componentName,
				VolumeMounts: []radixv1.RadixVolumeMount{test.volumeMount},
			}
			volumeName, err := GetVolumeMountVolumeName(&test.volumeMount, componentName)
			s.Require().NoError(err)
			desiredVolumes := []corev1.Volume{{Name: volumeName, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}}}
			_, err = CreateOrUpdateCsiAzureVolumeResourcesForDeployComponent(context.Background(), kubeClient, rd, component, desiredVolumes)
			s.Require().NoError(err)
			pvc, err := kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
			s.Require().NoError(err)
			actualPV, err := kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
			s.Require().NoError(err)
			s.ElementsMatch(test.expectedMountOptions, actualPV.Spec.MountOptions)
		})
	}
}

func (s *volumeMountTestSuite) Test_RadixVolumeMountPVLabels() {
	tests := map[string]struct {
		volumeMount    radixv1.RadixVolumeMount
		appName        string
		componentName  string
		namespace      string
		expectedLabels map[string]string
	}{
		"deprecated volume": {
			volumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "vol1",
				Path:    "anypath",
				Storage: "anystorage",
			},
			appName:       "app1",
			componentName: "comp1",
			namespace:     "ns1",
			expectedLabels: map[string]string{
				kube.RadixAppLabel:             "app1",
				kube.RadixComponentLabel:       "comp1",
				kube.RadixNamespace:            "ns1",
				kube.RadixVolumeMountNameLabel: "vol1",
			},
		},
		"blofuse2": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "vol2",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
				},
			},
			appName:       "app2",
			componentName: "comp2",
			namespace:     "ns2",
			expectedLabels: map[string]string{
				kube.RadixAppLabel:             "app2",
				kube.RadixComponentLabel:       "comp2",
				kube.RadixNamespace:            "ns2",
				kube.RadixVolumeMountNameLabel: "vol2",
			},
		},
	}

	for testName, test := range tests {
		s.Run(testName, func() {
			const (
				pvcName = "anypvc"
			)
			kubeClient := kubefake.NewClientset()
			rd := &radixv1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "anyrd", Namespace: test.namespace},
				Spec:       radixv1.RadixDeploymentSpec{AppName: test.appName, Environment: "anyenv"},
			}
			component := &radixv1.RadixDeployComponent{Name: test.componentName, VolumeMounts: []radixv1.RadixVolumeMount{test.volumeMount}}
			volumeName, err := GetVolumeMountVolumeName(&test.volumeMount, test.componentName)
			s.Require().NoError(err)
			desiredVolumes := []corev1.Volume{{Name: volumeName, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}}}
			_, err = CreateOrUpdateCsiAzureVolumeResourcesForDeployComponent(context.Background(), kubeClient, rd, component, desiredVolumes)
			s.Require().NoError(err)
			pvc, err := kubeClient.CoreV1().PersistentVolumeClaims(test.namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
			s.Require().NoError(err)
			actualPV, err := kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
			s.Require().NoError(err)
			s.Equal(test.expectedLabels, actualPV.GetLabels())
		})
	}
}

func (s *volumeMountTestSuite) Test_RadixVolumeMountPVAndPVCRecreateOnChange() {
	tests := map[string]struct {
		initialVolumeMount radixv1.RadixVolumeMount
		changedVolumeMount radixv1.RadixVolumeMount
		expectRecreate     bool
	}{
		"deprecated volume: same spec should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "any",
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "any",
			},
			expectRecreate: false,
		},
		"deprecated volume: change Storage should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "initial",
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "changed",
			},
			expectRecreate: true,
		},
		"deprecated volume: change GID recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "any",
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "any",
				GID:     "1337",
			},

			expectRecreate: true,
		},
		"deprecated volume: change UID recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "any",
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "any",
				UID:     "1337",
			},
			expectRecreate: true,
		},
		"deprecated volume: change RequestsStorage recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "any",
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Type:            radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:            "any",
				Storage:         "any",
				RequestsStorage: resource.MustParse("1337M"),
			},
			expectRecreate: true,
		},
		"deprecated volume: setting AccessMode to the defined default should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "any",
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Type:       radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:       "any",
				Storage:    "any",
				AccessMode: "ReadOnlyMany",
			},
			expectRecreate: false,
		},
		"deprecated volume: setting AccessMode to a non-default value should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "any",
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Type:       radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:       "any",
				Storage:    "any",
				AccessMode: "ReadWriteMany",
			},
			expectRecreate: true,
		},
		"change from deprecated volume to blobfuse2 to recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "any",
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: same spec should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: change Container recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "initial",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "change",
				},
			},
			expectRecreate: true,
		},
	}

	for testName, test := range tests {
		s.Run(testName, func() {
			const (
				appName       = "anyapp"
				componentName = "anycomp"
				namespace     = "anyns"
				pvcName       = "anypvc"
			)
			kubeClient := kubefake.NewClientset()
			rd := &radixv1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "anyrd", Namespace: namespace},
				Spec:       radixv1.RadixDeploymentSpec{AppName: appName, Environment: "anyenv"},
			}

			// Run sync with initialVolumeMount to create initial PV and PVC
			initialComponent := &radixv1.RadixDeployComponent{Name: componentName, VolumeMounts: []radixv1.RadixVolumeMount{test.initialVolumeMount}}
			initialVolumeName, err := GetVolumeMountVolumeName(&test.initialVolumeMount, componentName)
			s.Require().NoError(err)
			initialVolumes := []corev1.Volume{{
				Name:         initialVolumeName,
				VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}},
			}}
			initialVolumes, err = CreateOrUpdateCsiAzureVolumeResourcesForDeployComponent(context.Background(), kubeClient, rd, initialComponent, initialVolumes)
			s.Require().NoError(err)
			initialPVC, err := kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), initialVolumes[0].PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
			s.Require().NoError(err)
			initialPV, err := kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), initialPVC.Spec.VolumeName, metav1.GetOptions{})
			s.Require().NoError(err)
			s.NotNil(initialPV)

			// Run sync with changedVolumeMount to verify recreation of new PV and PVC
			changedComponent := &radixv1.RadixDeployComponent{Name: componentName, VolumeMounts: []radixv1.RadixVolumeMount{test.changedVolumeMount}}

			// The next four lines is a HACK because the generated volume name includes info from the RadixVolumeMount that relates to the driver and external storage container. Why? Don't know
			changedVolumeName, err := GetVolumeMountVolumeName(&test.changedVolumeMount, componentName)
			s.Require().NoError(err)
			initialVolumeModified := initialVolumes[0].DeepCopy()
			initialVolumeModified.Name = changedVolumeName

			changedVolumes, err := CreateOrUpdateCsiAzureVolumeResourcesForDeployComponent(context.Background(), kubeClient, rd, changedComponent, []corev1.Volume{*initialVolumeModified})
			s.Require().NoError(err)
			changedPVC, err := kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), changedVolumes[0].PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
			s.Require().NoError(err)
			changedPV, err := kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), changedPVC.Spec.VolumeName, metav1.GetOptions{})
			s.Require().NoError(err)

			// Perform tests
			pvcList, err := kubeClient.CoreV1().PersistentVolumeClaims(namespace).List(context.Background(), metav1.ListOptions{})
			s.Require().NoError(err)
			pvList, err := kubeClient.CoreV1().PersistentVolumes().List(context.Background(), metav1.ListOptions{})
			s.Require().NoError(err)

			if test.expectRecreate {
				s.NotEqual(initialPVC.Name, changedPVC.Name)
				s.NotEqual(initialPV.Name, changedPV.Name)
				s.Len(pvcList.Items, 2)
				s.Len(pvList.Items, 2)
			} else {
				s.Equal(initialPVC.Name, changedPVC.Name)
				s.Equal(initialPV.Name, changedPV.Name)
				s.Len(pvcList.Items, 1)
				s.Len(pvList.Items, 1)
			}
		})
	}
}

// func (s *volumeMountTestSuite) Test_CreateOrUpdateCsiAzureResources() {
// 	var (
// 		anotherComponentName   = strings.ToLower(utils.RandString(10))
// 		anotherVolumeMountName = strings.ToLower(utils.RandString(10))
// 	)

// 	tests := map[string]deploymentVolumesTestScenario{
// 		"Create new volume": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createDeprecatedRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {}),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, nil),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
// 					},
// 					existingPvs: []corev1.PersistentVolume{},
// 					expectedPvs: []corev1.PersistentVolume{
// 						createExpectedPv(props, func(pv *corev1.PersistentVolume) {}),
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobVolume1Storage1(nil))
// 		}(),
// 		"Update storage in existing volume name and storage": func() deploymentVolumesTestScenario {
// 			type scenarioProperties struct {
// 				changedNewRadixVolumeName        string
// 				changedNewRadixVolumeStorageName string
// 				expectedVolumeName               string
// 				expectedNewSecretName            string
// 				expectedNewPvcName               string
// 				expectedNewPvName                string
// 			}
// 			getScenario := func(props expectedPvcPvProperties, scenarioProps scenarioProperties) deploymentVolumesTestScenario {
// 				existingPv := createExpectedPv(props, func(pv *corev1.PersistentVolume) {})
// 				existingPvc := createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {})
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createDeprecatedRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {
// 							vm.Name = scenarioProps.changedNewRadixVolumeName
// 							vm.Storage = scenarioProps.changedNewRadixVolumeStorageName
// 						}),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, func(v *corev1.Volume) {
// 							v.Name = scenarioProps.expectedVolumeName
// 						}),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 					},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 						createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {
// 							pvc.ObjectMeta.Name = scenarioProps.expectedNewPvcName
// 							pvc.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = scenarioProps.changedNewRadixVolumeName
// 							pvc.Spec.VolumeName = scenarioProps.expectedNewPvName
// 						}),
// 					},
// 					existingPvs: []corev1.PersistentVolume{
// 						existingPv,
// 					},
// 					expectedPvs: []corev1.PersistentVolume{
// 						existingPv,
// 						createExpectedPv(props, func(pv *corev1.PersistentVolume) {
// 							pv.ObjectMeta.Name = scenarioProps.expectedNewPvName
// 							pv.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = scenarioProps.changedNewRadixVolumeName
// 							setVolumeMountAttribute(pv, props.radixVolumeMountType, scenarioProps.changedNewRadixVolumeStorageName)
// 							pv.Spec.ClaimRef.Name = scenarioProps.expectedNewPvcName
// 							pv.Spec.CSI.NodeStageSecretRef.Name = scenarioProps.expectedNewSecretName
// 						}),
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobVolume1Storage1(nil), scenarioProperties{
// 				changedNewRadixVolumeName:        "volume101",
// 				changedNewRadixVolumeStorageName: "storage101",
// 				expectedVolumeName:               "csi-az-blob-some-component-volume101-storage101",
// 				expectedNewSecretName:            "some-component-volume101-csiazurecreds",
// 				expectedNewPvcName:               "pvc-csi-az-blob-some-component-volume101-storage101-12345",
// 				expectedNewPvName:                "pv-radixvolumemount-some-uuid",
// 			})

// 		}(),
// 		"Set readonly volume": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				existingPvc := createRandomPvc(props, props.namespace, props.componentName)
// 				expectedPvc := createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {
// 					pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
// 				})
// 				existingPv := createRandomPv(props, props.namespace, props.componentName)
// 				expectedPv := createExpectedPv(props, nil)
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createDeprecatedRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) { vm.AccessMode = string(corev1.ReadOnlyMany) }),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, func(v *corev1.Volume) {}),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 					},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 						expectedPvc,
// 					},
// 					existingPvs: []corev1.PersistentVolume{
// 						existingPv,
// 					},
// 					expectedPvs: []corev1.PersistentVolume{
// 						existingPv,
// 						expectedPv,
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobVolume1Storage1(func(props *expectedPvcPvProperties) {
// 				props.readOnly = false
// 			}))
// 		}(),
// 		"Set ReadWriteOnce volume": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				existingPvc := createExpectedPvc(props, nil)
// 				existingPv := createExpectedPv(props, nil)
// 				matchPvAndPvc(&existingPv, &existingPvc)
// 				expectedPv := modifyPv(existingPv, func(pv *corev1.PersistentVolume) {
// 					pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
// 					pv.Spec.MountOptions = getMountOptions(modifyProps(props, func(props *expectedPvcPvProperties) { props.readOnly = false }))
// 				})
// 				expectedPvc := modifyPvc(existingPvc, func(pvc *corev1.PersistentVolumeClaim) {
// 					pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
// 				})
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createDeprecatedRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) { vm.AccessMode = string(corev1.ReadWriteOnce) }),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, func(v *corev1.Volume) {}),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 					},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 						expectedPvc,
// 					},
// 					existingPvs: []corev1.PersistentVolume{
// 						existingPv,
// 					},
// 					expectedPvs: []corev1.PersistentVolume{
// 						existingPv,
// 						expectedPv,
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobVolume1Storage1(nil))
// 		}(),
// 		"Set ReadWriteMany volume": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				existingPvc := createExpectedPvc(props, nil)
// 				existingPv := createExpectedPv(props, nil)
// 				matchPvAndPvc(&existingPv, &existingPvc)
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createDeprecatedRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) { vm.AccessMode = string(corev1.ReadWriteMany) }),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, func(v *corev1.Volume) {}),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 					},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 						modifyPvc(existingPvc, func(pvc *corev1.PersistentVolumeClaim) {
// 							pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
// 						}),
// 					},
// 					existingPvs: []corev1.PersistentVolume{
// 						existingPv,
// 					},
// 					expectedPvs: []corev1.PersistentVolume{
// 						existingPv,
// 						modifyPv(existingPv, func(pv *corev1.PersistentVolume) {
// 							pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
// 							pv.Spec.MountOptions = getMountOptions(modifyProps(props, func(props *expectedPvcPvProperties) { props.readOnly = false }))
// 						}),
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobVolume1Storage1(nil))
// 		}(),
// 		"Set ReadOnlyMany volume": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				existingPvc := createExpectedPvc(props, nil)
// 				existingPv := createExpectedPv(props, nil)
// 				matchPvAndPvc(&existingPv, &existingPvc)
// 				existingPv = modifyPv(existingPv, func(pv *corev1.PersistentVolume) {
// 					pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
// 					pv.Spec.MountOptions = getMountOptions(modifyProps(props, func(props *expectedPvcPvProperties) { props.readOnly = false }))
// 				})
// 				existingPvc = modifyPvc(existingPvc, func(pvc *corev1.PersistentVolumeClaim) {
// 					pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
// 				})
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createDeprecatedRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) { vm.AccessMode = string(corev1.ReadOnlyMany) }),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, func(v *corev1.Volume) {}),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 					},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 						modifyPvc(existingPvc, func(pvc *corev1.PersistentVolumeClaim) {
// 							pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
// 						}),
// 					},
// 					existingPvs: []corev1.PersistentVolume{
// 						existingPv,
// 					},
// 					expectedPvs: []corev1.PersistentVolume{
// 						existingPv,
// 						modifyPv(existingPv, func(pv *corev1.PersistentVolume) {
// 							pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
// 							pv.Spec.MountOptions = getMountOptions(modifyProps(props, func(props *expectedPvcPvProperties) { props.readOnly = true }))
// 						}),
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobVolume1Storage1(nil))
// 		}(),
// 		"Create new BlobFuse2 volume has streaming by default and streaming options not set": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {}),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, func(v *corev1.Volume) {}),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
// 					},
// 					existingPvs: []corev1.PersistentVolume{},
// 					expectedPvs: []corev1.PersistentVolume{
// 						createExpectedPv(props, func(pv *corev1.PersistentVolume) {
// 							pv.Spec.MountOptions = getMountOptions(props, "--streaming=true", "--block-cache-pool-size=750")
// 						}),
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobFuse2Volume1Storage1(nil))
// 		}(),
// 		"Create new BlobFuse2 volume has implicit streaming by default and streaming options set": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {
// 							vm.BlobFuse2.StreamingOptions = &radixv1.BlobFuse2StreamingOptions{
// 								StreamCache:      pointers.Ptr(uint64(101)),
// 								BlockSize:        pointers.Ptr(uint64(102)),
// 								BufferSize:       pointers.Ptr(uint64(103)),
// 								MaxBuffers:       pointers.Ptr(uint64(104)),
// 								MaxBlocksPerFile: pointers.Ptr(uint64(105)),
// 							}
// 						}),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, func(v *corev1.Volume) {}),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
// 					},
// 					existingPvs: []corev1.PersistentVolume{},
// 					expectedPvs: []corev1.PersistentVolume{
// 						createExpectedPv(props, func(pv *corev1.PersistentVolume) {
// 							pv.Spec.MountOptions = getMountOptions(props,
// 								"--streaming=true",
// 								"--block-cache-pool-size=101",
// 							)
// 						}),
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobFuse2Volume1Storage1(nil))
// 		}(),
// 		"Create new BlobFuse2 volume has disabled streaming": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {
// 							vm.BlobFuse2.StreamingOptions = &radixv1.BlobFuse2StreamingOptions{
// 								Enabled:          pointers.Ptr(false),
// 								StreamCache:      pointers.Ptr(uint64(101)),
// 								BlockSize:        pointers.Ptr(uint64(102)),
// 								BufferSize:       pointers.Ptr(uint64(103)),
// 								MaxBuffers:       pointers.Ptr(uint64(104)),
// 								MaxBlocksPerFile: pointers.Ptr(uint64(105)),
// 							}
// 						}),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, func(v *corev1.Volume) {}),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
// 					},
// 					existingPvs: []corev1.PersistentVolume{},
// 					expectedPvs: []corev1.PersistentVolume{
// 						createExpectedPv(props, func(pv *corev1.PersistentVolume) {
// 							pv.Spec.MountOptions = getMountOptions(props)
// 						}),
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobFuse2Volume1Storage1(nil))
// 		}(),
// 		"Do not change existing PersistentVolume with class name, when creating new PVC": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				pvForAnotherComponent := createRandomAutoProvisionedPvWithStorageClass(props, props.namespace, anotherComponentName, anotherVolumeMountName)
// 				pvcForAnotherComponent := createRandomAutoProvisionedPvcWithStorageClass(props, props.namespace, anotherComponentName, anotherVolumeMountName)
// 				matchPvAndPvc(&pvForAnotherComponent, &pvcForAnotherComponent)
// 				volume := createTestVolume(props, func(v *corev1.Volume) {})
// 				existingPv := createAutoProvisionedPvWithStorageClass(props, func(pv *corev1.PersistentVolume) { pv.Spec.ClaimRef.Name = volume.PersistentVolumeClaim.ClaimName })
// 				expectedPvc := createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {})
// 				expectedPv := createExpectedPv(props, func(pv *corev1.PersistentVolume) {})
// 				matchPvAndPvc(&expectedPv, &expectedPvc)
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createRandomVolumeMount(func(vm *radixv1.RadixVolumeMount) { vm.Name = anotherVolumeMountName }),
// 						createDeprecatedRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {}),
// 					},
// 					volumes: []corev1.Volume{
// 						volume,
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{
// 						pvcForAnotherComponent,
// 					},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						expectedPvc,
// 						pvcForAnotherComponent,
// 					},
// 					existingPvs: []corev1.PersistentVolume{
// 						existingPv,
// 						pvForAnotherComponent,
// 					},
// 					expectedPvs: []corev1.PersistentVolume{
// 						expectedPv,
// 						existingPv,
// 						pvForAnotherComponent,
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobVolume1Storage1(nil))
// 		}(),
// 		"Do not change existing PersistentVolume without class name, when creating new PVC": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				pvForAnotherComponent := createRandomPv(props, props.namespace, anotherComponentName)
// 				pvcForAnotherComponent := createRandomPvc(props, props.namespace, anotherComponentName)
// 				matchPvAndPvc(&pvForAnotherComponent, &pvcForAnotherComponent)
// 				existingPv := createExpectedPv(props, func(pv *corev1.PersistentVolume) {})
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createDeprecatedRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {}),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, func(v *corev1.Volume) {}),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{
// 						pvcForAnotherComponent,
// 					},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {}),
// 						pvcForAnotherComponent,
// 					},
// 					existingPvs: []corev1.PersistentVolume{
// 						existingPv,
// 						pvForAnotherComponent,
// 					},
// 					expectedPvs: []corev1.PersistentVolume{
// 						existingPv,
// 						pvForAnotherComponent,
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobVolume1Storage1(nil))
// 		}(),
// 		"Do not change existing PVC with class name, when creating new PersistentVolume": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				pvForAnotherComponent := createRandomAutoProvisionedPvWithStorageClass(props, props.namespace, anotherComponentName, anotherVolumeMountName)
// 				pvcForAnotherComponent := createRandomAutoProvisionedPvcWithStorageClass(props, props.namespace, anotherComponentName, anotherVolumeMountName)
// 				matchPvAndPvc(&pvForAnotherComponent, &pvcForAnotherComponent)
// 				existingPvc := createExpectedPvc(props, func(pvc *corev1.PersistentVolumeClaim) {})
// 				expectedPvc := createRandomPvc(props, props.namespace, componentName1)
// 				expectedPv := createRandomPv(props, props.namespace, componentName1)
// 				matchPvAndPvc(&expectedPv, &expectedPvc)
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createDeprecatedRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {}),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, func(v *corev1.Volume) {}),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{
// 						pvcForAnotherComponent,
// 						existingPvc,
// 					},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						pvcForAnotherComponent,
// 						expectedPvc,
// 					},
// 					existingPvs: []corev1.PersistentVolume{
// 						pvForAnotherComponent,
// 					},
// 					expectedPvs: []corev1.PersistentVolume{
// 						pvForAnotherComponent,
// 						expectedPv,
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobVolume1Storage1(nil))
// 		}(),
// 		"Create PV for existing PVC without PV name": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				existingPvc := createRandomAutoProvisionedPvcWithStorageClass(props, props.namespace, props.componentName, props.radixVolumeMountName)
// 				existingPvc.Spec.VolumeName = "" //auto-provisioned PVCs have empty volume name
// 				expectedPvc := createRandomPvc(props, props.namespace, componentName1)
// 				expectedPv := createRandomPv(props, props.namespace, componentName1)
// 				matchPvAndPvc(&expectedPv, &expectedPvc)
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createDeprecatedRadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {}),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, func(v *corev1.Volume) {
// 							v.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
// 								ClaimName: existingPvc.Name,
// 							}
// 						}),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 					},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 						expectedPvc,
// 					},
// 					existingPvs: []corev1.PersistentVolume{},
// 					expectedPvs: []corev1.PersistentVolume{
// 						expectedPv,
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobVolume1Storage1(nil))
// 		}(),
// 		"Create PV and PVC with useAzureIdentity": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				expectedPvc := createRandomPvc(props, props.namespace, componentName1)
// 				expectedPv := createExpectedPvWithIdentity(props, nil)
// 				matchPvAndPvc(&expectedPv, &expectedPvc)
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {
// 							vm.BlobFuse2.UseAzureIdentity = pointers.Ptr(true)
// 							vm.BlobFuse2.StorageAccount = props.storageAccountName
// 							vm.BlobFuse2.SubscriptionId = props.subscriptionId
// 							vm.BlobFuse2.ResourceGroup = props.resourceGroup
// 							vm.BlobFuse2.UseAdls = nil
// 							vm.BlobFuse2.StreamingOptions = &radixv1.BlobFuse2StreamingOptions{Enabled: pointers.Ptr(false)}
// 						}),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, func(v *corev1.Volume) {}),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						expectedPvc,
// 					},
// 					existingPvs: []corev1.PersistentVolume{},
// 					expectedPvs: []corev1.PersistentVolume{
// 						expectedPv,
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobVolume1Storage1(func(props *expectedPvcPvProperties) {
// 				setPropsForBlob2Fuse2AzureIdentity(props)
// 			}))
// 		}(),
// 		"Not changed PV and PVC with useAzureIdentity": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				existingPvc := createRandomPvc(props, props.namespace, componentName1)
// 				existingPv := createExpectedPvWithIdentity(props, nil)
// 				matchPvAndPvc(&existingPv, &existingPvc)
// 				expectedPvc := createRandomPvc(props, props.namespace, componentName1)
// 				expectedPv := createExpectedPvWithIdentity(props, nil)
// 				matchPvAndPvc(&expectedPv, &expectedPvc)
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {
// 							setVolumeMountPropsForBlob2Fuse2AzureIdentity(props, vm)
// 						}),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, func(v *corev1.Volume) {
// 							v.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
// 								ClaimName: existingPvc.Name,
// 							}
// 						}),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 					},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						expectedPvc,
// 					},
// 					existingPvs: []corev1.PersistentVolume{
// 						existingPv,
// 					},
// 					expectedPvs: []corev1.PersistentVolume{
// 						expectedPv,
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobVolume1Storage1(func(props *expectedPvcPvProperties) {
// 				setPropsForBlob2Fuse2AzureIdentity(props)
// 			}))
// 		}(),
// 		"Created PV and PVC with useAzureIdentity on changed storage account": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				existingPvc := createRandomPvc(props, props.namespace, componentName1)
// 				existingPv := createExpectedPvWithIdentity(props, nil)
// 				matchPvAndPvc(&existingPv, &existingPvc)
// 				changedProps := modifyProps(props, func(props *expectedPvcPvProperties) {
// 					props.storageAccountName = testChangedStorageAccountName
// 				})
// 				expectedPvc := createRandomPvc(changedProps, props.namespace, componentName1)
// 				expectedPv := createExpectedPvWithIdentity(changedProps, nil)
// 				matchPvAndPvc(&expectedPv, &expectedPvc)
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {
// 							setVolumeMountPropsForBlob2Fuse2AzureIdentity(props, vm)
// 							vm.BlobFuse2.StorageAccount = testChangedStorageAccountName
// 						}),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, func(v *corev1.Volume) {
// 							v.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
// 								ClaimName: existingPvc.Name,
// 							}
// 						}),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 					},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 						expectedPvc,
// 					},
// 					existingPvs: []corev1.PersistentVolume{
// 						existingPv,
// 					},
// 					expectedPvs: []corev1.PersistentVolume{
// 						existingPv,
// 						expectedPv,
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobVolume1Storage1(func(props *expectedPvcPvProperties) {
// 				setPropsForBlob2Fuse2AzureIdentity(props)
// 			}))
// 		}(),
// 		"Created PV and PVC with useAzureIdentity on changed resource group": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				existingPvc := createRandomPvc(props, props.namespace, componentName1)
// 				existingPv := createExpectedPvWithIdentity(props, nil)
// 				matchPvAndPvc(&existingPv, &existingPvc)
// 				changedProps := modifyProps(props, func(props *expectedPvcPvProperties) {
// 					props.resourceGroup = testChangedResourceGroup
// 				})
// 				expectedPvc := createRandomPvc(changedProps, props.namespace, componentName1)
// 				expectedPv := createExpectedPvWithIdentity(changedProps, nil)
// 				matchPvAndPvc(&expectedPv, &expectedPvc)
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {
// 							setVolumeMountPropsForBlob2Fuse2AzureIdentity(props, vm)
// 							vm.BlobFuse2.ResourceGroup = testChangedResourceGroup
// 						}),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, func(v *corev1.Volume) {
// 							v.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
// 								ClaimName: existingPvc.Name,
// 							}
// 						}),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 					},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 						expectedPvc,
// 					},
// 					existingPvs: []corev1.PersistentVolume{
// 						existingPv,
// 					},
// 					expectedPvs: []corev1.PersistentVolume{
// 						existingPv,
// 						expectedPv,
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobVolume1Storage1(func(props *expectedPvcPvProperties) {
// 				setPropsForBlob2Fuse2AzureIdentity(props)
// 			}))
// 		}(),
// 		"Created PV and PVC with useAzureIdentity on changed subscription id": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				existingPvc := createRandomPvc(props, props.namespace, componentName1)
// 				existingPv := createExpectedPvWithIdentity(props, nil)
// 				matchPvAndPvc(&existingPv, &existingPvc)
// 				changedProps := modifyProps(props, func(props *expectedPvcPvProperties) {
// 					props.subscriptionId = testChangedSubscriptionId
// 				})
// 				expectedPvc := createRandomPvc(changedProps, props.namespace, componentName1)
// 				expectedPv := createExpectedPvWithIdentity(changedProps, nil)
// 				matchPvAndPvc(&expectedPv, &expectedPvc)
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {
// 							setVolumeMountPropsForBlob2Fuse2AzureIdentity(props, vm)
// 							vm.BlobFuse2.SubscriptionId = testChangedSubscriptionId
// 						}),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, func(v *corev1.Volume) {
// 							v.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
// 								ClaimName: existingPvc.Name,
// 							}
// 						}),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 					},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 						expectedPvc,
// 					},
// 					existingPvs: []corev1.PersistentVolume{
// 						existingPv,
// 					},
// 					expectedPvs: []corev1.PersistentVolume{
// 						existingPv,
// 						expectedPv,
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobVolume1Storage1(func(props *expectedPvcPvProperties) {
// 				setPropsForBlob2Fuse2AzureIdentity(props)
// 			}))
// 		}(),
// 		"Created PV and PVC with useAzureIdentity on changed tenant id": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				existingPvc := createRandomPvc(props, props.namespace, componentName1)
// 				existingPv := createExpectedPvWithIdentity(props, nil)
// 				matchPvAndPvc(&existingPv, &existingPvc)
// 				changedProps := modifyProps(props, func(props *expectedPvcPvProperties) {
// 					props.tenantId = testChangedTenantId
// 				})
// 				expectedPvc := createRandomPvc(changedProps, props.namespace, componentName1)
// 				expectedPv := createExpectedPvWithIdentity(changedProps, nil)
// 				matchPvAndPvc(&expectedPv, &expectedPvc)
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {
// 							setVolumeMountPropsForBlob2Fuse2AzureIdentity(props, vm)
// 							vm.BlobFuse2.TenantId = testChangedTenantId
// 						}),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, func(v *corev1.Volume) {
// 							v.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
// 								ClaimName: existingPvc.Name,
// 							}
// 						}),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 					},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 						expectedPvc,
// 					},
// 					existingPvs: []corev1.PersistentVolume{
// 						existingPv,
// 					},
// 					expectedPvs: []corev1.PersistentVolume{
// 						existingPv,
// 						expectedPv,
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobVolume1Storage1(func(props *expectedPvcPvProperties) {
// 				setPropsForBlob2Fuse2AzureIdentity(props)
// 				props.tenantId = testTenantId
// 			}))
// 		}(),
// 		"Changed PV and PVC from useAzureIdentity to use a Client Secret": func() deploymentVolumesTestScenario {
// 			getScenario := func(props expectedPvcPvProperties) deploymentVolumesTestScenario {
// 				existingPvc := createRandomPvc(props, props.namespace, componentName1)
// 				existingPv := createExpectedPvWithIdentity(props, nil)
// 				matchPvAndPvc(&existingPv, &existingPvc)
// 				changedProps := modifyProps(props, func(props *expectedPvcPvProperties) {
// 					props.tenantId = testChangedTenantId
// 				})
// 				expectedPvc := createRandomPvc(changedProps, props.namespace, componentName1)
// 				expectedPv := createExpectedPv(changedProps, nil)
// 				matchPvAndPvc(&expectedPv, &expectedPvc)
// 				return deploymentVolumesTestScenario{
// 					radixVolumeMounts: []radixv1.RadixVolumeMount{
// 						createBlobFuse2RadixVolumeMount(props, func(vm *radixv1.RadixVolumeMount) {}),
// 					},
// 					volumes: []corev1.Volume{
// 						createTestVolume(props, func(v *corev1.Volume) {
// 							v.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
// 								ClaimName: existingPvc.Name,
// 							}
// 						}),
// 					},
// 					existingPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 					},
// 					expectedPvcs: []corev1.PersistentVolumeClaim{
// 						existingPvc,
// 						expectedPvc,
// 					},
// 					existingPvs: []corev1.PersistentVolume{
// 						existingPv,
// 					},
// 					expectedPvs: []corev1.PersistentVolume{
// 						existingPv,
// 						expectedPv,
// 					},
// 				}
// 			}
// 			return getScenario(getPropsCsiBlobVolume1Storage1(func(props *expectedPvcPvProperties) {
// 				setPropsForBlob2Fuse2AzureIdentity(props)
// 			}))
// 		}(),
// 	}
// 	for testName, test := range tests {
// 		s.Run(testName, func() {
// 			testEnv := getTestEnv()
// 			radixDeployment := buildRd(appName1, envName1, componentName1, testClientId, test.radixVolumeMounts)
// 			putExistingDeploymentVolumesScenarioDataToFakeCluster(testEnv.kubeUtil.KubeClient(), &test)
// 			desiredVolumes := getDesiredDeployment(componentName1, test.volumes).Spec.Template.Spec.Volumes

// 			deployComponent := radixDeployment.Spec.Components[0]
// 			actualVolumes, err := CreateOrUpdateCsiAzureVolumeResourcesForDeployComponent(context.Background(), testEnv.kubeUtil.KubeClient(), radixDeployment, &deployComponent, desiredVolumes)
// 			s.Require().NoError(err)
// 			s.Equal(len(test.volumes), len(actualVolumes), "Number of volumes is not equal")

// 			existingPvcs, existingPvs, err := getExistingPvcsAndPersistentVolumeFromFakeCluster(testEnv.kubeUtil.KubeClient())
// 			s.Require().NoError(err)
// 			s.Len(existingPvcs, len(test.expectedPvcs), "PVC-s count is not equal")
// 			s.True(equalPersistentVolumeClaims(&test.expectedPvcs, &existingPvcs), "PVC-s are not equal")
// 			s.Len(existingPvs, len(test.expectedPvs), "PV-s count is not equal")
// 			s.True(equalPersistentVolumes(&test.expectedPvs, &existingPvs), "PV-s are not equal")
// 		})
// 	}
// }

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
