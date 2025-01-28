package volumemount

import (
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"k8s.io/api/core/v1"
	"testing"
)

func Test_EqualPersistentVolumes(t *testing.T) {
	createPv := func(modify func(pv *v1.PersistentVolume)) *v1.PersistentVolume {
		pv := createExpectedPv(getPropsCsiBlobVolume1Storage1(nil), modify)
		return &pv
	}
	tests := []struct {
		name     string
		pv1      *v1.PersistentVolume
		pv2      *v1.PersistentVolume
		expected bool
	}{
		{
			name:     "both nil",
			pv1:      nil,
			pv2:      nil,
			expected: false,
		},
		{
			name:     "one nil",
			pv1:      createPv(nil),
			pv2:      nil,
			expected: false,
		},
		{
			name:     "equal",
			pv1:      createPv(nil),
			pv2:      createPv(nil),
			expected: true,
		},
		{
			name: "different access mode",
			pv1: createPv(func(pv *v1.PersistentVolume) {
				pv.Spec.AccessModes = []v1.PersistentVolumeAccessMode{v1.ReadWriteMany}
			}),
			pv2: createPv(func(pv *v1.PersistentVolume) {
				pv.Spec.AccessModes = []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany}
			}),
			expected: false,
		},
		{
			name: "no access mode",
			pv1:  createPv(func(pv *v1.PersistentVolume) { pv.Spec.AccessModes = nil }),
			pv2: createPv(func(pv *v1.PersistentVolume) {
				pv.Spec.AccessModes = []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany}
			}),
			expected: false,
		},
		{
			name:     "no ClaimRef",
			pv1:      createPv(nil),
			pv2:      createPv(func(pv *v1.PersistentVolume) { pv.Spec.ClaimRef = nil }),
			expected: false,
		},
		{
			name:     "different ClaimRef name",
			pv1:      createPv(nil),
			pv2:      createPv(func(pv *v1.PersistentVolume) { pv.Spec.ClaimRef.Name = utils.RandString(10) }),
			expected: false,
		},
		{
			name:     "different ClaimRef namespace",
			pv1:      createPv(nil),
			pv2:      createPv(func(pv *v1.PersistentVolume) { pv.Spec.ClaimRef.Namespace = utils.RandString(10) }),
			expected: false,
		},
		{
			name:     "different ClaimRef kind",
			pv1:      createPv(nil),
			pv2:      createPv(func(pv *v1.PersistentVolume) { pv.Spec.ClaimRef.Kind = "secret" }),
			expected: false,
		},
		{
			name:     "no CSI",
			pv1:      createPv(nil),
			pv2:      createPv(func(pv *v1.PersistentVolume) { pv.Spec.CSI = nil }),
			expected: false,
		},
		{
			name:     "no CSI VolumeAttributes",
			pv1:      createPv(nil),
			pv2:      createPv(func(pv *v1.PersistentVolume) { pv.Spec.CSI.VolumeAttributes = nil }),
			expected: false,
		},
		{
			name: "different CSI VolumeAttribute csiVolumeMountAttributeContainerName",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *v1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributeContainerName] = utils.RandString(10)
			}),
			expected: false,
		},
		{
			name: "different CSI VolumeAttribute csiVolumeMountAttributeProtocol",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *v1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributeProtocol] = utils.RandString(10)
			}),
			expected: false,
		},
		{
			name: "ignore different CSI VolumeAttribute csiVolumeMountAttributePvName",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *v1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributePvName] = utils.RandString(10)
			}),
			expected: true,
		},
		{
			name: "ignore different CSI VolumeAttribute csiVolumeMountAttributePvcName",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *v1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributePvcName] = utils.RandString(10)
			}),
			expected: true,
		},
		{
			name: "ignore different CSI VolumeAttribute csiVolumeMountAttributeProvisionerIdentity",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *v1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributeProvisionerIdentity] = utils.RandString(10)
			}),
			expected: true,
		},
		{
			name: "different CSI VolumeAttribute csiVolumeMountAttributePvcNamespace",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *v1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributePvcNamespace] = utils.RandString(10)
			}),
			expected: false,
		},
		{
			name: "different CSI VolumeAttribute csiVolumeMountAttributeSecretNamespace",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *v1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributeSecretNamespace] = utils.RandString(10)
			}),
			expected: false,
		},
		{
			name: "extra CSI VolumeAttribute",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *v1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes["some-extra-attribute"] = utils.RandString(10)
			}),
			expected: false,
		},
		{
			name: "different CSI Driver",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *v1.PersistentVolume) {
				pv.Spec.CSI.Driver = utils.RandString(10)
			}),
			expected: false,
		},
		{
			name: "no CSI NodeStageSecretRef",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *v1.PersistentVolume) {
				pv.Spec.CSI.NodeStageSecretRef = nil
			}),
			expected: false,
		},
		{
			name: "different CSI NodeStageSecretRef",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *v1.PersistentVolume) {
				pv.Spec.CSI.NodeStageSecretRef.Name = utils.RandString(10)
			}),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EqualPersistentVolumes(tt.pv1, tt.pv2); got != tt.expected {
				t.Errorf("EqualPersistentVolumes() = %v, want %v", got, tt.expected)
			}
		})
	}
}
