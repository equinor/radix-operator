//nolint:staticcheck
package volumemount

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	testutils "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	ensureTestAppName       = "anyapp"
	ensureTestEnvName       = "anyenv"
	ensureTestNamespace     = "anyapp-anyenv"
	ensureTestComponentName = "anycomp"
	ensureTestRdName        = "anyrd"
)

type reconcileComponentVolumesTestSuite struct {
	testSuite
}

func TestReconcileComponentVolumesTestSuite(t *testing.T) {
	suite.Run(t, new(reconcileComponentVolumesTestSuite))
}

func (s *reconcileComponentVolumesTestSuite) SetupTest() {
	s.setupTest()
	testutils.CreateEnvNamespace(s.kubeClient, ensureTestAppName, ensureTestEnvName)
}

func (s *reconcileComponentVolumesTestSuite) SetupSubTest() {
	s.SetupTest()
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
	desired, err := buildDesiredVolumeResources(ensureTestAppName, rd, component, vm)
	s.Require().NoError(err)
	volumeName, err := getVolumeMountVolumeName(&vm, component.GetName())
	s.Require().NoError(err)

	pvcName := fmt.Sprintf("pvc-%s-%s", volumeName, pvcSuffix)
	pvName := fmt.Sprintf("pv-radixvolumemount-%s", uuid.New().String())
	builder := newBlobfuse2PersistentVolumeBuilder(component, vm)

	pv := desired.pv.DeepCopy()
	pv.SetName(pvName)
	pv.SetCreationTimestamp(metav1.NewTime(created))
	pv.Labels["any-other-label"] = "any-value"
	pv.Spec = builder.buildSpec(pvcName, rd.Namespace, buildVolumeHandle(ensureTestAppName, rd.Spec.Environment, component.GetName(), vm.Name, uuid.New().String()))
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

func (s *testSuite) pvcExists(name string) bool {
	_, err := s.kubeClient.CoreV1().PersistentVolumeClaims(ensureTestNamespace).Get(context.Background(), name, metav1.GetOptions{})
	return err == nil
}

func (s *testSuite) pvExists(name string) bool {
	_, err := s.kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), name, metav1.GetOptions{})
	return err == nil
}

func claimNameOf(volumes []corev1.Volume, volumeName string) string {
	for _, v := range volumes {
		if v.Name == volumeName && v.PersistentVolumeClaim != nil {
			return v.PersistentVolumeClaim.ClaimName
		}
	}
	return ""
}

func (s *reconcileComponentVolumesTestSuite) Test_LegacyPairIsReusedAndNothingIsCreated() {
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

			volumes, volumeMounts, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, ensureTestAppName, rd, component)
			s.Require().NoError(err)

			volumeName, err := getVolumeMountVolumeName(&vm, component.GetName())
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

func (s *reconcileComponentVolumesTestSuite) Test_OldestMatchingClaimIsSelected() {
	vm := blobFuse2VolumeMount("vol", "container")
	rd, component := newEnsureTestRd(vm)
	volumeName, err := getVolumeMountVolumeName(&vm, component.GetName())
	s.Require().NoError(err)
	now := time.Now()
	_, _ = s.createLegacyPair(rd, component, vm, "bbbbb", now.Add(-1*time.Hour))
	_, oldestPvc := s.createLegacyPair(rd, component, vm, "aaaab", now.Add(-2*time.Hour))

	volumes, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, ensureTestAppName, rd, component)
	s.Require().NoError(err)
	s.Equal(oldestPvc.Name, claimNameOf(volumes, volumeName))
}

func (s *reconcileComponentVolumesTestSuite) Test_ChangedSpecCreatesNewPair() {
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
			legacyPv, legacyPvc := s.createLegacyPair(rd, component, initialVm, "abcde", time.Now())

			changedVm := blobFuse2VolumeMount("vol", "container", modify)
			changedRd, changedComponent := newEnsureTestRd(changedVm)
			changedVolumeName, err := getVolumeMountVolumeName(&changedVm, changedComponent.GetName())
			s.Require().NoError(err)

			volumes, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, ensureTestAppName, changedRd, changedComponent)
			s.Require().NoError(err)

			claimName := claimNameOf(volumes, changedVolumeName)
			s.NotEqual(legacyPvc.Name, claimName)
			pvc, err := s.kubeClient.CoreV1().PersistentVolumeClaims(ensureTestNamespace).Get(context.Background(), claimName, metav1.GetOptions{})
			s.Require().NoError(err)
			pv, err := s.kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
			s.Require().NoError(err)
			desired, err := buildDesiredVolumeResources(ensureTestAppName, changedRd, changedComponent, changedVm)
			s.Require().NoError(err)
			s.True(matchesDesired(pv, pvc, desired.pv, desired.pvc), "the new pair must match the changed spec")
			s.True(s.pvcExists(legacyPvc.Name), "EnsureComponentVolumes must not delete the current PVC")
			s.True(s.pvExists(legacyPv.Name), "EnsureComponentVolumes must not delete the current PV")

			volumes, _, err = ReconcileComponentVolumes(context.Background(), s.kubeUtil, ensureTestAppName, changedRd, changedComponent)
			s.Require().NoError(err)
			s.Equal(claimName, claimNameOf(volumes, changedVolumeName), "the new pair must be reused")
			s.False(s.pvcExists(legacyPvc.Name), "ReconcileComponentVolumes must delete the PVC that does not match")
			s.False(s.pvExists(legacyPv.Name), "ReconcileComponentVolumes must delete the PV that does not match")
			s.True(s.pvcExists(pvc.Name))
			s.True(s.pvExists(pv.Name))
		})
	}
}

func (s *reconcileComponentVolumesTestSuite) Test_SettingThatDoesNotChangeSpecReusesPair() {
	initialVm := blobFuse2VolumeMount("vol", "container")
	rd, component := newEnsureTestRd(initialVm)
	_, legacyPvc := s.createLegacyPair(rd, component, initialVm, "abcde", time.Now())

	changedVm := blobFuse2VolumeMount("vol", "container", func(b *radixv1.RadixBlobFuse2VolumeMount) {
		b.BlockCacheOptions = &radixv1.BlobFuse2BlockCacheOptions{BlockSize: new(defaultBlockCacheBlockSize), PrefetchCount: new(defaultBlockCachePrefetchCount)}
		b.CacheMode = new(radixv1.BlobFuse2CacheModeBlock)
		b.Protocol = radixv1.BlobFuse2ProtocolFuse2
	})
	changedRd, changedComponent := newEnsureTestRd(changedVm)
	volumeName, err := getVolumeMountVolumeName(&changedVm, changedComponent.GetName())
	s.Require().NoError(err)

	volumes, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, ensureTestAppName, changedRd, changedComponent)
	s.Require().NoError(err)
	s.Equal(legacyPvc.Name, claimNameOf(volumes, volumeName))
}

func (s *reconcileComponentVolumesTestSuite) Test_RepeatedCallsCreateExactlyOnePair() {
	vm := blobFuse2VolumeMount("vol", "container")
	rd, component := newEnsureTestRd(vm)

	volumes1, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, ensureTestAppName, rd, component)
	s.Require().NoError(err)
	volumes2, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, ensureTestAppName, rd, component)
	s.Require().NoError(err)
	volumes3, _, err := ReconcileComponentVolumes(context.Background(), s.kubeUtil, ensureTestAppName, rd, component)
	s.Require().NoError(err)
	s.Equal(volumes1, volumes2)
	s.Equal(volumes1, volumes3)

	pvNames, pvcNames := s.listPvAndPvcNames()
	s.Len(pvNames, 1)
	s.Len(pvcNames, 1)
}

func (s *reconcileComponentVolumesTestSuite) Test_PairWithStaleClaimRefIsReplaced() {
	vm := blobFuse2VolumeMount("vol", "container")
	rd, component := newEnsureTestRd(vm)
	// The PV is still Bound to a deleted PVC with the same name, so the recreated PVC never binds to it
	stalePv, stalePvc := s.createLegacyPair(rd, component, vm, "abcde", time.Now())
	stalePv.Spec.ClaimRef.UID = "deleted-pvc-uid"
	_, err := s.kubeClient.CoreV1().PersistentVolumes().Update(context.Background(), stalePv, metav1.UpdateOptions{})
	s.Require().NoError(err)

	volumes, _, err := ReconcileComponentVolumes(context.Background(), s.kubeUtil, ensureTestAppName, rd, component)
	s.Require().NoError(err)

	volumeName, err := getVolumeMountVolumeName(&vm, component.GetName())
	s.Require().NoError(err)
	claimName := claimNameOf(volumes, volumeName)
	s.NotEqual(stalePvc.Name, claimName)
	s.True(s.pvcExists(claimName))
	s.False(s.pvcExists(stalePvc.Name))
	s.False(s.pvExists(stalePv.Name))
}

func (s *reconcileComponentVolumesTestSuite) Test_ClaimRefWithPvcUIDIsNotUpdated() {
	vm := blobFuse2VolumeMount("vol", "container")
	rd, component := newEnsureTestRd(vm)
	legacyPv, _ := s.createLegacyPair(rd, component, vm, "abcde", time.Now())

	_, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, ensureTestAppName, rd, component)
	s.Require().NoError(err)
	pv, err := s.kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), legacyPv.Name, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(legacyPv.Spec.ClaimRef.UID, pv.Spec.ClaimRef.UID)
	s.Equal(legacyPv.ResourceVersion, pv.ResourceVersion, "PV must not be updated")
}

func (s *reconcileComponentVolumesTestSuite) Test_PvcBeingDeletedIsNotReused() {
	vm := blobFuse2VolumeMount("vol", "container")
	rd, component := newEnsureTestRd(vm)
	_, legacyPvc := s.createLegacyPair(rd, component, vm, "abcde", time.Now())
	legacyPvc.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	_, err := s.kubeClient.CoreV1().PersistentVolumeClaims(ensureTestNamespace).Update(context.Background(), legacyPvc, metav1.UpdateOptions{})
	s.Require().NoError(err)

	volumes, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, ensureTestAppName, rd, component)
	s.Require().NoError(err)
	volumeName, err := getVolumeMountVolumeName(&vm, component.GetName())
	s.Require().NoError(err)
	claimName := claimNameOf(volumes, volumeName)
	s.NotEqual(legacyPvc.Name, claimName)
	s.True(s.pvcExists(claimName))
}

func (s *reconcileComponentVolumesTestSuite) Test_VolumesAndMountsAreConsistentAndOrdered() {
	vms := []radixv1.RadixVolumeMount{
		blobFuse2VolumeMount("vol1", "container1"),
		{Name: "empty", Path: "/mnt/empty", EmptyDir: &radixv1.RadixEmptyDirVolumeMount{SizeLimit: resource.MustParse("1Mi")}},
		blobFuse2VolumeMount("vol2", "container2"),
	}
	rd, component := newEnsureTestRd(vms...)

	volumes, volumeMounts, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, ensureTestAppName, rd, component)
	s.Require().NoError(err)
	s.Require().Len(volumes, 3)
	s.Require().Len(volumeMounts, 3)
	for i, vm := range vms {
		volumeName, err := getVolumeMountVolumeName(&vm, component.GetName())
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

func (s *reconcileComponentVolumesTestSuite) Test_PairsAreNotSharedBetweenComponentsOrVolumeMounts() {
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
			volumes, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, ensureTestAppName, otherRd, otherComponent)
			s.Require().NoError(err)
			volumeName, err := getVolumeMountVolumeName(&test.vm, otherComponent.GetName())
			s.Require().NoError(err)
			claimName := claimNameOf(volumes, volumeName)
			s.NotEmpty(claimName)
			s.NotEqual(legacyPvc.Name, claimName)
		})
	}
}

func (s *reconcileComponentVolumesTestSuite) Test_ReconcileKeepsSelectedPairAndDeletesAllOtherPairsOfComponent() {
	vm := blobFuse2VolumeMount("vol", "container")
	rd, component := newEnsureTestRd(vm)
	now := time.Now()
	keptPv, keptPvc := s.createLegacyPair(rd, component, vm, "aaaaa", now.Add(-2*time.Hour))
	duplicatePv, duplicatePvc := s.createLegacyPair(rd, component, vm, "bbbbb", now.Add(-1*time.Hour))
	oldSpecVm := blobFuse2VolumeMount("vol", "oldcontainer")
	oldSpecPv, oldSpecPvc := s.createLegacyPair(rd, component, oldSpecVm, "ccccc", now)
	removedVm := blobFuse2VolumeMount("removed", "container")
	removedPv, removedPvc := s.createLegacyPair(rd, component, removedVm, "ddddd", now)
	otherRd, otherComponent := newEnsureTestRd(vm)
	otherComponent.Name = "othercomp"
	otherPv, otherPvc := s.createLegacyPair(otherRd, otherComponent, vm, "eeeee", now)
	releasedPv := keptPv.DeepCopy()
	releasedPv.ObjectMeta = metav1.ObjectMeta{Name: "pv-released", Labels: keptPv.Labels}
	releasedPv.Spec.ClaimRef.Name = "deleted-pvc"
	releasedPv.Status.Phase = corev1.VolumeReleased
	_, err := s.kubeClient.CoreV1().PersistentVolumes().Create(context.Background(), releasedPv, metav1.CreateOptions{})
	s.Require().NoError(err)

	volumes, _, err := ReconcileComponentVolumes(context.Background(), s.kubeUtil, ensureTestAppName, rd, component)
	s.Require().NoError(err)

	volumeName, err := getVolumeMountVolumeName(&vm, component.GetName())
	s.Require().NoError(err)
	s.Equal(keptPvc.Name, claimNameOf(volumes, volumeName), "the oldest matching pair must be reused")
	s.True(s.pvcExists(keptPvc.Name))
	s.True(s.pvExists(keptPv.Name))
	for _, deleted := range []struct{ pv, pvc string }{{duplicatePv.Name, duplicatePvc.Name}, {oldSpecPv.Name, oldSpecPvc.Name}, {removedPv.Name, removedPvc.Name}} {
		s.False(s.pvcExists(deleted.pvc), "PVC %s must be deleted", deleted.pvc)
		s.False(s.pvExists(deleted.pv), "PV %s must be deleted", deleted.pv)
	}
	s.False(s.pvExists(releasedPv.Name), "a released PV of the component must be deleted")
	s.True(s.pvcExists(otherPvc.Name), "the PVC of another component must not be deleted")
	s.True(s.pvExists(otherPv.Name), "the PV of another component must not be deleted")
}

func (s *reconcileComponentVolumesTestSuite) Test_ReconcileDeletesNothingWhenEnsureFails() {
	oldSpecVm := blobFuse2VolumeMount("vol", "oldcontainer")
	rd, component := newEnsureTestRd(oldSpecVm)
	oldSpecPv, oldSpecPvc := s.createLegacyPair(rd, component, oldSpecVm, "abcde", time.Now())
	component.VolumeMounts = []radixv1.RadixVolumeMount{blobFuse2VolumeMount("vol", "newcontainer")}
	s.kubeClient.(*fake.Clientset).PrependReactor("create", "persistentvolumeclaims", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("any create error")
	})

	_, _, err := ReconcileComponentVolumes(context.Background(), s.kubeUtil, ensureTestAppName, rd, component)
	s.Require().Error(err)
	s.True(s.pvcExists(oldSpecPvc.Name))
	s.True(s.pvExists(oldSpecPv.Name))
}

func (s *reconcileComponentVolumesTestSuite) Test_GarbageCollectVolumesNoLongerInSpec() {
	vm := blobFuse2VolumeMount("vol", "container")
	rd, component := newEnsureTestRd(vm)
	rd.Spec.Jobs = []radixv1.RadixDeployJobComponent{{Name: "anyjob", VolumeMounts: []radixv1.RadixVolumeMount{vm}}}
	componentPv, componentPvc := s.createLegacyPair(rd, component, vm, "aaaaa", time.Now())
	jobPv, jobPvc := s.createLegacyPair(rd, &rd.Spec.Jobs[0], vm, "bbbbb", time.Now())
	removedComponent := &radixv1.RadixDeployComponent{Name: "removedcomp", VolumeMounts: []radixv1.RadixVolumeMount{vm}}
	removedPv, removedPvc := s.createLegacyPair(rd, removedComponent, vm, "ccccc", time.Now())
	removedReleasedPv := removedPv.DeepCopy()
	removedReleasedPv.ObjectMeta = metav1.ObjectMeta{Name: "pv-removed-released", Labels: removedPv.Labels}
	removedReleasedPv.Status.Phase = corev1.VolumeReleased
	_, err := s.kubeClient.CoreV1().PersistentVolumes().Create(context.Background(), removedReleasedPv, metav1.CreateOptions{})
	s.Require().NoError(err)

	untouchedPvcs := []*corev1.PersistentVolumeClaim{
		{ObjectMeta: metav1.ObjectMeta{Name: "no-labels", Namespace: ensureTestNamespace}},
		{ObjectMeta: metav1.ObjectMeta{Name: "other-app", Namespace: ensureTestNamespace, Labels: map[string]string{kube.RadixAppLabel: "otherapp", kube.RadixComponentLabel: "removedcomp", kube.RadixVolumeMountNameLabel: "vol"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "no-volume-mount-label", Namespace: ensureTestNamespace, Labels: map[string]string{kube.RadixAppLabel: ensureTestAppName, kube.RadixComponentLabel: "removedcomp"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "other-namespace", Namespace: "other-ns", Labels: map[string]string{kube.RadixAppLabel: ensureTestAppName, kube.RadixComponentLabel: "removedcomp", kube.RadixVolumeMountNameLabel: "vol"}}},
	}
	for _, pvc := range untouchedPvcs {
		_, err := s.kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(context.Background(), pvc, metav1.CreateOptions{})
		s.Require().NoError(err)
	}
	untouchedPvs := []*corev1.PersistentVolume{
		{ObjectMeta: metav1.ObjectMeta{Name: "pv-no-labels"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "pv-other-namespace", Labels: map[string]string{kube.RadixAppLabel: ensureTestAppName, kube.RadixNamespace: "other-ns", kube.RadixComponentLabel: "removedcomp", kube.RadixVolumeMountNameLabel: "vol"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "pv-other-app", Labels: map[string]string{kube.RadixAppLabel: "otherapp", kube.RadixNamespace: ensureTestNamespace, kube.RadixComponentLabel: "removedcomp", kube.RadixVolumeMountNameLabel: "vol"}}},
	}
	for _, pv := range untouchedPvs {
		_, err := s.kubeClient.CoreV1().PersistentVolumes().Create(context.Background(), pv, metav1.CreateOptions{})
		s.Require().NoError(err)
	}

	s.Require().NoError(GarbageCollectVolumesNoLongerInSpec(context.Background(), s.kubeClient, ensureTestAppName, rd))

	s.True(s.pvcExists(componentPvc.Name))
	s.True(s.pvExists(componentPv.Name))
	s.True(s.pvcExists(jobPvc.Name))
	s.True(s.pvExists(jobPv.Name))
	s.False(s.pvcExists(removedPvc.Name))
	s.False(s.pvExists(removedPv.Name))
	s.False(s.pvExists(removedReleasedPv.Name))
	for _, pvc := range untouchedPvcs {
		_, err := s.kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(context.Background(), pvc.Name, metav1.GetOptions{})
		s.NoError(err, "PVC %s/%s must not be deleted", pvc.Namespace, pvc.Name)
	}
	for _, pv := range untouchedPvs {
		s.True(s.pvExists(pv.Name), "PV %s must not be deleted", pv.Name)
	}
}

func Test_DesiredNamesAndVolumeHandle(t *testing.T) {
	vm := blobFuse2VolumeMount("vol", "container", func(b *radixv1.RadixBlobFuse2VolumeMount) {
		b.BlockCacheOptions = &radixv1.BlobFuse2BlockCacheOptions{DiskSize: new(uint32(100))}
	})
	rd, component := newEnsureTestRd(vm)
	desired, err := buildDesiredVolumeResources(ensureTestAppName, rd, component, vm)
	require.NoError(t, err)

	uniquePart, found := strings.CutPrefix(desired.pv.Name, "pv-"+ensureTestAppName+"-")
	require.True(t, found, "PV name %s must start with pv-<appname>-", desired.pv.Name)
	_, err = ulid.ParseStrict(strings.ToUpper(uniquePart))
	require.NoError(t, err, "the unique part of the PV name must be a ULID")
	assert.Equal(t, strings.ToLower(uniquePart), uniquePart, "names must be lower case")
	expectedHandle := fmt.Sprintf("radixvolumemount#%s#%s#%s#%s#%s", ensureTestAppName, ensureTestEnvName, ensureTestComponentName, "vol", uniquePart)
	assert.Equal(t, fmt.Sprintf("pvc-%s-%s-%s", ensureTestComponentName, "vol", uniquePart), desired.pvc.Name)
	assert.Equal(t, expectedHandle, desired.pv.Spec.CSI.VolumeHandle)
	assert.Equal(t, desired.pvc.Name, desired.pv.Spec.ClaimRef.Name)
	assert.Equal(t, desired.pv.Name, desired.pvc.Spec.VolumeName)
	assert.Contains(t, desired.pv.Spec.MountOptions, fmt.Sprintf("--block-cache-path=/mnt/%s#blockcache", expectedHandle), "the block cache path must be derived from the volume handle")

	next, err := buildDesiredVolumeResources(ensureTestAppName, rd, component, vm)
	require.NoError(t, err)
	assert.NotEqual(t, desired.pv.Name, next.pv.Name, "each build must get new names")
	assert.True(t, matchesDesired(next.pv, next.pvc, desired.pv, desired.pvc), "builds with new names must have the same spec")
}

func Test_MatchesDesired(t *testing.T) {
	vm := blobFuse2VolumeMount("vol", "container", func(b *radixv1.RadixBlobFuse2VolumeMount) {
		b.BlockCacheOptions = &radixv1.BlobFuse2BlockCacheOptions{DiskSize: new(uint32(100))}
	})
	rd, component := newEnsureTestRd(vm)
	desired, err := buildDesiredVolumeResources(ensureTestAppName, rd, component, vm)
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
