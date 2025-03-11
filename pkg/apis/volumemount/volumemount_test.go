//nolint:staticcheck
package volumemount

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/internal"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type volumeMountTestSuite struct {
	testSuite
}

func TestVolumeMountTestSuite(t *testing.T) {
	suite.Run(t, new(volumeMountTestSuite))
}

func (s *volumeMountTestSuite) Test_NoVolumeMounts() {
	volumeMounts, _ := GetRadixDeployComponentVolumeMounts(&radixv1.RadixDeployComponent{}, "")
	s.Equal(0, len(volumeMounts))
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
	s.Run("One Blob CSI Azure volume mount ", func() {
		s.T().Parallel()
		s.T().Logf("Test case %s", scenarios[0].name)
		component := &radixv1.RadixDeployComponent{Name: "app", VolumeMounts: []radixv1.RadixVolumeMount{scenarios[0].radixVolumeMount}}

		volumeMounts, err := GetRadixDeployComponentVolumeMounts(component, "")
		s.Nil(err)
		s.Equal(1, len(volumeMounts), "Unexpected volume count")
		if len(volumeMounts) > 0 {
			mount := volumeMounts[0]
			s.Less(len(volumeMounts[0].Name), 64, "Volume name is too long")
			s.Equal(scenarios[0].expectedVolumeName, mount.Name, "Mismatching volume names")
			s.Equal(scenarios[0].radixVolumeMount.Path, mount.MountPath, "Mismatching volume paths")
		}

	})
	s.Run("Multiple Blob CSI Azure volume mount ", func() {
		s.T().Parallel()
		s.T().Logf("Test case %s", scenarios[0].name)
		component := &radixv1.RadixDeployComponent{
			Name:         "app",
			VolumeMounts: []radixv1.RadixVolumeMount{scenarios[0].radixVolumeMount, scenarios[1].radixVolumeMount, scenarios[2].radixVolumeMount},
		}
		volumeMounts, err := GetRadixDeployComponentVolumeMounts(component, "")
		s.Equal(3, len(volumeMounts), "Unexpected volume count")
		s.Nil(err)
		for idx, testCase := range scenarios {
			if len(volumeMounts) > 0 {
				volumeMountName := volumeMounts[idx].Name
				s.Less(len(volumeMountName), 64)
				if len(volumeMountName) > 60 {
					s.True(internal.EqualTillPostfix(testCase.expectedVolumeName, volumeMountName, nameRandPartLength), "Mismatching volume name prefixes %s and %s", volumeMountName, testCase.expectedVolumeName)
				} else {
					s.Equal(testCase.expectedVolumeName, volumeMountName, "Mismatching volume names")
				}
				s.Equal(testCase.radixVolumeMount.Path, volumeMounts[idx].MountPath, "Mismatching volume paths")
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
	s.Run("Failing Blob CSI Azure volume mount", func() {
		s.T().Parallel()
		for _, testCase := range scenarios {
			s.T().Logf("Test case %s", testCase.name)
			component := &radixv1.RadixDeployComponent{Name: "app", VolumeMounts: []radixv1.RadixVolumeMount{testCase.radixVolumeMount}}

			_, err := GetRadixDeployComponentVolumeMounts(component, "")
			s.NotNil(err)
			s.Equal(testCase.expectedError, err.Error())
		}
	})
}

func (s *volumeMountTestSuite) Test_GetNewVolumes() {
	namespace := "some-namespace"
	componentName := "some-component"
	s.Run("No volumes in component", func() {
		s.T().Parallel()
		component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts().BuildComponent()
		volumes, err := GetVolumes(context.Background(), s.kubeUtil, namespace, &component, "", nil)
		s.Nil(err)
		s.Len(volumes, 0)
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
	s.Run("CSI Azure volumes", func() {
		s.T().Parallel()

		for _, scenario := range scenarios {
			s.T().Logf("Scenario %s", scenario.name)
			component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts(scenario.radixVolumeMount).BuildComponent()
			volumes, err := GetVolumes(context.Background(), s.kubeUtil, namespace, &component, "", nil)
			s.Nil(err)
			s.Len(volumes, 1, "Unexpected volume count")
			volume := volumes[0]
			if len(volume.Name) > 60 {
				s.True(internal.EqualTillPostfix(scenario.expectedVolumeName, volume.Name, nameRandPartLength), "Mismatching volume name prefixes %s and %s", scenario.expectedVolumeName, volume.Name)
			} else {
				s.Equal(scenario.expectedVolumeName, volume.Name, "Mismatching volume names")
			}
			s.Less(len(volume.Name), 64, "Volume name is too long")
			s.NotNil(volume.PersistentVolumeClaim, "PVC is nil")
			s.True(internal.EqualTillPostfix(scenario.expectedPvcName, volume.PersistentVolumeClaim.ClaimName, nameRandPartLength), "Mismatching PVC name prefixes %s and %s", scenario.expectedPvcName, volume.PersistentVolumeClaim.ClaimName)
		}
	})
	s.Run("Unsupported volume type", func() {
		s.T().Parallel()

		mounts := []radixv1.RadixVolumeMount{
			{Type: "unsupported-type", Name: "volume1", Container: "storage1", Path: "path1"},
		}
		component := utils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts(mounts...).BuildComponent()
		volumes, err := GetVolumes(context.Background(), s.kubeUtil, namespace, &component, "", nil)
		s.Len(volumes, 0, "Unexpected volume count")
		s.NotNil(err)
		s.Equal("unsupported volume type unsupported-type", err.Error())
	})
}

func (s *volumeMountTestSuite) Test_GetVolumesForComponent() {
	const (
		appName       = "any-app"
		environment   = "some-env"
		componentName = "some-component"
	)
	// namespace := fmt.Sprintf("%s-%s", appName, environment)
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
	}

	s.Run("No volumes", func() {
		s.T().Parallel()

		radixDeployment := buildRd(appName, environment, componentName, "", []radixv1.RadixVolumeMount{})
		deployComponent := radixDeployment.Spec.Components[0]

		volumes, err := GetVolumes(context.Background(), s.kubeUtil, radixDeployment.GetNamespace(), &deployComponent, radixDeployment.GetName(), nil)

		s.Nil(err)
		s.Len(volumes, 0, "No volumes should be returned")

	})
	s.Run("Exists volume", func() {
		s.T().Parallel()
		for _, scenario := range scenarios {
			s.T().Logf("Test case %s", scenario.name)

			radixDeployment := buildRd(appName, environment, componentName, "", []radixv1.RadixVolumeMount{scenario.radixVolumeMount})
			deployComponent := radixDeployment.Spec.Components[0]

			volumes, err := GetVolumes(context.Background(), s.kubeUtil, radixDeployment.GetNamespace(), &deployComponent, radixDeployment.GetName(), nil)

			s.Nil(err)
			s.Len(volumes, 1, "Unexpected volume count")
			s.Equal(scenario.expectedVolumeName, volumes[0].Name, "Mismatching volume names")
			s.NotNil(volumes[0].PersistentVolumeClaim, "PVC is nil")
			s.True(internal.EqualTillPostfix(scenario.expectedPvcName, volumes[0].PersistentVolumeClaim.ClaimName, nameRandPartLength), "Mismatching PVC name prefixes %s and %s", scenario.expectedPvcName, volumes[0].PersistentVolumeClaim.ClaimName)
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

	s.Run("No volumes", func() {
		s.T().Parallel()
		radixDeployment := buildRd(appName, environment, componentName, "", []radixv1.RadixVolumeMount{})
		deployComponent := radixDeployment.Spec.Components[0]

		volumes, err := GetRadixDeployComponentVolumeMounts(&deployComponent, "")

		s.Nil(err)
		s.Len(volumes, 0, "No volumes should be returned")
	})
	s.Run("Exists volume", func() {
		s.T().Parallel()
		for _, scenario := range scenarios {
			s.T().Logf("Test case %s", scenario.name)

			radixDeployment := buildRd(appName, environment, componentName, "", []radixv1.RadixVolumeMount{scenario.radixVolumeMount})
			deployComponent := radixDeployment.Spec.Components[0]

			volumeMounts, err := GetRadixDeployComponentVolumeMounts(&deployComponent, "")

			s.Nil(err)
			s.Len(volumeMounts, 1)
			volumeMountName := volumeMounts[0].Name
			if len(volumeMountName) > 60 {
				s.True(internal.EqualTillPostfix(scenario.expectedVolumeName, volumeMountName, nameRandPartLength), "Mismatching volume name prefixes %s and %s", scenario.expectedVolumeName, volumeMountName)
			} else {
				s.Equal(scenario.expectedVolumeName, volumeMountName)
			}
			s.Less(len(volumeMountName), 64, "Volume name is too long")
			s.Equal(scenario.radixVolumeMount.Path, volumeMounts[0].MountPath, "Mismatching volume paths")
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

			rd := &radixv1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "anyrd", Namespace: namespace},
				Spec:       radixv1.RadixDeploymentSpec{AppName: appName, Environment: envName},
			}
			component := &radixv1.RadixDeployComponent{Name: componentName, VolumeMounts: []radixv1.RadixVolumeMount{test.volumeMount}}
			volumeName, err := GetVolumeMountVolumeName(&test.volumeMount, componentName)
			s.Require().NoError(err)
			desiredVolumes := []corev1.Volume{{Name: volumeName, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}}}
			actualVolumes, err := CreateOrUpdatePVCVolumeResourcesForDeployComponent(context.Background(), s.kubeClient, rd, component, desiredVolumes)
			s.Require().NoError(err)
			s.Require().Len(actualVolumes, 1)
			s.Equal(pvcName, actualVolumes[0].VolumeSource.PersistentVolumeClaim.ClaimName)
			pvc, err := s.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
			s.Require().NoError(err)
			_, err = s.kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
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
			rd := &radixv1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "anyrd", Namespace: namespace},
				Spec:       radixv1.RadixDeploymentSpec{AppName: appName, Environment: envName},
			}
			component := &radixv1.RadixDeployComponent{Name: componentName, VolumeMounts: []radixv1.RadixVolumeMount{test.volumeMount}}
			volumeName, err := GetVolumeMountVolumeName(&test.volumeMount, componentName)
			s.Require().NoError(err)
			desiredVolumes := []corev1.Volume{{Name: volumeName, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}}}
			_, err = CreateOrUpdatePVCVolumeResourcesForDeployComponent(context.Background(), s.kubeClient, rd, component, desiredVolumes)
			s.Require().NoError(err)
			actualPVC, err := s.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
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
			rd := &radixv1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "anyrd", Namespace: namespace},
				Spec:       radixv1.RadixDeploymentSpec{AppName: test.appName, Environment: envName},
			}
			component := &radixv1.RadixDeployComponent{Name: test.componentName, VolumeMounts: []radixv1.RadixVolumeMount{test.volumeMount}}
			volumeName, err := GetVolumeMountVolumeName(&test.volumeMount, test.componentName)
			s.Require().NoError(err)
			desiredVolumes := []corev1.Volume{{Name: volumeName, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}}}
			_, err = CreateOrUpdatePVCVolumeResourcesForDeployComponent(context.Background(), s.kubeClient, rd, component, desiredVolumes)
			s.Require().NoError(err)
			pvc, err := s.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
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
			_, err = CreateOrUpdatePVCVolumeResourcesForDeployComponent(context.Background(), s.kubeClient, rd, component, desiredVolumes)
			s.Require().NoError(err)
			pvc, err := s.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
			s.Require().NoError(err)
			actualPV, err := s.kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
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
		volumeMount                     radixv1.RadixVolumeMount
		expectedMountOptions            []string
		expectedMountOptionsArgNameOnly []string
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
		"blobfuse2: CacheMode=Block, setting PoolSize below minimum required should use calculated min value": {
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
		"blobfuse2: CacheMode=Block, set Parallelism": {
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
		"blobfuse2: CacheMode=Block, enable disk cache with default values": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyvol",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						DiskSize: pointers.Ptr[uint32](1337),
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
				"--block-cache-parallelism=8",
				"--block-cache-disk-size=1337",
				"--block-cache-disk-timeout=120",
			},
			expectedMountOptionsArgNameOnly: []string{
				"--block-cache-path",
			},
		},
		"blobfuse2: CacheMode=Block, disk cache with custom timeout": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyvol",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						DiskSize:    pointers.Ptr[uint32](1337),
						DiskTimeout: pointers.Ptr[uint32](99),
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
				"--block-cache-parallelism=8",
				"--block-cache-disk-size=1337",
				"--block-cache-disk-timeout=99",
			},
			expectedMountOptionsArgNameOnly: []string{
				"--block-cache-path",
			},
		},
		"blobfuse2: CacheMode=Block, set disk size below minimum should use calculated min value": {
			volumeMount: radixv1.RadixVolumeMount{
				Name: "anyvol",
				Path: "anypath",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "anycontainer",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](16),
						PrefetchCount: pointers.Ptr[uint32](20),
						DiskSize:      pointers.Ptr[uint32](319),
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
				"--block-cache-block-size=16",
				"--block-cache-pool-size=320",
				"--block-cache-prefetch=20",
				"--block-cache-prefetch-on-open=false",
				"--block-cache-parallelism=8",
				"--block-cache-disk-size=320",
				"--block-cache-disk-timeout=120",
			},
			expectedMountOptionsArgNameOnly: []string{
				"--block-cache-path",
			},
		},
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
			_, err = CreateOrUpdatePVCVolumeResourcesForDeployComponent(context.Background(), s.kubeClient, rd, component, desiredVolumes)
			s.Require().NoError(err)
			pvc, err := s.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
			s.Require().NoError(err)
			actualPV, err := s.kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
			s.Require().NoError(err)
			s.Subset(actualPV.Spec.MountOptions, test.expectedMountOptions)
			actualMountOptionArgNames := slice.Map(actualPV.Spec.MountOptions, func(o string) string {
				return strings.Split(o, "=")[0]
			})
			s.Subset(actualMountOptionArgNames, test.expectedMountOptionsArgNameOnly)
			s.Equal(len(test.expectedMountOptions)+len(test.expectedMountOptionsArgNameOnly), len(actualPV.Spec.MountOptions))
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
			rd := &radixv1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "anyrd", Namespace: test.namespace},
				Spec:       radixv1.RadixDeploymentSpec{AppName: test.appName, Environment: "anyenv"},
			}
			component := &radixv1.RadixDeployComponent{Name: test.componentName, VolumeMounts: []radixv1.RadixVolumeMount{test.volumeMount}}
			volumeName, err := GetVolumeMountVolumeName(&test.volumeMount, test.componentName)
			s.Require().NoError(err)
			desiredVolumes := []corev1.Volume{{Name: volumeName, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}}}
			_, err = CreateOrUpdatePVCVolumeResourcesForDeployComponent(context.Background(), s.kubeClient, rd, component, desiredVolumes)
			s.Require().NoError(err)
			pvc, err := s.kubeClient.CoreV1().PersistentVolumeClaims(test.namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
			s.Require().NoError(err)
			actualPV, err := s.kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
			s.Require().NoError(err)
			s.Equal(test.expectedLabels, actualPV.GetLabels())
		})
	}
}

func (s *volumeMountTestSuite) Test_RadixVolumeMountPVAndPVCRecreateOnChange() {
	tests := map[string]struct {
		initialVolumeMount radixv1.RadixVolumeMount
		initialIdentity    *radixv1.Identity
		changedVolumeMount radixv1.RadixVolumeMount
		changedIdentity    *radixv1.Identity
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
		"deprecated volume: set GID should recreate PV and PVC": {
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
		"deprecated volume: change GID should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "any",
				GID:     "1000",
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "any",
				GID:     "1337",
			},

			expectRecreate: true,
		},
		"deprecated volume: keep GID should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "any",
				GID:     "1337",
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "any",
				GID:     "1337",
			},

			expectRecreate: false,
		},
		"deprecated volume: set UID should recreate PV and PVC": {
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
		"deprecated volume: change UID should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "any",
				UID:     "1000",
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "any",
				UID:     "1337",
			},
			expectRecreate: true,
		},
		"deprecated volume: keep UID should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "any",
				UID:     "1337",
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:    "any",
				Storage: "any",
				UID:     "1337",
			},
			expectRecreate: false,
		},
		"deprecated volume: set RequestsStorage should recreate PV and PVC": {
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
		"deprecated volume: change RequestsStorage should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Type:            radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:            "any",
				Storage:         "any",
				RequestsStorage: resource.MustParse("100M"),
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Type:            radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:            "any",
				Storage:         "any",
				RequestsStorage: resource.MustParse("1337M"),
			},
			expectRecreate: true,
		},
		"deprecated volume: keep RequestsStorage should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Type:            radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:            "any",
				Storage:         "any",
				RequestsStorage: resource.MustParse("1337M"),
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Type:            radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:            "any",
				Storage:         "any",
				RequestsStorage: resource.MustParse("1337M"),
			},
			expectRecreate: false,
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
		"deprecated volume: change AccessMode should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Type:       radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:       "any",
				Storage:    "any",
				AccessMode: "ReadWriteOnce",
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Type:       radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:       "any",
				Storage:    "any",
				AccessMode: "ReadWriteMany",
			},
			expectRecreate: true,
		},
		"deprecated volume: keep AccessMode should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Type:       radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:       "any",
				Storage:    "any",
				AccessMode: "ReadWriteOnce",
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Type:       radixv1.MountTypeBlobFuse2FuseCsiAzure,
				Name:       "any",
				Storage:    "any",
				AccessMode: "ReadWriteOnce",
			},
			expectRecreate: false,
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
		"blobfuse2: same default spec should not recreate PV and PVC": {
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
		"blobfuse2: change Container should recreate PV and PVC": {
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
		"blobfuse2: set GID should recreate PV and PVC": {
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
					GID:       "1337",
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: change GID should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					GID:       "1000",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					GID:       "1337",
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: keep GID should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					GID:       "1337",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					GID:       "1337",
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set UID should recreate PV and PVC": {
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
					UID:       "1337",
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: change UID should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					UID:       "1000",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					UID:       "1337",
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: keep UID should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					UID:       "1337",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					UID:       "1337",
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set RequestsStorage should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:       "any",
					RequestsStorage: resource.MustParse("1337M"),
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: change RequestsStorage should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:       "any",
					RequestsStorage: resource.MustParse("1000M"),
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:       "any",
					RequestsStorage: resource.MustParse("1337M"),
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: keep RequestsStorage should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:       "any",
					RequestsStorage: resource.MustParse("1337M"),
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:       "any",
					RequestsStorage: resource.MustParse("1337M"),
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: setting AccessMode to the defined default should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:  "any",
					AccessMode: "ReadOnlyMany",
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: change AccessMode should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:  "any",
					AccessMode: "ReadWriteMany",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:  "any",
					AccessMode: "ReadWriteOnce",
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: keep AccessMode should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:  "any",
					AccessMode: "ReadWriteMany",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:  "any",
					AccessMode: "ReadWriteMany",
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: setting UseAdls to the defined default should not recreate PV and PVC": {
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
					UseAdls:   pointers.Ptr(false),
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: change UseAdls should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					UseAdls:   pointers.Ptr(false),
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					UseAdls:   pointers.Ptr(true),
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: set StorageAccount should recreate PV and PVCx": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:      "any",
					StorageAccount: "changed",
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: change StorageAccount should recreate PV and PVCx": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:      "any",
					StorageAccount: "initial",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:      "any",
					StorageAccount: "changed",
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: set UseAzureIdentity to the defined default should not recreate PV and PVCx": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "any",
					UseAzureIdentity: pointers.Ptr(false),
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set UseAzureIdentity to non-default should recreate PV and PVCx": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "any",
					UseAzureIdentity: pointers.Ptr(true),
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: set ResourceGroup when UseAzureIdentity=false should not recreate PV and PVCx": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "any",
					UseAzureIdentity: pointers.Ptr(false),
					ResourceGroup:    "initial",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "any",
					UseAzureIdentity: pointers.Ptr(false),
					ResourceGroup:    "change",
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: change ResourceGroup when UseAzureIdentity=true should recreate PV and PVCx": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "any",
					UseAzureIdentity: pointers.Ptr(true),
					ResourceGroup:    "initial",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "any",
					UseAzureIdentity: pointers.Ptr(true),
					ResourceGroup:    "change",
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: change SubscriptionId when UseAzureIdentity=false should not recreate PV and PVCx": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "any",
					UseAzureIdentity: pointers.Ptr(false),
					SubscriptionId:   "initial",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "any",
					UseAzureIdentity: pointers.Ptr(false),
					SubscriptionId:   "change",
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: change SubscriptionId when UseAzureIdentity=true should recreate PV and PVCx": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "any",
					UseAzureIdentity: pointers.Ptr(true),
					SubscriptionId:   "initial",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "any",
					UseAzureIdentity: pointers.Ptr(true),
					SubscriptionId:   "change",
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: change TenantId when UseAzureIdentity=false should not recreate PV and PVCx": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "any",
					UseAzureIdentity: pointers.Ptr(false),
					TenantId:         "initial",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "any",
					UseAzureIdentity: pointers.Ptr(false),
					TenantId:         "change",
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: change TenantId when UseAzureIdentity=true should recreate PV and PVCx": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "any",
					UseAzureIdentity: pointers.Ptr(true),
					SubscriptionId:   "initial",
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "any",
					UseAzureIdentity: pointers.Ptr(true),
					SubscriptionId:   "change",
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: change Identity.Azure.ClientID when UseAzureIdentity=false should not recreate PV and PVCx": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "any",
					UseAzureIdentity: pointers.Ptr(false),
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "any",
					UseAzureIdentity: pointers.Ptr(false),
				},
			},
			initialIdentity: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "initial"}},
			changedIdentity: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "change"}},
			expectRecreate:  false,
		},
		"blobfuse2: change Identity.Azure.ClientID when UseAzureIdentity=true should recreate PV and PVCx": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "any",
					UseAzureIdentity: pointers.Ptr(true),
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container:        "any",
					UseAzureIdentity: pointers.Ptr(true),
				},
			},
			initialIdentity: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "initial"}},
			changedIdentity: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "change"}},
			expectRecreate:  true,
		},
		"blobfuse2: set AttributeCacheOptions.Timeout to the defined default should not recreate PV and PVC": {
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
					AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{
						Timeout: pointers.Ptr[uint32](0),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set AttributeCacheOptions.Timeout to a non-default value should recreate PV and PVC": {
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
					AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{
						Timeout: pointers.Ptr[uint32](1337),
					},
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: change AttributeCacheOptions.Timeout should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{
						Timeout: pointers.Ptr[uint32](1000),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{
						Timeout: pointers.Ptr[uint32](1337),
					},
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: keep AttributeCacheOptions.Timeout should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{
						Timeout: pointers.Ptr[uint32](1337),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{
						Timeout: pointers.Ptr[uint32](1337),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set FileCacheOptions.Timeout to the defined default should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeFile),
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeFile),
					FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{
						Timeout: pointers.Ptr[uint32](120),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set FileCacheOptions.Timeout to a non-default value should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeFile),
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeFile),
					FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{
						Timeout: pointers.Ptr[uint32](1337),
					},
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: change FileCacheOptions.Timeout should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeFile),
					FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{
						Timeout: pointers.Ptr[uint32](1000),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeFile),
					FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{
						Timeout: pointers.Ptr[uint32](1337),
					},
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: keep FileCacheOptions.Timeout should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeFile),
					FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{
						Timeout: pointers.Ptr[uint32](1337),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeFile),
					FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{
						Timeout: pointers.Ptr[uint32](1337),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set BlockCacheOptions.BlockSize to the defined default should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize: pointers.Ptr[uint32](4),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set BlockCacheOptions.BlockSize to a non-default value should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize: pointers.Ptr[uint32](8),
					},
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: change BlockCacheOptions.BlockSize should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize: pointers.Ptr[uint32](16),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize: pointers.Ptr[uint32](8),
					},
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: keep BlockCacheOptions.BlockSize should non recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize: pointers.Ptr[uint32](8),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize: pointers.Ptr[uint32](8),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set BlockCacheOptions.PrefetchCount to the defined default should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						PrefetchCount: pointers.Ptr[uint32](11),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set BlockCacheOptions.PrefetchCount to a non-default value should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						PrefetchCount: pointers.Ptr[uint32](12),
					},
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: change BlockCacheOptions.PrefetchCount should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						PrefetchCount: pointers.Ptr[uint32](0),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						PrefetchCount: pointers.Ptr[uint32](12),
					},
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: keep BlockCacheOptions.PrefetchCount should non recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						PrefetchCount: pointers.Ptr[uint32](12),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						PrefetchCount: pointers.Ptr[uint32](12),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set BlockCacheOptions.Parallelism to the defined default should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						Parallelism: pointers.Ptr[uint32](8),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set BlockCacheOptions.Parallelism to a non-default value should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						Parallelism: pointers.Ptr[uint32](1337),
					},
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: change BlockCacheOptions.Parallelism should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						Parallelism: pointers.Ptr[uint32](1000),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						Parallelism: pointers.Ptr[uint32](1337),
					},
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: keep BlockCacheOptions.Parallelism should non recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						Parallelism: pointers.Ptr[uint32](12),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						Parallelism: pointers.Ptr[uint32](12),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set BlockCacheOptions.PrefetchOnOpen to the defined default should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						PrefetchOnOpen: pointers.Ptr(false),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set BlockCacheOptions.PrefetchOnOpen to a non-default value should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						PrefetchOnOpen: pointers.Ptr(true),
					},
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: change BlockCacheOptions.PrefetchOnOpen should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						PrefetchOnOpen: pointers.Ptr(true),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						PrefetchOnOpen: pointers.Ptr(false),
					},
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: keep BlockCacheOptions.PrefetchOnOpen should non recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						PrefetchOnOpen: pointers.Ptr(true),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						PrefetchOnOpen: pointers.Ptr(true),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set BlockCacheOptions.PoolSize to the calculated minumum should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](8),
						PrefetchCount: pointers.Ptr[uint32](20),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](8),
						PrefetchCount: pointers.Ptr[uint32](20),
						PoolSize:      pointers.Ptr[uint32](160),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set BlockCacheOptions.PoolSize to higher than calculated minumum should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](8),
						PrefetchCount: pointers.Ptr[uint32](20),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](8),
						PrefetchCount: pointers.Ptr[uint32](20),
						PoolSize:      pointers.Ptr[uint32](161),
					},
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: set BlockCacheOptions.PoolSize to lower than calculated minumum should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](8),
						PrefetchCount: pointers.Ptr[uint32](20),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](8),
						PrefetchCount: pointers.Ptr[uint32](20),
						PoolSize:      pointers.Ptr[uint32](159),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: change BlockCacheOptions.PoolSize should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](8),
						PrefetchCount: pointers.Ptr[uint32](20),
						PoolSize:      pointers.Ptr[uint32](1000),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](8),
						PrefetchCount: pointers.Ptr[uint32](20),
						PoolSize:      pointers.Ptr[uint32](1337),
					},
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: keep BlockCacheOptions.PoolSize should non recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](8),
						PrefetchCount: pointers.Ptr[uint32](20),
						PoolSize:      pointers.Ptr[uint32](1337),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](8),
						PrefetchCount: pointers.Ptr[uint32](20),
						PoolSize:      pointers.Ptr[uint32](1337),
					},
				},
			},
			expectRecreate: false,
		},

		"blobfuse2: set BlockCacheOptions.DiskTimeout when DiskSize not set should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						DiskTimeout: pointers.Ptr[uint32](1000),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set BlockCacheOptions.DiskTimeout when DiskSize=0 should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						DiskSize:    pointers.Ptr[uint32](0),
						DiskTimeout: pointers.Ptr[uint32](1000),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set BlockCacheOptions.DiskTimeout to default value when DiskSize set should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						DiskSize: pointers.Ptr[uint32](1),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						DiskSize:    pointers.Ptr[uint32](1),
						DiskTimeout: pointers.Ptr[uint32](120),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set BlockCacheOptions.DiskTimeout to non-default value when DiskSize set should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						DiskSize: pointers.Ptr[uint32](1),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						DiskSize:    pointers.Ptr[uint32](1),
						DiskTimeout: pointers.Ptr[uint32](1337),
					},
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: set BlockCacheOptions.DiskSize lower than calculated minimum value should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](8),
						PrefetchCount: pointers.Ptr[uint32](20),
						DiskSize:      pointers.Ptr[uint32](1),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](8),
						PrefetchCount: pointers.Ptr[uint32](20),
						DiskSize:      pointers.Ptr[uint32](2),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: set BlockCacheOptions.DiskSize higher than calculated minimum value should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](8),
						PrefetchCount: pointers.Ptr[uint32](20),
						DiskSize:      pointers.Ptr[uint32](160),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](8),
						PrefetchCount: pointers.Ptr[uint32](20),
						DiskSize:      pointers.Ptr[uint32](161),
					},
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: change BlockCacheOptions.DiskSize should recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](8),
						PrefetchCount: pointers.Ptr[uint32](20),
						DiskSize:      pointers.Ptr[uint32](1000),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](8),
						PrefetchCount: pointers.Ptr[uint32](20),
						DiskSize:      pointers.Ptr[uint32](1337),
					},
				},
			},
			expectRecreate: true,
		},
		"blobfuse2: keep BlockCacheOptions.DiskSize should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](8),
						PrefetchCount: pointers.Ptr[uint32](20),
						DiskSize:      pointers.Ptr[uint32](1337),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						BlockSize:     pointers.Ptr[uint32](8),
						PrefetchCount: pointers.Ptr[uint32](20),
						DiskSize:      pointers.Ptr[uint32](1337),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: change FileCacheOptions when CacheMode=Block should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{
						Timeout: pointers.Ptr[uint32](1000),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeBlock),
					FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{
						Timeout: pointers.Ptr[uint32](1337),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: change BlockCacheOptions when CacheMode=File should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeFile),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						PoolSize: pointers.Ptr[uint32](1000),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeFile),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						PoolSize: pointers.Ptr[uint32](1337),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: change FileCacheOptions when CacheMode=DirectIO should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeDirectIO),
					FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{
						Timeout: pointers.Ptr[uint32](1000),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeDirectIO),
					FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{
						Timeout: pointers.Ptr[uint32](1337),
					},
				},
			},
			expectRecreate: false,
		},
		"blobfuse2: change BlockCacheOptions when CacheMode=DirectIO should not recreate PV and PVC": {
			initialVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeDirectIO),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						PoolSize: pointers.Ptr[uint32](1000),
					},
				},
			},
			changedVolumeMount: radixv1.RadixVolumeMount{
				Name: "any",
				BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: "any",
					CacheMode: pointers.Ptr(radixv1.BlobFuse2CacheModeDirectIO),
					BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{
						PoolSize: pointers.Ptr[uint32](1337),
					},
				},
			},
			expectRecreate: false,
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
			rd := &radixv1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "anyrd", Namespace: namespace},
				Spec:       radixv1.RadixDeploymentSpec{AppName: appName, Environment: "anyenv"},
			}

			// Run sync with initialVolumeMount to create initial PV and PVC
			initialComponent := &radixv1.RadixDeployComponent{Name: componentName, VolumeMounts: []radixv1.RadixVolumeMount{test.initialVolumeMount}, Identity: test.initialIdentity}
			initialVolumeName, err := GetVolumeMountVolumeName(&test.initialVolumeMount, componentName)
			s.Require().NoError(err)
			initialVolumes := []corev1.Volume{{
				Name:         initialVolumeName,
				VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}},
			}}
			initialVolumes, err = CreateOrUpdatePVCVolumeResourcesForDeployComponent(context.Background(), s.kubeClient, rd, initialComponent, initialVolumes)
			s.Require().NoError(err)
			initialPVC, err := s.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), initialVolumes[0].PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
			s.Require().NoError(err)
			initialPV, err := s.kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), initialPVC.Spec.VolumeName, metav1.GetOptions{})
			s.Require().NoError(err)
			s.NotNil(initialPV)

			// Run sync with changedVolumeMount to verify recreation of new PV and PVC
			changedComponent := &radixv1.RadixDeployComponent{Name: componentName, VolumeMounts: []radixv1.RadixVolumeMount{test.changedVolumeMount}, Identity: test.changedIdentity}

			// The next four lines is a hack because the generated volume name includes info from the RadixVolumeMount that relates to the driver and external storage container.
			changedVolumeName, err := GetVolumeMountVolumeName(&test.changedVolumeMount, componentName)
			s.Require().NoError(err)
			initialVolumeModified := initialVolumes[0].DeepCopy()
			initialVolumeModified.Name = changedVolumeName

			changedVolumes, err := CreateOrUpdatePVCVolumeResourcesForDeployComponent(context.Background(), s.kubeClient, rd, changedComponent, []corev1.Volume{*initialVolumeModified})
			s.Require().NoError(err)
			changedPVC, err := s.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), changedVolumes[0].PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
			s.Require().NoError(err)
			changedPV, err := s.kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), changedPVC.Spec.VolumeName, metav1.GetOptions{})
			s.Require().NoError(err)

			// Perform tests
			pvcList, err := s.kubeClient.CoreV1().PersistentVolumeClaims(namespace).List(context.Background(), metav1.ListOptions{})
			s.Require().NoError(err)
			pvList, err := s.kubeClient.CoreV1().PersistentVolumes().List(context.Background(), metav1.ListOptions{})
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
