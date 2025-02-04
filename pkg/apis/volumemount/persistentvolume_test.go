package volumemount

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
)

type pvTestSuite struct {
	testSuite
}

func TestPvTestSuite(t *testing.T) {
	suite.Run(t, new(pvTestSuite))
}

func (s *pvTestSuite) Test_EqualPersistentVolumes() {
	createPv := func(modify func(pv *corev1.PersistentVolume)) *corev1.PersistentVolume {
		pv := createExpectedPv(getPropsCsiBlobVolume1Storage1(nil), modify)
		return &pv
	}
	createPvWithProps := func(modify func(*expectedPvcPvProperties)) *corev1.PersistentVolume {
		pv := createExpectedPv(getPropsCsiBlobVolume1Storage1(modify), nil)
		return &pv
	}
	tests := []struct {
		name     string
		pv1      *corev1.PersistentVolume
		pv2      *corev1.PersistentVolume
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
			pv1: createPv(func(pv *corev1.PersistentVolume) {
				pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
			}),
			pv2: createPv(func(pv *corev1.PersistentVolume) {
				pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
			}),
			expected: false,
		},
		{
			name: "no access mode",
			pv1:  createPv(func(pv *corev1.PersistentVolume) { pv.Spec.AccessModes = nil }),
			pv2: createPv(func(pv *corev1.PersistentVolume) {
				pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
			}),
			expected: false,
		},
		{
			name:     "no ClaimRef",
			pv1:      createPv(nil),
			pv2:      createPv(func(pv *corev1.PersistentVolume) { pv.Spec.ClaimRef = nil }),
			expected: false,
		},
		{
			name:     "different ClaimRef name",
			pv1:      createPv(nil),
			pv2:      createPv(func(pv *corev1.PersistentVolume) { pv.Spec.ClaimRef.Name = utils.RandString(10) }),
			expected: false,
		},
		{
			name:     "different ClaimRef namespace",
			pv1:      createPv(nil),
			pv2:      createPv(func(pv *corev1.PersistentVolume) { pv.Spec.ClaimRef.Namespace = utils.RandString(10) }),
			expected: false,
		},
		{
			name:     "different ClaimRef kind",
			pv1:      createPv(nil),
			pv2:      createPv(func(pv *corev1.PersistentVolume) { pv.Spec.ClaimRef.Kind = "secret" }),
			expected: false,
		},
		{
			name:     "no CSI",
			pv1:      createPv(nil),
			pv2:      createPv(func(pv *corev1.PersistentVolume) { pv.Spec.CSI = nil }),
			expected: false,
		},
		{
			name:     "no CSI VolumeAttributes",
			pv1:      createPv(nil),
			pv2:      createPv(func(pv *corev1.PersistentVolume) { pv.Spec.CSI.VolumeAttributes = nil }),
			expected: false,
		},
		{
			name: "different CSI VolumeAttribute csiVolumeMountAttributeContainerName",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributeContainerName] = utils.RandString(10)
			}),
			expected: false,
		},
		{
			name: "different CSI VolumeAttribute csiVolumeMountAttributeProtocol",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributeProtocol] = utils.RandString(10)
			}),
			expected: false,
		},
		{
			name: "ignore different CSI VolumeAttribute csiVolumeMountAttributeProvisionerIdentity",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributeProvisionerIdentity] = utils.RandString(10)
			}),
			expected: true,
		},
		{
			name: "different CSI VolumeAttribute csiVolumeMountAttributeSecretNamespace",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributeSecretNamespace] = utils.RandString(10)
			}),
			expected: false,
		},
		{
			name: "extra CSI VolumeAttribute",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes["some-extra-attribute"] = utils.RandString(10)
			}),
			expected: false,
		},
		{
			name: "different CSI Driver",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.Driver = utils.RandString(10)
			}),
			expected: false,
		},
		{
			name: "no CSI NodeStageSecretRef",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.NodeStageSecretRef = nil
			}),
			expected: false,
		},
		{
			name: "different CSI NodeStageSecretRef",
			pv1:  createPv(nil),
			pv2: createPv(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.NodeStageSecretRef.Name = utils.RandString(10)
			}),
			expected: false,
		},
		{
			name: "different namespace",
			pv1:  createPv(nil),
			pv2: createPvWithProps(func(props *expectedPvcPvProperties) {
				props.namespace = utils.RandString(10)
			}),
			expected: false,
		},
		{
			name: "different blobStorageName",
			pv1:  createPv(nil),
			pv2: createPvWithProps(func(props *expectedPvcPvProperties) {
				props.blobStorageName = utils.RandString(10)
			}),
			expected: false,
		},
		{
			name: "different pvGid",
			pv1:  createPv(nil),
			pv2: createPvWithProps(func(props *expectedPvcPvProperties) {
				props.pvGid = "7779"
			}),
			expected: false,
		},
		{
			name: "different pvUid",
			pv1: createPvWithProps(func(props *expectedPvcPvProperties) {
				props.pvGid = ""
				props.pvUid = "7779"
			}),
			pv2: createPvWithProps(func(props *expectedPvcPvProperties) {
				props.pvGid = ""
				props.pvUid = "8889"
			}),
			expected: false,
		},
		{
			name: "different pvProvisioner",
			pv1:  createPv(nil),
			pv2: createPvWithProps(func(props *expectedPvcPvProperties) {
				props.pvProvisioner = utils.RandString(10)
			}),
			expected: false,
		},
		{
			name: "different pvSecretName",
			pv1:  createPv(nil),
			pv2: createPvWithProps(func(props *expectedPvcPvProperties) {
				props.pvSecretName = utils.RandString(10)
			}),
			expected: false,
		},
		{
			name: "different readOnly",
			pv1: createPvWithProps(func(props *expectedPvcPvProperties) {
				props.readOnly = true
			}),
			pv2: createPvWithProps(func(props *expectedPvcPvProperties) {
				props.readOnly = false
			}),
			expected: false,
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			if got := EqualPersistentVolumes(tt.pv1, tt.pv2); got != tt.expected {
				s.T().Errorf("EqualPersistentVolumes() = %v, want %v", got, tt.expected)
			}
		})
	}
}
