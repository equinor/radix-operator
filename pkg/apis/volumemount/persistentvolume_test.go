package volumemount_test

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/volumemount"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Test_ComparePersistentVolumes(t *testing.T) {
	validPV := func(modify func(pv *corev1.PersistentVolume)) *corev1.PersistentVolume {
		pv := &corev1.PersistentVolume{
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver:           "anydriver",
						VolumeAttributes: map[string]string{"any": "any"},
						NodeStageSecretRef: &corev1.SecretReference{
							Name:      "anysecret",
							Namespace: "anysecretns",
						},
					},
				},
				MountOptions: []string{"--any=any"},
				Capacity:     corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1")},
				AccessModes:  []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				ClaimRef: &corev1.ObjectReference{
					APIVersion: "anyapi",
					Kind:       "anykind",
					Namespace:  "anypvcns",
					Name:       "anypvc-abcde",
				},
			},
		}
		if modify != nil {
			modify(pv)
		}
		return pv
	}

	tests := map[string]struct {
		pv1         *corev1.PersistentVolume
		pv2         *corev1.PersistentVolume
		expectEqual bool
	}{
		"base pv1 and pv2 are equal": {
			pv1:         validPV(nil),
			pv2:         validPV(nil),
			expectEqual: true,
		},
		"pv1 is nil": {
			pv1:         nil,
			pv2:         validPV(nil),
			expectEqual: false,
		},
		"pv1 and pv2 are nil": {
			pv1:         nil,
			pv2:         nil,
			expectEqual: false,
		},
		"pv2 is nil": {
			pv1:         validPV(nil),
			pv2:         nil,
			expectEqual: false,
		},
		"pv1.Spec.CSI is nil": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI = nil
			}),
			pv2:         validPV(nil),
			expectEqual: false,
		},
		"pv2.Spec.CSI is nil": {
			pv1: validPV(nil),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI = nil
			}),
			expectEqual: false,
		},
		"different volumeattribute value": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes = map[string]string{
					"pv": "pv1",
				}
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes = map[string]string{
					"pv": "pv2",
				}
			}),
			expectEqual: false,
		},
		"different volumeattribute keys": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes = map[string]string{
					"pv1": "any",
				}
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes = map[string]string{
					"pv2": "any",
				}
			}),
			expectEqual: false,
		},
		"ignore different obsolete sctorageclass volumeattributes": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes = map[string]string{
					"csi.storage.k8s.io/pv/name":                   "pv1",
					"csi.storage.k8s.io/pvc/name":                  "pv1",
					"csi.storage.k8s.io/pvc/namespace":             "pv1",
					"storage.kubernetes.io/csiProvisionerIdentity": "pv1",
				}
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.VolumeAttributes = map[string]string{
					"csi.storage.k8s.io/pv/name":                   "pv2",
					"csi.storage.k8s.io/pvc/name":                  "pv2",
					"csi.storage.k8s.io/pvc/namespace":             "pv2",
					"storage.kubernetes.io/csiProvisionerIdentity": "pv2",
				}
			}),
			expectEqual: true,
		},
		"different mountoptions": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.MountOptions = []string{"--arg=pv1"}
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.MountOptions = []string{"--arg=pv2"}
			}),
			expectEqual: false,
		},
		"mountoption --block-cache-path should ignore value": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.MountOptions = []string{"--block-cache-path=pv1"}
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.MountOptions = []string{"--block-cache-path=pv2"}
			}),
			expectEqual: true,
		},
		"order of mountoptions should not matter": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.MountOptions = []string{"--arg1", "--arg2"}
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.MountOptions = []string{"--arg2", "--arg1"}
			}),
			expectEqual: true,
		},
		"different capacity, both set": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1M")}
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2M")}
			}),
			expectEqual: false,
		},
		"different capacity, pv1 not set": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.Capacity = nil
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2M")}
			}),
			expectEqual: false,
		},
		"different capacity, pv2 not set": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2M")}
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.Capacity = nil
			}),
			expectEqual: false,
		},
		"capacity nil and empty map should be equal": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.Capacity = corev1.ResourceList{}
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.Capacity = nil
			}),
			expectEqual: true,
		},
		"different accessmodes": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany, corev1.ReadWriteMany}
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany, corev1.ReadWriteOnce}
			}),
			expectEqual: false,
		},
		"same accessmodes in different order should be equal": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany, corev1.ReadWriteMany}
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany, corev1.ReadOnlyMany}
			}),
			expectEqual: true,
		},
		"accessmodes nil and empty shoul be equal": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.AccessModes = nil
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{}
			}),
			expectEqual: true,
		},
		"different CSI driver": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.Driver = "pv1"
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.Driver = "pv2"
			}),
			expectEqual: false,
		},
		"pv1 claimref is nil": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.ClaimRef = nil
			}),
			pv2:         validPV(nil),
			expectEqual: false,
		},
		"pv2 claimref is nil": {
			pv1: validPV(nil),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.ClaimRef = nil
			}),
			expectEqual: false,
		},
		"bloh claimref is nil": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.ClaimRef = nil
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.ClaimRef = nil
			}),
			expectEqual: true,
		},
		"different claimref name": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.ClaimRef.Name = "pv1-12345"
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.ClaimRef.Name = "pv2-12345"
			}),
			expectEqual: false,
		},
		"different claimref suffix": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.ClaimRef.Name = "pv-12345"
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.ClaimRef.Name = "pv-abcde"
			}),
			expectEqual: true,
		},
		"different claimref namespace": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.ClaimRef.Namespace = "pv1"
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.ClaimRef.Namespace = "pv2"
			}),
			expectEqual: false,
		},
		"different claimref kind": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.ClaimRef.Kind = "pv1"
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.ClaimRef.Kind = "pv2"
			}),
			expectEqual: false,
		},
		"different claimref apiversion": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.ClaimRef.APIVersion = "pv1"
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.ClaimRef.APIVersion = "pv2"
			}),
			expectEqual: false,
		},
		"different claimref UID should be ignored": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.ClaimRef.UID = "pv1"
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.ClaimRef.UID = "pv2"
			}),
			expectEqual: true,
		},
		"different claimref ResourceVersion should be ignored": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.ClaimRef.ResourceVersion = "pv1"
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.ClaimRef.ResourceVersion = "pv2"
			}),
			expectEqual: true,
		},
		"different nodestagesecretref name": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.NodeStageSecretRef.Name = "pv1"
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.NodeStageSecretRef.Name = "pv2"
			}),
			expectEqual: false,
		},
		"different nodestagesecretref namespace": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.NodeStageSecretRef.Namespace = "pv1"
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.NodeStageSecretRef.Namespace = "pv2"
			}),
			expectEqual: false,
		},
		"both nodestagesecretref is nil": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.NodeStageSecretRef = nil
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.NodeStageSecretRef = nil
			}),
			expectEqual: true,
		},
		"nodestagesecretref pv1 is nil and pv2 is empty": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.NodeStageSecretRef = nil
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.NodeStageSecretRef = &corev1.SecretReference{}
			}),
			expectEqual: false,
		},
		"nodestagesecretref pv1 is empty and pv2 is nil": {
			pv1: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.NodeStageSecretRef = &corev1.SecretReference{}
			}),
			pv2: validPV(func(pv *corev1.PersistentVolume) {
				pv.Spec.CSI.NodeStageSecretRef = nil
			}),
			expectEqual: false,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			actualEqual := volumemount.ComparePersistentVolumes(test.pv1, test.pv2)
			assert.Equal(t, test.expectEqual, actualEqual)
		})
	}

}
