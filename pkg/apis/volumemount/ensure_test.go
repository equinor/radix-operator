//nolint:staticcheck
package volumemount

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	ensureTestAppName       = "anyapp"
	ensureTestEnvName       = "anyenv"
	ensureTestNamespace     = "anyapp-anyenv"
	ensureTestComponentName = "anycomp"
	ensureTestRdName        = "anyrd"
)

type ensureComponentVolumesTestSuite struct {
	testSuite
}

func TestEnsureComponentVolumesTestSuite(t *testing.T) {
	suite.Run(t, new(ensureComponentVolumesTestSuite))
}

func newEnsureTestRd(volumeMounts ...radixv1.RadixVolumeMount) (*radixv1.RadixDeployment, *radixv1.RadixDeployComponent) {
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: ensureTestRdName, Namespace: ensureTestNamespace},
		Spec: radixv1.RadixDeploymentSpec{
			AppName:     ensureTestAppName,
			Environment: ensureTestEnvName,
			Components:  []radixv1.RadixDeployComponent{{Name: ensureTestComponentName, VolumeMounts: volumeMounts}},
		},
	}
	return rd, &rd.Spec.Components[0]
}

func blobFuse2VolumeMount(name, container string, modify ...func(*radixv1.RadixBlobFuse2VolumeMount)) radixv1.RadixVolumeMount {
	vm := radixv1.RadixVolumeMount{Name: name, Path: "/mnt/" + name, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Container: container}}
	for _, m := range modify {
		m(vm.BlobFuse2)
	}
	return vm
}

// createLegacyPair creates a bound PV/PVC pair as created by previous operator versions: a random PVC name suffix,
// a PV name and volumeHandle with a UUID, a block cache path with another UUID, and attributes and labels added by others.
func (s *testSuite) createLegacyPair(rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent, vm radixv1.RadixVolumeMount, pvcSuffix string, created time.Time) (*corev1.PersistentVolume, *corev1.PersistentVolumeClaim) {
	desired, err := buildDesiredVolumeResources(rd, component, vm)
	s.Require().NoError(err)
	volumeName, err := GetVolumeMountVolumeName(&vm, component.GetName())
	s.Require().NoError(err)

	pvcName := fmt.Sprintf("pvc-%s-%s", volumeName, pvcSuffix)
	pvName := fmt.Sprintf("pv-radixvolumemount-%s", uuid.New().String())
	builder := newBlobfuse2PersistentVolumeBuilder(rd.Spec.AppName, rd.Spec.Environment, component, vm)

	pv := desired.pv.DeepCopy()
	pv.SetName(pvName)
	pv.SetCreationTimestamp(metav1.NewTime(created))
	pv.Labels["any-other-label"] = "any-value"
	pv.Spec = builder.BuildSpec(pvcName, rd.Namespace, buildVolumeHandle(rd.Spec.AppName, rd.Spec.Environment, component.GetName(), vm.Name, uuid.New().String()))
	for i, option := range pv.Spec.MountOptions {
		if normalizeMountOption(option) == blockCachePathMountOption {
			pv.Spec.MountOptions[i] = fmt.Sprintf("%s=/mnt/%s#blockcache", blockCachePathMountOption, uuid.New().String())
		}
	}
	slices.Reverse(pv.Spec.MountOptions)
	pv.Spec.ClaimRef.UID = types.UID(uuid.New().String())
	pv.Spec.ClaimRef.ResourceVersion = "123"
	pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributePvName] = pvName
	pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributePvcName] = pvcName
	pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributePvcNamespace] = rd.Namespace
	pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributeProvisionerIdentity] = "any-identity"
	pv.Status.Phase = corev1.VolumeBound

	pvc := desired.pvc.DeepCopy()
	pvc.SetName(pvcName)
	pvc.SetUID(pv.Spec.ClaimRef.UID)
	pvc.SetCreationTimestamp(metav1.NewTime(created))
	pvc.Spec.VolumeName = pvName
	pvc.Status.Phase = corev1.ClaimBound

	pv, err = s.kubeClient.CoreV1().PersistentVolumes().Create(context.Background(), pv, metav1.CreateOptions{})
	s.Require().NoError(err)
	pvc, err = s.kubeClient.CoreV1().PersistentVolumeClaims(rd.Namespace).Create(context.Background(), pvc, metav1.CreateOptions{})
	s.Require().NoError(err)
	return pv, pvc
}

func (s *testSuite) listPvAndPvcNames() ([]string, []string) {
	pvList, err := s.kubeClient.CoreV1().PersistentVolumes().List(context.Background(), metav1.ListOptions{})
	s.Require().NoError(err)
	pvcList, err := s.kubeClient.CoreV1().PersistentVolumeClaims("").List(context.Background(), metav1.ListOptions{})
	s.Require().NoError(err)
	var pvNames, pvcNames []string
	for _, pv := range pvList.Items {
		pvNames = append(pvNames, pv.Name)
	}
	for _, pvc := range pvcList.Items {
		pvcNames = append(pvcNames, pvc.Name)
	}
	slices.Sort(pvNames)
	slices.Sort(pvcNames)
	return pvNames, pvcNames
}

func claimNameOf(volumes []corev1.Volume, volumeName string) string {
	for _, v := range volumes {
		if v.Name == volumeName && v.PersistentVolumeClaim != nil {
			return v.PersistentVolumeClaim.ClaimName
		}
	}
	return ""
}

func (s *ensureComponentVolumesTestSuite) Test_LegacyPairIsReusedAndNothingIsCreated() {
	volumeMounts := map[string]radixv1.RadixVolumeMount{
		"default": blobFuse2VolumeMount("vol", "container"),
		"block disk": blobFuse2VolumeMount("vol", "container", func(b *radixv1.RadixBlobFuse2VolumeMount) {
			b.BlockCacheOptions = &radixv1.BlobFuse2BlockCacheOptions{DiskSize: new(uint32(100))}
		}),
		"read write": blobFuse2VolumeMount("vol", "container", func(b *radixv1.RadixBlobFuse2VolumeMount) { b.AccessMode = "ReadWriteMany"; b.GID = "1000" }),
		"identity": blobFuse2VolumeMount("vol", "container", func(b *radixv1.RadixBlobFuse2VolumeMount) {
			b.UseAzureIdentity = new(true)
			b.StorageAccount = "anyaccount"
			b.ResourceGroup = "anyrg"
			b.SubscriptionId = "anysub"
		}),
	}
	for name, vm := range volumeMounts {
		s.Run(name, func() {
			rd, component := newEnsureTestRd(vm)
			component.Identity = &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "any-client-id"}}
			_, legacyPvc := s.createLegacyPair(rd, component, vm, "abcde", time.Now())
			pvsBefore, pvcsBefore := s.listPvAndPvcNames()

			volumes, volumeMounts, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, rd, component, nil)
			s.Require().NoError(err)

			volumeName, err := GetVolumeMountVolumeName(&vm, component.GetName())
			s.Require().NoError(err)
			s.Equal(legacyPvc.Name, claimNameOf(volumes, volumeName))
			s.Require().Len(volumeMounts, 1)
			s.Equal(volumeName, volumeMounts[0].Name)
			pvsAfter, pvcsAfter := s.listPvAndPvcNames()
			s.Equal(pvsBefore, pvsAfter, "no PV must be created")
			s.Equal(pvcsBefore, pvcsAfter, "no PVC must be created")
		})
	}
}

func (s *ensureComponentVolumesTestSuite) Test_SelectsClaimFromHintThenOldest() {
	vm := blobFuse2VolumeMount("vol", "container")
	rd, component := newEnsureTestRd(vm)
	volumeName, err := GetVolumeMountVolumeName(&vm, component.GetName())
	s.Require().NoError(err)
	now := time.Now()
	hintFor := func(claimName string) []corev1.Volume {
		return []corev1.Volume{{Name: volumeName, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claimName}}}}
	}

	tests := map[string]struct {
		hint         func(oldest, newest *corev1.PersistentVolumeClaim) []corev1.Volume
		expectNewest bool
	}{
		"hint is preferred":                {hint: func(_, newest *corev1.PersistentVolumeClaim) []corev1.Volume { return hintFor(newest.Name) }, expectNewest: true},
		"oldest is selected without hint":  {hint: func(_, _ *corev1.PersistentVolumeClaim) []corev1.Volume { return nil }},
		"hint that does not match ignored": {hint: func(_, _ *corev1.PersistentVolumeClaim) []corev1.Volume { return hintFor("pvc-unknown") }},
	}
	for name, test := range tests {
		s.Run(name, func() {
			_, newestPvc := s.createLegacyPair(rd, component, vm, "bbbbb", now.Add(-1*time.Hour))
			_, oldestPvc := s.createLegacyPair(rd, component, vm, "aaaab", now.Add(-2*time.Hour))
			volumes, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, rd, component, test.hint(oldestPvc, newestPvc))
			s.Require().NoError(err)
			expected := oldestPvc.Name
			if test.expectNewest {
				expected = newestPvc.Name
			}
			s.Equal(expected, claimNameOf(volumes, volumeName))
			pvNames, pvcNames := s.listPvAndPvcNames()
			s.Len(pvNames, 2)
			s.Len(pvcNames, 2)
		})
	}
}

func (s *ensureComponentVolumesTestSuite) Test_HashNamedPairIsPreferredOverOlderLegacyPair() {
	vm := blobFuse2VolumeMount("vol", "container")
	rd, component := newEnsureTestRd(vm)
	desired, err := buildDesiredVolumeResources(rd, component, vm)
	s.Require().NoError(err)
	s.Require().NoError(createDesiredVolumeResources(context.Background(), s.kubeClient, desired))
	_, _ = s.createLegacyPair(rd, component, vm, "aaaaa", time.Now().Add(-24*time.Hour))
	volumeName, err := GetVolumeMountVolumeName(&vm, component.GetName())
	s.Require().NoError(err)

	volumes, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, rd, component, nil)
	s.Require().NoError(err)
	s.Equal(desired.pvc.Name, claimNameOf(volumes, volumeName))
}

func (s *ensureComponentVolumesTestSuite) Test_ChangedSpecCreatesNewHashNamedPair() {
	tests := map[string]func(*radixv1.RadixBlobFuse2VolumeMount){
		"requestsStorage": func(b *radixv1.RadixBlobFuse2VolumeMount) { b.RequestsStorage = resource.MustParse("10Gi") },
		"accessMode":      func(b *radixv1.RadixBlobFuse2VolumeMount) { b.AccessMode = "ReadWriteMany" },
		"container":       func(b *radixv1.RadixBlobFuse2VolumeMount) { b.Container = "othercontainer" },
		"gid":             func(b *radixv1.RadixBlobFuse2VolumeMount) { b.GID = "1000" },
		"uid":             func(b *radixv1.RadixBlobFuse2VolumeMount) { b.UID = "1000" },
		"storageAccount":  func(b *radixv1.RadixBlobFuse2VolumeMount) { b.StorageAccount = "otheraccount" },
		"useAdls":         func(b *radixv1.RadixBlobFuse2VolumeMount) { b.UseAdls = new(true) },
		"cacheMode":       func(b *radixv1.RadixBlobFuse2VolumeMount) { b.CacheMode = new(radixv1.BlobFuse2CacheModeFile) },
		"attributeCacheTimeout": func(b *radixv1.RadixBlobFuse2VolumeMount) {
			b.AttributeCacheOptions = &radixv1.BlobFuse2AttributeCacheOptions{Timeout: new(uint32(10))}
		},
		"blockCacheDiskSize": func(b *radixv1.RadixBlobFuse2VolumeMount) {
			b.BlockCacheOptions = &radixv1.BlobFuse2BlockCacheOptions{DiskSize: new(uint32(100))}
		},
		"blockCacheBlockSize": func(b *radixv1.RadixBlobFuse2VolumeMount) {
			b.BlockCacheOptions = &radixv1.BlobFuse2BlockCacheOptions{BlockSize: new(uint32(8))}
		},
	}
	for name, modify := range tests {
		s.Run(name, func() {
			initialVm := blobFuse2VolumeMount("vol", "container")
			rd, component := newEnsureTestRd(initialVm)
			_, legacyPvc := s.createLegacyPair(rd, component, initialVm, "abcde", time.Now())

			changedVm := blobFuse2VolumeMount("vol", "container", modify)
			changedRd, changedComponent := newEnsureTestRd(changedVm)
			changedVolumeName, err := GetVolumeMountVolumeName(&changedVm, changedComponent.GetName())
			s.Require().NoError(err)
			hint := []corev1.Volume{{Name: changedVolumeName, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: legacyPvc.Name}}}}

			volumes, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, changedRd, changedComponent, hint)
			s.Require().NoError(err)

			desired, err := buildDesiredVolumeResources(changedRd, changedComponent, changedVm)
			s.Require().NoError(err)
			s.Equal(desired.pvc.Name, claimNameOf(volumes, changedVolumeName))
			s.NotEqual(legacyPvc.Name, desired.pvc.Name)

			pvc, err := s.kubeClient.CoreV1().PersistentVolumeClaims(ensureTestNamespace).Get(context.Background(), desired.pvc.Name, metav1.GetOptions{})
			s.Require().NoError(err)
			pv, err := s.kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
			s.Require().NoError(err)
			s.Equal(desired.pv.Name, pv.Name)
			s.True(matchesDesired(pv, pvc, desired.pv, desired.pvc))

			_, err = s.kubeClient.CoreV1().PersistentVolumeClaims(ensureTestNamespace).Get(context.Background(), legacyPvc.Name, metav1.GetOptions{})
			s.NoError(err, "existing PVC must not be deleted")
		})
	}
}

func (s *ensureComponentVolumesTestSuite) Test_SettingThatDoesNotChangeSpecReusesPair() {
	initialVm := blobFuse2VolumeMount("vol", "container")
	rd, component := newEnsureTestRd(initialVm)
	_, legacyPvc := s.createLegacyPair(rd, component, initialVm, "abcde", time.Now())

	changedVm := blobFuse2VolumeMount("vol", "container", func(b *radixv1.RadixBlobFuse2VolumeMount) {
		b.BlockCacheOptions = &radixv1.BlobFuse2BlockCacheOptions{BlockSize: new(defaultBlockCacheBlockSize), PrefetchCount: new(defaultBlockCachePrefetchCount)}
		b.CacheMode = new(radixv1.BlobFuse2CacheModeBlock)
		b.Protocol = radixv1.BlobFuse2ProtocolFuse2
	})
	changedRd, changedComponent := newEnsureTestRd(changedVm)
	volumeName, err := GetVolumeMountVolumeName(&changedVm, changedComponent.GetName())
	s.Require().NoError(err)

	volumes, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, changedRd, changedComponent, nil)
	s.Require().NoError(err)
	s.Equal(legacyPvc.Name, claimNameOf(volumes, volumeName))
}

func (s *ensureComponentVolumesTestSuite) Test_RepeatedCallsCreateExactlyOnePair() {
	vm := blobFuse2VolumeMount("vol", "container")
	rd, component := newEnsureTestRd(vm)
	desired, err := buildDesiredVolumeResources(rd, component, vm)
	s.Require().NoError(err)

	// Simulates two controllers that both found no matching pair
	s.Require().NoError(createDesiredVolumeResources(context.Background(), s.kubeClient, desired))
	s.Require().NoError(createDesiredVolumeResources(context.Background(), s.kubeClient, desired))

	volumes1, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, rd, component, nil)
	s.Require().NoError(err)
	volumes2, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, rd, component, nil)
	s.Require().NoError(err)
	s.Equal(volumes1, volumes2)

	pvNames, pvcNames := s.listPvAndPvcNames()
	s.Equal([]string{desired.pv.Name}, pvNames)
	s.Equal([]string{desired.pvc.Name}, pvcNames)
}

func (s *ensureComponentVolumesTestSuite) Test_ReleasedHashNamedPvIsReboundToRecreatedPvc() {
	vm := blobFuse2VolumeMount("vol", "container")
	rd, component := newEnsureTestRd(vm)
	desired, err := buildDesiredVolumeResources(rd, component, vm)
	s.Require().NoError(err)

	releasedPv := desired.pv.DeepCopy()
	releasedPv.Spec.ClaimRef.UID = "old-pvc-uid"
	releasedPv.Spec.ClaimRef.ResourceVersion = "42"
	releasedPv.Status.Phase = corev1.VolumeReleased
	_, err = s.kubeClient.CoreV1().PersistentVolumes().Create(context.Background(), releasedPv, metav1.CreateOptions{})
	s.Require().NoError(err)

	volumes, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, rd, component, nil)
	s.Require().NoError(err)
	volumeName, err := GetVolumeMountVolumeName(&vm, component.GetName())
	s.Require().NoError(err)
	s.Equal(desired.pvc.Name, claimNameOf(volumes, volumeName))

	pv, err := s.kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), desired.pv.Name, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Empty(pv.Spec.ClaimRef.UID)
	s.Empty(pv.Spec.ClaimRef.ResourceVersion)
	s.Equal(desired.pvc.Name, pv.Spec.ClaimRef.Name)
	s.Equal(desired.pv.Spec.MountOptions, pv.Spec.MountOptions, "PV spec must not be changed")
	_, err = s.kubeClient.CoreV1().PersistentVolumeClaims(ensureTestNamespace).Get(context.Background(), desired.pvc.Name, metav1.GetOptions{})
	s.NoError(err)
}

func (s *ensureComponentVolumesTestSuite) Test_StaleClaimRefOfMatchingPairIsCleared() {
	vm := blobFuse2VolumeMount("vol", "container")
	rd, component := newEnsureTestRd(vm)
	desired, err := buildDesiredVolumeResources(rd, component, vm)
	s.Require().NoError(err)

	// The PV is still Bound to a deleted PVC with the same name, and the PVC has been recreated
	staleBoundPv := desired.pv.DeepCopy()
	staleBoundPv.Spec.ClaimRef.UID = "deleted-pvc-uid"
	staleBoundPv.Spec.ClaimRef.ResourceVersion = "42"
	staleBoundPv.Status.Phase = corev1.VolumeBound
	_, err = s.kubeClient.CoreV1().PersistentVolumes().Create(context.Background(), staleBoundPv, metav1.CreateOptions{})
	s.Require().NoError(err)
	recreatedPvc := desired.pvc.DeepCopy()
	recreatedPvc.SetUID("recreated-pvc-uid")
	recreatedPvc.Status.Phase = corev1.ClaimPending
	_, err = s.kubeClient.CoreV1().PersistentVolumeClaims(ensureTestNamespace).Create(context.Background(), recreatedPvc, metav1.CreateOptions{})
	s.Require().NoError(err)

	volumes, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, rd, component, nil)
	s.Require().NoError(err)
	volumeName, err := GetVolumeMountVolumeName(&vm, component.GetName())
	s.Require().NoError(err)
	s.Equal(desired.pvc.Name, claimNameOf(volumes, volumeName))
	pv, err := s.kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), desired.pv.Name, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Empty(pv.Spec.ClaimRef.UID)
	s.Empty(pv.Spec.ClaimRef.ResourceVersion)
}

func (s *ensureComponentVolumesTestSuite) Test_ClaimRefWithPvcUIDIsNotUpdated() {
	vm := blobFuse2VolumeMount("vol", "container")
	rd, component := newEnsureTestRd(vm)
	legacyPv, _ := s.createLegacyPair(rd, component, vm, "abcde", time.Now())

	_, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, rd, component, nil)
	s.Require().NoError(err)
	pv, err := s.kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), legacyPv.Name, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(legacyPv.Spec.ClaimRef.UID, pv.Spec.ClaimRef.UID)
	s.Equal(legacyPv.ResourceVersion, pv.ResourceVersion, "PV must not be updated")
}

func (s *ensureComponentVolumesTestSuite) Test_PvcBeingDeletedIsNotReused() {
	vm := blobFuse2VolumeMount("vol", "container")
	rd, component := newEnsureTestRd(vm)
	_, legacyPvc := s.createLegacyPair(rd, component, vm, "abcde", time.Now())
	legacyPvc.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	_, err := s.kubeClient.CoreV1().PersistentVolumeClaims(ensureTestNamespace).Update(context.Background(), legacyPvc, metav1.UpdateOptions{})
	s.Require().NoError(err)

	volumes, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, rd, component, nil)
	s.Require().NoError(err)
	desired, err := buildDesiredVolumeResources(rd, component, vm)
	s.Require().NoError(err)
	volumeName, err := GetVolumeMountVolumeName(&vm, component.GetName())
	s.Require().NoError(err)
	s.Equal(desired.pvc.Name, claimNameOf(volumes, volumeName))
}

func (s *ensureComponentVolumesTestSuite) Test_VolumesAndMountsAreConsistentAndOrdered() {
	vms := []radixv1.RadixVolumeMount{
		blobFuse2VolumeMount("vol1", "container1"),
		{Name: "empty", Path: "/mnt/empty", EmptyDir: &radixv1.RadixEmptyDirVolumeMount{SizeLimit: resource.MustParse("1Mi")}},
		blobFuse2VolumeMount("vol2", "container2"),
	}
	rd, component := newEnsureTestRd(vms...)

	volumes, volumeMounts, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, rd, component, nil)
	s.Require().NoError(err)
	s.Require().Len(volumes, 3)
	s.Require().Len(volumeMounts, 3)
	for i, vm := range vms {
		volumeName, err := GetVolumeMountVolumeName(&vm, component.GetName())
		s.Require().NoError(err)
		s.Equal(volumeName, volumes[i].Name)
		s.Equal(volumeName, volumeMounts[i].Name)
		s.Equal(vm.Path, volumeMounts[i].MountPath)
	}
	s.NotNil(volumes[0].PersistentVolumeClaim)
	s.NotNil(volumes[1].EmptyDir)
	s.NotNil(volumes[2].PersistentVolumeClaim)
	s.NotEqual(volumes[0].PersistentVolumeClaim.ClaimName, volumes[2].PersistentVolumeClaim.ClaimName)
}

func (s *ensureComponentVolumesTestSuite) Test_PairsAreNotSharedBetweenComponentsOrVolumeMounts() {
	vm := blobFuse2VolumeMount("vol", "container")
	otherVm := blobFuse2VolumeMount("othervol", "container")
	tests := map[string]struct {
		vm            radixv1.RadixVolumeMount
		componentName string
	}{
		"other component":         {vm: vm, componentName: "othercomp"},
		"other volume mount name": {vm: otherVm, componentName: ensureTestComponentName},
	}
	for name, test := range tests {
		s.Run(name, func() {
			rd, component := newEnsureTestRd(vm)
			_, legacyPvc := s.createLegacyPair(rd, component, vm, "abcde", time.Now())

			otherRd, otherComponent := newEnsureTestRd(test.vm)
			otherComponent.Name = test.componentName
			volumes, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, otherRd, otherComponent, nil)
			s.Require().NoError(err)
			volumeName, err := GetVolumeMountVolumeName(&test.vm, otherComponent.GetName())
			s.Require().NoError(err)
			claimName := claimNameOf(volumes, volumeName)
			s.NotEmpty(claimName)
			s.NotEqual(legacyPvc.Name, claimName)
		})
	}
}

func Test_HashIsDeterministic(t *testing.T) {
	vm := blobFuse2VolumeMount("vol", "container", func(b *radixv1.RadixBlobFuse2VolumeMount) {
		b.GID = "1000"
		b.BlockCacheOptions = &radixv1.BlobFuse2BlockCacheOptions{DiskSize: new(uint32(100))}
	})
	rd, component := newEnsureTestRd(vm)
	first, err := buildDesiredVolumeResources(rd, component, vm)
	if err != nil {
		t.Fatal(err)
	}
	for range 20 {
		next, err := buildDesiredVolumeResources(rd, component, vm)
		if err != nil {
			t.Fatal(err)
		}
		if first.pv.Name != next.pv.Name || first.pvc.Name != next.pvc.Name || first.pv.Spec.CSI.VolumeHandle != next.pv.Spec.CSI.VolumeHandle {
			t.Fatalf("names differ: %s/%s vs %s/%s", first.pv.Name, first.pvc.Name, next.pv.Name, next.pvc.Name)
		}
	}

	hash1, err := hashDesiredVolumeResources(ensureTestNamespace, first.pv, first.pvc)
	if err != nil {
		t.Fatal(err)
	}
	shuffledPv := first.pv.DeepCopy()
	slices.Reverse(shuffledPv.Spec.MountOptions)
	shuffledPv.Spec.CSI.VolumeHandle = "any-other-handle"
	shuffledPv.Spec.ClaimRef.Name = "any-other-name"
	shuffledPv.Spec.ClaimRef.UID = "any-uid"
	shuffledPv.Labels = maps.Clone(first.pv.Labels)
	shuffledPvc := first.pvc.DeepCopy()
	shuffledPvc.Spec.VolumeName = "any-other-pv"
	hash2, err := hashDesiredVolumeResources(ensureTestNamespace, shuffledPv, shuffledPvc)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 != hash2 {
		t.Fatalf("hash differs: %s vs %s", hash1, hash2)
	}

	hash3, err := hashDesiredVolumeResources("othernamespace", first.pv, first.pvc)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 == hash3 {
		t.Fatal("hash must include namespace")
	}
}

func Test_DesiredNamesAndVolumeHandleUseHash(t *testing.T) {
	vm := blobFuse2VolumeMount("vol", "container", func(b *radixv1.RadixBlobFuse2VolumeMount) {
		b.BlockCacheOptions = &radixv1.BlobFuse2BlockCacheOptions{DiskSize: new(uint32(100))}
	})
	rd, component := newEnsureTestRd(vm)
	desired, err := buildDesiredVolumeResources(rd, component, vm)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := hashDesiredVolumeResources(ensureTestNamespace, desired.pv, desired.pvc)
	if err != nil {
		t.Fatal(err)
	}
	expectedHandle := fmt.Sprintf("radixvolumemount#%s#%s#%s#%s#%s", ensureTestAppName, ensureTestEnvName, ensureTestComponentName, "vol", hash[:20])
	checks := map[string][2]string{
		"pv name":       {"pv-radixvolumemount-" + hash[:20], desired.pv.Name},
		"pvc name":      {"pvc-csi-blobfuse2-fuse2-anycomp-vol-container-" + hash[:10], desired.pvc.Name},
		"volume handle": {expectedHandle, desired.pv.Spec.CSI.VolumeHandle},
		"claimRef":      {desired.pvc.Name, desired.pv.Spec.ClaimRef.Name},
		"volumeName":    {desired.pv.Name, desired.pvc.Spec.VolumeName},
	}
	for name, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s: expected %s, got %s", name, c[0], c[1])
		}
	}
	if !slices.Contains(desired.pv.Spec.MountOptions, fmt.Sprintf("--block-cache-path=/mnt/%s#blockcache", expectedHandle)) {
		t.Errorf("block cache path must be derived from the volume handle: %v", desired.pv.Spec.MountOptions)
	}
}

func Test_MatchesDesired(t *testing.T) {
	vm := blobFuse2VolumeMount("vol", "container", func(b *radixv1.RadixBlobFuse2VolumeMount) {
		b.BlockCacheOptions = &radixv1.BlobFuse2BlockCacheOptions{DiskSize: new(uint32(100))}
	})
	rd, component := newEnsureTestRd(vm)
	desired, err := buildDesiredVolumeResources(rd, component, vm)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		modify   func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim)
		expected bool
	}{
		"identical": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {}, expected: true},
		"ignored: names, uid, volumeHandle, block cache path, provisioner attributes, extra PV labels, order": {
			modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
				pv.Name, pvc.Name = "pv-other", "pvc-other"
				pvc.Spec.VolumeName, pv.Spec.ClaimRef.Name = pv.Name, pvc.Name
				pv.Spec.ClaimRef.UID, pv.Spec.ClaimRef.ResourceVersion = "any-uid", "1"
				pv.Spec.CSI.VolumeHandle = "radixvolumemount#any#" + uuid.NewString()
				for i, o := range pv.Spec.MountOptions {
					if normalizeMountOption(o) == blockCachePathMountOption {
						pv.Spec.MountOptions[i] = blockCachePathMountOption + "=/mnt/" + uuid.NewString()
					}
				}
				slices.Reverse(pv.Spec.MountOptions)
				pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributePvName] = "x"
				pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributeProvisionerIdentity] = "x"
				pv.Labels["extra"] = "x"
			},
			expected: true,
		},
		"pvc not bound to pv":      {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) { pvc.Spec.VolumeName = "other" }},
		"pv claimRef to other pvc": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) { pv.Spec.ClaimRef.Name = "other" }},
		"pv claimRef to other namespace": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
			pv.Spec.ClaimRef.Namespace = "other"
		}},
		"pv without claimRef": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) { pv.Spec.ClaimRef = nil }},
		"pvc extra label":     {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) { pvc.Labels["extra"] = "x" }},
		"pvc missing label": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
			delete(pvc.Labels, kube.RadixMountTypeLabel)
		}},
		"pv label value differs": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
			pv.Labels[kube.RadixNamespace] = "other"
		}},
		"pv label missing": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
			delete(pv.Labels, kube.RadixVolumeMountNameLabel)
		}},
		"pvc storage request": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
			pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Mi")
		}},
		"pvc access mode": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
			pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
		}},
		"pvc volume mode": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
			pvc.Spec.VolumeMode = new(corev1.PersistentVolumeBlock)
		}},
		"pvc storage class": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
			pvc.Spec.StorageClassName = new("default")
		}},
		"pvc nil storage class": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) { pvc.Spec.StorageClassName = nil }, expected: true},
		"pvc in other namespace": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
			pvc.Namespace = "other"
			pv.Spec.ClaimRef.Namespace = "other"
		}},
		"pv capacity": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
			pv.Spec.Capacity[corev1.ResourceStorage] = resource.MustParse("2Mi")
		}},
		"pv access mode": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
			pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
		}},
		"pv mount option added": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
			pv.Spec.MountOptions = append(pv.Spec.MountOptions, "-o gid=1000")
		}},
		"pv block cache path missing": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
			pv.Spec.MountOptions = slices.DeleteFunc(pv.Spec.MountOptions, func(o string) bool { return normalizeMountOption(o) == blockCachePathMountOption })
		}},
		"pv csi driver":  {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) { pv.Spec.CSI.Driver = "other" }},
		"pv without csi": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) { pv.Spec.CSI = nil }},
		"pv node stage secret": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
			pv.Spec.CSI.NodeStageSecretRef = &corev1.SecretReference{Name: "other", Namespace: ensureTestNamespace}
		}},
		"pv volume attribute": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
			pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributeContainerName] = "other"
		}},
		"pv reclaim policy": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
			pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimDelete
		}},
		"pv storage class": {modify: func(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) {
			pv.Spec.StorageClassName = "default"
		}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			pv, pvc := desired.pv.DeepCopy(), desired.pvc.DeepCopy()
			test.modify(pv, pvc)
			if actual := matchesDesired(pv, pvc, desired.pv, desired.pvc); actual != test.expected {
				t.Errorf("expected %v, got %v", test.expected, actual)
			}
		})
	}
}
