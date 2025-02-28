package volumemount

import (
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type pvcTestSuite struct {
	testSuite
}

func TestPvcTestSuite(t *testing.T) {
	suite.Run(t, new(pvcTestSuite))
}

func (s *pvcTestSuite) Test_EqualPersistentVolumeClaims() {
	createPvc := func(modify func(pv *corev1.PersistentVolumeClaim)) *corev1.PersistentVolumeClaim {
		pv := createExpectedPvc(getPropsCsiBlobVolume1Storage1(nil), modify)
		return &pv
	}
	tests := []struct {
		name     string
		pvc1     *corev1.PersistentVolumeClaim
		pvc2     *corev1.PersistentVolumeClaim
		expected bool
	}{
		{
			name:     "both nil",
			pvc1:     nil,
			pvc2:     nil,
			expected: false,
		},
		{
			name:     "one nil",
			pvc1:     createPvc(nil),
			pvc2:     nil,
			expected: false,
		},
		{
			name:     "equal",
			pvc1:     createPvc(nil),
			pvc2:     createPvc(nil),
			expected: true,
		},
		{
			name: "different namespaces",
			pvc1: createPvc(func(pv *corev1.PersistentVolumeClaim) {
				pv.ObjectMeta.Namespace = "namespace1"
			}),
			pvc2: createPvc(func(pv *corev1.PersistentVolumeClaim) {
				pv.ObjectMeta.Namespace = "namespace2"
			}),
			expected: false,
		},
		{
			name: "no storage resource",
			pvc1: createPvc(func(pv *corev1.PersistentVolumeClaim) {
				pv.Spec.Resources.Requests = map[corev1.ResourceName]resource.Quantity{}
			}),
			pvc2: createPvc(func(pv *corev1.PersistentVolumeClaim) {
				pv.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("1M")
			}),
			expected: false,
		},
		{
			name: "different storage resource",
			pvc1: createPvc(func(pv *corev1.PersistentVolumeClaim) {
				pv.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("1M")
			}),
			pvc2: createPvc(func(pv *corev1.PersistentVolumeClaim) {
				pv.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2M")
			}),
			expected: false,
		},
		{
			name: "no access mode",
			pvc1: createPvc(func(pv *corev1.PersistentVolumeClaim) { pv.Spec.AccessModes = nil }),
			pvc2: createPvc(func(pv *corev1.PersistentVolumeClaim) {
				pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
			}),
			expected: false,
		},
		{
			name: "different access mode",
			pvc1: createPvc(func(pv *corev1.PersistentVolumeClaim) {
				pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
			}),
			pvc2: createPvc(func(pv *corev1.PersistentVolumeClaim) {
				pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
			}),
			expected: false,
		},
		{
			name: "no volume mode",
			pvc1: createPvc(func(pv *corev1.PersistentVolumeClaim) { pv.Spec.VolumeMode = nil }),
			pvc2: createPvc(func(pv *corev1.PersistentVolumeClaim) {
				pv.Spec.VolumeMode = pointers.Ptr(corev1.PersistentVolumeBlock)
			}),
			expected: false,
		},
		{
			name: "different volume mode",
			pvc1: createPvc(func(pv *corev1.PersistentVolumeClaim) {
				pv.Spec.VolumeMode = pointers.Ptr(corev1.PersistentVolumeFilesystem)
			}),
			pvc2: createPvc(func(pv *corev1.PersistentVolumeClaim) {
				pv.Spec.VolumeMode = pointers.Ptr(corev1.PersistentVolumeBlock)
			}),
			expected: false,
		},
		{
			name:     "different app name label",
			pvc1:     createPvc(func(pv *corev1.PersistentVolumeClaim) { pv.ObjectMeta.Labels[kube.RadixAppLabel] = "app1" }),
			pvc2:     createPvc(func(pv *corev1.PersistentVolumeClaim) { pv.ObjectMeta.Labels[kube.RadixAppLabel] = "app2" }),
			expected: false,
		},
		{
			name: "different radix component label",
			pvc1: createPvc(func(pv *corev1.PersistentVolumeClaim) {
				pv.ObjectMeta.Labels[kube.RadixComponentLabel] = "componentName1"
			}),
			pvc2: createPvc(func(pv *corev1.PersistentVolumeClaim) {
				pv.ObjectMeta.Labels[kube.RadixComponentLabel] = "componentName2"
			}),
			expected: false,
		},
		{
			name:     "different volume mount name label",
			pvc1:     createPvc(func(pv *corev1.PersistentVolumeClaim) { pv.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = "name1" }),
			pvc2:     createPvc(func(pv *corev1.PersistentVolumeClaim) { pv.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = "name2" }),
			expected: false,
		},
		{
			name:     "different volume mount type label",
			pvc1:     createPvc(func(pv *corev1.PersistentVolumeClaim) { pv.ObjectMeta.Labels[kube.RadixMountTypeLabel] = "type1" }),
			pvc2:     createPvc(func(pv *corev1.PersistentVolumeClaim) { pv.ObjectMeta.Labels[kube.RadixMountTypeLabel] = "type2" }),
			expected: false,
		},
		{
			name:     "extra label",
			pvc1:     createPvc(func(pv *corev1.PersistentVolumeClaim) {}),
			pvc2:     createPvc(func(pv *corev1.PersistentVolumeClaim) { pv.ObjectMeta.Labels["extra-label"] = "extra-value" }),
			expected: false,
		},
		{
			name:     "missing label",
			pvc1:     createPvc(func(pv *corev1.PersistentVolumeClaim) {}),
			pvc2:     createPvc(func(pv *corev1.PersistentVolumeClaim) { delete(pv.ObjectMeta.Labels, kube.RadixAppLabel) }),
			expected: false,
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			if got := ComparePersistentVolumeClaims(tt.pvc1, tt.pvc2); got != tt.expected {
				s.T().Errorf("EqualPersistentVolumeClaims() = %v, want %v", got, tt.expected)
			}
		})
	}
}
