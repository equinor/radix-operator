package volumemount_test

import (
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/volumemount"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_ComparePersistentVolumeClaims(t *testing.T) {
	validPVC := func(modify func(pvc *corev1.PersistentVolumeClaim)) *corev1.PersistentVolumeClaim {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: v1.ObjectMeta{
				Namespace: "any",
				Labels: map[string]string{
					"any": "any",
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Limits:   corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2")},
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1")},
				},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				VolumeMode:  pointers.Ptr(corev1.PersistentVolumeFilesystem),
			},
		}
		if modify != nil {
			modify(pvc)
		}
		return pvc
	}

	tests := map[string]struct {
		pvc1        *corev1.PersistentVolumeClaim
		pvc2        *corev1.PersistentVolumeClaim
		expectEqual bool
	}{
		"base pvc1 and pvc2 are equal": {
			pvc1:        validPVC(nil),
			pvc2:        validPVC(nil),
			expectEqual: true,
		},
		"pvc1 is nil": {
			pvc1:        nil,
			pvc2:        validPVC(nil),
			expectEqual: false,
		},
		"pvc2 is nil": {
			pvc1:        validPVC(nil),
			pvc2:        nil,
			expectEqual: false,
		},
		"pvc1 and pvc2 are nil": {
			pvc1:        nil,
			pvc2:        nil,
			expectEqual: false,
		},
		"different namespace": {
			pvc1: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Namespace = "pvc1"
			}),
			pvc2: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Namespace = "pvc2"
			}),
			expectEqual: false,
		},
		"different resource requests": {
			pvc1: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.Resources = corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1M")},
				}
			}),
			pvc2: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.Resources = corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2M")},
				}
			}),
			expectEqual: false,
		},
		"different resource limits": {
			pvc1: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.Resources = corev1.VolumeResourceRequirements{
					Limits: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1M")},
				}
			}),
			pvc2: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.Resources = corev1.VolumeResourceRequirements{
					Limits: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2M")},
				}
			}),
			expectEqual: false,
		},
		"resource requests nil and empty should be equal": {
			pvc1: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.Resources.Requests = nil
			}),
			pvc2: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.Resources.Requests = corev1.ResourceList{}
			}),
			expectEqual: true,
		},
		"resource limits nil and empty should be equal": {
			pvc1: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.Resources.Limits = nil
			}),
			pvc2: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.Resources.Limits = corev1.ResourceList{}
			}),
			expectEqual: true,
		},
		"different accessmodes": {
			pvc1: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany, corev1.ReadWriteMany}
			}),
			pvc2: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany, corev1.ReadWriteOnce}
			}),
			expectEqual: false,
		},
		"same accessmodes in different order should be equal": {
			pvc1: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany, corev1.ReadWriteMany}
			}),
			pvc2: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany, corev1.ReadOnlyMany}
			}),
			expectEqual: true,
		},
		"accessmodes nil and empty should be equal": {
			pvc1: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.AccessModes = nil
			}),
			pvc2: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{}
			}),
			expectEqual: true,
		},
		"different volumemode": {
			pvc1: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.VolumeMode = pointers.Ptr(corev1.PersistentVolumeBlock)
			}),
			pvc2: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.VolumeMode = pointers.Ptr(corev1.PersistentVolumeFilesystem)
			}),
			expectEqual: false,
		},
		"volumemode nil on pv1 and pv2 should be equal": {
			pvc1: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.VolumeMode = nil
			}),
			pvc2: validPVC(func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.VolumeMode = nil
			}),
			expectEqual: true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			actualEqual := volumemount.ComparePersistentVolumeClaims(test.pvc1, test.pvc2)
			assert.Equal(t, test.expectEqual, actualEqual)
		})
	}
}
