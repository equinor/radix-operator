//nolint:staticcheck
package volumemount

import (
	"context"
	"errors"
	"testing"
	"time"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

type garbageCollectVolumesTestSuite struct {
	testSuite
}

func TestGarbageCollectVolumesTestSuite(t *testing.T) {
	suite.Run(t, new(garbageCollectVolumesTestSuite))
}

const gcTestGracePeriod = 10 * time.Minute

var gcTestNow = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

func gcTestOld() time.Time { return gcTestNow.Add(-time.Hour) }

func newGcTestRd(name string, active bool, volumeMounts ...radixv1.RadixVolumeMount) *radixv1.RadixDeployment {
	rd, _ := newEnsureTestRd(volumeMounts...)
	rd.Name = name
	if active {
		rd.Status.Condition = radixv1.DeploymentActive
	} else {
		rd.Status.Condition = radixv1.DeploymentInactive
	}
	return rd
}

func newGcTestJobRd(name string, volumeMounts ...radixv1.RadixVolumeMount) *radixv1.RadixDeployment {
	rd := newGcTestRd(name, false)
	rd.Spec.Components = nil
	rd.Spec.Jobs = []radixv1.RadixDeployJobComponent{{Name: "anyjob", VolumeMounts: volumeMounts}}
	return rd
}

func (s *garbageCollectVolumesTestSuite) runGC(activeRd *radixv1.RadixDeployment) {
	s.Require().NoError(GarbageCollectVolumes(context.Background(), s.kubeUtil, activeRd, gcTestNow, gcTestGracePeriod))
}

func (s *garbageCollectVolumesTestSuite) pvcExists(name string) bool {
	_, err := s.kubeClient.CoreV1().PersistentVolumeClaims(ensureTestNamespace).Get(context.Background(), name, metav1.GetOptions{})
	return err == nil
}

func (s *garbageCollectVolumesTestSuite) pvExists(name string) bool {
	_, err := s.kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), name, metav1.GetOptions{})
	return err == nil
}

func (s *garbageCollectVolumesTestSuite) createRadixDeployment(rd *radixv1.RadixDeployment) {
	_, err := s.kubeUtil.RadixClient().RadixV1().RadixDeployments(rd.Namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
}

func (s *garbageCollectVolumesTestSuite) createBatch(name, rdName, jobName string, status radixv1.RadixBatchStatus, jobs ...radixv1.RadixBatchJob) {
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ensureTestNamespace},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{LocalObjectReference: radixv1.LocalObjectReference{Name: rdName}, Job: jobName},
			Jobs:                  jobs,
		},
		Status: status,
	}
	_, err := s.kubeUtil.RadixClient().RadixV1().RadixBatches(ensureTestNamespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
}

func pvcVolume(claimName string) corev1.Volume {
	return corev1.Volume{Name: "any", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claimName}}}
}

func (s *garbageCollectVolumesTestSuite) Test_K1_PairMatchingActiveRdIsKept() {
	vm := blobFuse2VolumeMount("vol", "container")
	activeRd := newGcTestRd("active", true, vm)
	pv, pvc := s.createLegacyPair(activeRd, &activeRd.Spec.Components[0], vm, "abcde", gcTestOld())

	s.runGC(activeRd)

	s.True(s.pvcExists(pvc.Name))
	s.True(s.pvExists(pv.Name))
}

func (s *garbageCollectVolumesTestSuite) Test_K1_DuplicatePairsMatchingActiveRdAreKept() {
	vm := blobFuse2VolumeMount("vol", "container")
	activeRd := newGcTestRd("active", true, vm)
	_, pvc1 := s.createLegacyPair(activeRd, &activeRd.Spec.Components[0], vm, "aaaaa", gcTestOld())
	_, pvc2 := s.createLegacyPair(activeRd, &activeRd.Spec.Components[0], vm, "bbbbb", gcTestOld())

	s.runGC(activeRd)

	s.True(s.pvcExists(pvc1.Name))
	s.True(s.pvcExists(pvc2.Name))
}

func (s *garbageCollectVolumesTestSuite) Test_K1_PairMatchingJobInActiveRdIsKept() {
	vm := blobFuse2VolumeMount("vol", "container")
	activeRd := newGcTestJobRd("active", vm)
	activeRd.Status.Condition = radixv1.DeploymentActive
	_, pvc := s.createLegacyPair(activeRd, &activeRd.Spec.Jobs[0], vm, "abcde", gcTestOld())

	s.runGC(activeRd)

	s.True(s.pvcExists(pvc.Name))
}

func (s *garbageCollectVolumesTestSuite) Test_K0_NewPairIsKept() {
	oldVm := blobFuse2VolumeMount("vol", "oldcontainer")
	oldRd := newGcTestRd("old", false, oldVm)
	pv, pvc := s.createLegacyPair(oldRd, &oldRd.Spec.Components[0], oldVm, "abcde", gcTestNow.Add(-gcTestGracePeriod+time.Minute))
	activeRd := newGcTestRd("active", true, blobFuse2VolumeMount("vol", "newcontainer"))

	s.runGC(activeRd)

	s.True(s.pvcExists(pvc.Name))
	s.True(s.pvExists(pv.Name))
}

func (s *garbageCollectVolumesTestSuite) Test_LeftoverPairFromOldSpecIsDeletedPvcFirst() {
	oldVm := blobFuse2VolumeMount("vol", "oldcontainer")
	oldRd := newGcTestRd("old", false, oldVm)
	oldPv, oldPvc := s.createLegacyPair(oldRd, &oldRd.Spec.Components[0], oldVm, "aaaaa", gcTestOld())
	newVm := blobFuse2VolumeMount("vol", "newcontainer")
	activeRd := newGcTestRd("active", true, newVm)
	newPv, newPvc := s.createLegacyPair(activeRd, &activeRd.Spec.Components[0], newVm, "bbbbb", gcTestOld())
	fakeClient := s.kubeClient.(*fake.Clientset)
	fakeClient.ClearActions()

	s.runGC(activeRd)

	s.False(s.pvcExists(oldPvc.Name))
	s.False(s.pvExists(oldPv.Name))
	s.True(s.pvcExists(newPvc.Name))
	s.True(s.pvExists(newPv.Name))

	var deleted []string
	for _, action := range fakeClient.Actions() {
		if action.GetVerb() == "delete" {
			deleted = append(deleted, action.GetResource().Resource)
		}
	}
	s.Equal([]string{"persistentvolumeclaims", "persistentvolumes"}, deleted)
}

func (s *garbageCollectVolumesTestSuite) Test_PairForRemovedComponentOrVolumeMountIsDeleted() {
	vm := blobFuse2VolumeMount("vol", "container")
	oldRd := newGcTestRd("old", false, vm)
	_, pvc := s.createLegacyPair(oldRd, &oldRd.Spec.Components[0], vm, "abcde", gcTestOld())
	activeRd := newGcTestRd("active", true)

	s.runGC(activeRd)

	s.False(s.pvcExists(pvc.Name))
}

func (s *garbageCollectVolumesTestSuite) Test_K2_UnfinishedBatch() {
	batchVm := blobFuse2VolumeMount("vol", "batchcontainer")
	activeVm := blobFuse2VolumeMount("vol", "activecontainer")
	tests := map[string]struct {
		status     radixv1.RadixBatchStatus
		jobs       []radixv1.RadixBatchJob
		rdExists   bool
		expectKept bool
	}{
		"active batch for older RD is kept": {
			status:     radixv1.RadixBatchStatus{Condition: radixv1.RadixBatchCondition{Type: radixv1.BatchConditionTypeActive}},
			rdExists:   true,
			expectKept: true,
		},
		"waiting batch for older RD is kept": {
			status:     radixv1.RadixBatchStatus{Condition: radixv1.RadixBatchCondition{Type: radixv1.BatchConditionTypeWaiting}},
			rdExists:   true,
			expectKept: true,
		},
		"completed batch is deleted": {
			status:   radixv1.RadixBatchStatus{Condition: radixv1.RadixBatchCondition{Type: radixv1.BatchConditionTypeCompleted}, JobStatuses: []radixv1.RadixBatchJobStatus{{Name: "job1"}}},
			jobs:     []radixv1.RadixBatchJob{{Name: "job1"}},
			rdExists: true,
		},
		"completed batch with pending restart is kept": {
			status:     radixv1.RadixBatchStatus{Condition: radixv1.RadixBatchCondition{Type: radixv1.BatchConditionTypeCompleted}, JobStatuses: []radixv1.RadixBatchJobStatus{{Name: "job1"}}},
			jobs:       []radixv1.RadixBatchJob{{Name: "job1", Restart: "2026-01-01T00:00:00Z"}},
			rdExists:   true,
			expectKept: true,
		},
		"completed batch with handled restart is deleted": {
			status:   radixv1.RadixBatchStatus{Condition: radixv1.RadixBatchCondition{Type: radixv1.BatchConditionTypeCompleted}, JobStatuses: []radixv1.RadixBatchJobStatus{{Name: "job1", Restart: "2026-01-01T00:00:00Z"}}},
			jobs:     []radixv1.RadixBatchJob{{Name: "job1", Restart: "2026-01-01T00:00:00Z"}},
			rdExists: true,
		},
		"active batch for missing RD is deleted": {
			status: radixv1.RadixBatchStatus{Condition: radixv1.RadixBatchCondition{Type: radixv1.BatchConditionTypeActive}},
		},
	}
	for name, test := range tests {
		s.Run(name, func() {
			olderRd := newGcTestJobRd("older", batchVm)
			if test.rdExists {
				s.createRadixDeployment(olderRd)
			}
			activeRd := newGcTestJobRd("active", activeVm)
			activeRd.Status.Condition = radixv1.DeploymentActive
			s.createRadixDeployment(activeRd)
			s.createBatch("batch", olderRd.Name, "anyjob", test.status, test.jobs...)
			_, batchPvc := s.createLegacyPair(olderRd, &olderRd.Spec.Jobs[0], batchVm, "aaaaa", gcTestOld())
			_, activePvc := s.createLegacyPair(activeRd, &activeRd.Spec.Jobs[0], activeVm, "bbbbb", gcTestOld())

			s.runGC(activeRd)

			s.Equal(test.expectKept, s.pvcExists(batchPvc.Name))
			s.True(s.pvcExists(activePvc.Name))
		})
	}
}

func (s *garbageCollectVolumesTestSuite) Test_K3_PairReferencedByKubernetesJob() {
	oldVm := blobFuse2VolumeMount("vol", "oldcontainer")
	oldRd := newGcTestRd("old", false, oldVm)
	activeRd := newGcTestRd("active", true)

	s.Run("kept while the completed Job exists", func() {
		_, pvc := s.createLegacyPair(oldRd, &oldRd.Spec.Components[0], oldVm, "abcde", gcTestOld())
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: "anyjob", Namespace: ensureTestNamespace},
			Spec:       batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Volumes: []corev1.Volume{pvcVolume(pvc.Name)}}}},
			Status:     batchv1.JobStatus{Succeeded: 1},
		}
		_, err := s.kubeClient.BatchV1().Jobs(ensureTestNamespace).Create(context.Background(), job, metav1.CreateOptions{})
		s.Require().NoError(err)

		s.runGC(activeRd)
		s.True(s.pvcExists(pvc.Name))

		s.Require().NoError(s.kubeClient.BatchV1().Jobs(ensureTestNamespace).Delete(context.Background(), job.Name, metav1.DeleteOptions{}))
		s.runGC(activeRd)
		s.False(s.pvcExists(pvc.Name))
	})
}

func (s *garbageCollectVolumesTestSuite) Test_K4_PairMountedByPod() {
	oldVm := blobFuse2VolumeMount("vol", "oldcontainer")
	oldRd := newGcTestRd("old", false, oldVm)
	activeRd := newGcTestRd("active", true)
	tests := map[string]struct {
		phase      corev1.PodPhase
		expectKept bool
	}{
		"running pod from old ReplicaSet": {phase: corev1.PodRunning, expectKept: true},
		"pending pod":                     {phase: corev1.PodPending, expectKept: true},
		"succeeded pod":                   {phase: corev1.PodSucceeded},
		"failed pod":                      {phase: corev1.PodFailed},
	}
	for name, test := range tests {
		s.Run(name, func() {
			_, pvc := s.createLegacyPair(oldRd, &oldRd.Spec.Components[0], oldVm, "abcde", gcTestOld())
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "anypod", Namespace: ensureTestNamespace},
				Spec:       corev1.PodSpec{Volumes: []corev1.Volume{pvcVolume(pvc.Name)}},
				Status:     corev1.PodStatus{Phase: test.phase},
			}
			_, err := s.kubeClient.CoreV1().Pods(ensureTestNamespace).Create(context.Background(), pod, metav1.CreateOptions{})
			s.Require().NoError(err)

			s.runGC(activeRd)

			s.Equal(test.expectKept, s.pvcExists(pvc.Name))
		})
	}
}

func (s *garbageCollectVolumesTestSuite) Test_NonRadixAndOtherAppResourcesAreNeverTouched() {
	activeRd := newGcTestRd("active", true)
	pvcs := []*corev1.PersistentVolumeClaim{
		{ObjectMeta: metav1.ObjectMeta{Name: "no-labels", Namespace: ensureTestNamespace}},
		{ObjectMeta: metav1.ObjectMeta{Name: "other-app", Namespace: ensureTestNamespace, Labels: map[string]string{"radix-app": "otherapp", "radix-volume-mount-name": "vol"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "no-volume-mount-label", Namespace: ensureTestNamespace, Labels: map[string]string{"radix-app": ensureTestAppName}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "other-namespace", Namespace: "other-ns", Labels: map[string]string{"radix-app": ensureTestAppName, "radix-volume-mount-name": "vol"}}},
	}
	for _, pvc := range pvcs {
		_, err := s.kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(context.Background(), pvc, metav1.CreateOptions{})
		s.Require().NoError(err)
	}
	pvs := []*corev1.PersistentVolume{
		{ObjectMeta: metav1.ObjectMeta{Name: "pv-no-labels"}, Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeReleased}},
		{ObjectMeta: metav1.ObjectMeta{Name: "pv-other-namespace", Labels: map[string]string{"radix-app": ensureTestAppName, "radix-namespace": "other-ns", "radix-volume-mount-name": "vol"}}, Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeReleased}},
		{ObjectMeta: metav1.ObjectMeta{Name: "pv-other-app", Labels: map[string]string{"radix-app": "otherapp", "radix-namespace": ensureTestNamespace, "radix-volume-mount-name": "vol"}}, Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeReleased}},
	}
	for _, pv := range pvs {
		_, err := s.kubeClient.CoreV1().PersistentVolumes().Create(context.Background(), pv, metav1.CreateOptions{})
		s.Require().NoError(err)
	}

	s.runGC(activeRd)

	for _, pvc := range pvcs {
		_, err := s.kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(context.Background(), pvc.Name, metav1.GetOptions{})
		s.NoError(err, "PVC %s/%s must not be deleted", pvc.Namespace, pvc.Name)
	}
	for _, pv := range pvs {
		s.True(s.pvExists(pv.Name), "PV %s must not be deleted", pv.Name)
	}
}

func (s *garbageCollectVolumesTestSuite) Test_OrphanedPvs() {
	vm := blobFuse2VolumeMount("vol", "container")
	activeRd := newGcTestRd("active", true, vm)
	tests := map[string]struct {
		phase      corev1.PersistentVolumePhase
		created    time.Time
		claimName  func(keptPvcName string) string
		claimUID   *types.UID
		expectKept bool
	}{
		"released, claim deleted":                 {phase: corev1.VolumeReleased, created: gcTestOld(), claimName: func(string) string { return "deleted-pvc" }},
		"failed, claim deleted":                   {phase: corev1.VolumeFailed, created: gcTestOld(), claimName: func(string) string { return "deleted-pvc" }},
		"available, claim missing":                {phase: corev1.VolumeAvailable, created: gcTestOld(), claimName: func(string) string { return "missing-pvc" }},
		"available, new":                          {phase: corev1.VolumeAvailable, created: gcTestNow.Add(-time.Minute), claimName: func(string) string { return "missing-pvc" }, expectKept: true},
		"released, claim of kept PVC":             {phase: corev1.VolumeReleased, created: gcTestOld(), claimName: func(kept string) string { return kept }, expectKept: true},
		"released, claim without uid of kept PVC": {phase: corev1.VolumeReleased, created: gcTestOld(), claimName: func(kept string) string { return kept }, claimUID: new(types.UID("")), expectKept: true},
		"released, stale uid of a deleted PVC with the name of a kept PVC bound to another PV": {
			phase: corev1.VolumeReleased, created: gcTestOld(), claimName: func(kept string) string { return kept }, claimUID: new(types.UID("deleted-pvc-uid")),
		},
		"bound": {phase: corev1.VolumeBound, created: gcTestOld(), claimName: func(string) string { return "other-pvc" }, expectKept: true},
	}
	for name, test := range tests {
		s.Run(name, func() {
			keptPv, keptPvc := s.createLegacyPair(activeRd, &activeRd.Spec.Components[0], vm, "abcde", gcTestOld())
			orphan := keptPv.DeepCopy()
			orphan.ObjectMeta = metav1.ObjectMeta{Name: "pv-orphan", Labels: keptPv.Labels, CreationTimestamp: metav1.NewTime(test.created)}
			orphan.Spec.ClaimRef.Name = test.claimName(keptPvc.Name)
			if test.claimUID != nil {
				orphan.Spec.ClaimRef.UID = *test.claimUID
			}
			orphan.Status.Phase = test.phase
			_, err := s.kubeClient.CoreV1().PersistentVolumes().Create(context.Background(), orphan, metav1.CreateOptions{})
			s.Require().NoError(err)

			s.runGC(activeRd)

			s.Equal(test.expectKept, s.pvExists(orphan.Name))
			s.True(s.pvExists(keptPv.Name))
			s.True(s.pvcExists(keptPvc.Name))
		})
	}
}

func (s *garbageCollectVolumesTestSuite) Test_PvNotBoundToDeletedPvcIsNotDeleted() {
	oldVm := blobFuse2VolumeMount("vol", "oldcontainer")
	oldRd := newGcTestRd("old", false, oldVm)
	pv, pvc := s.createLegacyPair(oldRd, &oldRd.Spec.Components[0], oldVm, "abcde", gcTestOld())
	pv.Spec.ClaimRef.Name = "other-pvc"
	_, err := s.kubeClient.CoreV1().PersistentVolumes().Update(context.Background(), pv, metav1.UpdateOptions{})
	s.Require().NoError(err)
	activeRd := newGcTestRd("active", true)

	s.runGC(activeRd)

	s.False(s.pvcExists(pvc.Name))
	s.True(s.pvExists(pv.Name), "a PV whose claimRef refers to another PVC must not be deleted with the PVC")
}

func (s *garbageCollectVolumesTestSuite) Test_HashNamedPairCreatedByEnsureIsKept() {
	vm := blobFuse2VolumeMount("vol", "container")
	activeRd := newGcTestRd("active", true, vm)
	volumes, _, err := EnsureComponentVolumes(context.Background(), s.kubeUtil, activeRd, &activeRd.Spec.Components[0], nil)
	s.Require().NoError(err)

	s.runGC(activeRd)

	volumeName, err := GetVolumeMountVolumeName(&vm, ensureTestComponentName)
	s.Require().NoError(err)
	s.True(s.pvcExists(claimNameOf(volumes, volumeName)))
}

func (s *garbageCollectVolumesTestSuite) Test_K3_PairReferencedByWorkloadTemplate() {
	oldVm := blobFuse2VolumeMount("vol", "oldcontainer")
	oldRd := newGcTestRd("old", false, oldVm)
	activeRd := newGcTestRd("active", true)
	template := func(claimName string) corev1.PodTemplateSpec {
		return corev1.PodTemplateSpec{Spec: corev1.PodSpec{Volumes: []corev1.Volume{pvcVolume(claimName)}}}
	}
	tests := map[string]struct {
		create     func(claimName string) error
		expectKept bool
	}{
		"deployment scaled to zero": {
			create: func(claimName string) error {
				_, err := s.kubeClient.AppsV1().Deployments(ensureTestNamespace).Create(context.Background(), &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: "anycomp", Namespace: ensureTestNamespace},
					Spec:       appsv1.DeploymentSpec{Replicas: new(int32(0)), Template: template(claimName)},
				}, metav1.CreateOptions{})
				return err
			},
			expectKept: true,
		},
		"replicaset with replicas": {
			create: func(claimName string) error {
				_, err := s.kubeClient.AppsV1().ReplicaSets(ensureTestNamespace).Create(context.Background(), &appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{Name: "anycomp-old", Namespace: ensureTestNamespace},
					Spec:       appsv1.ReplicaSetSpec{Replicas: new(int32(1)), Template: template(claimName)},
				}, metav1.CreateOptions{})
				return err
			},
			expectKept: true,
		},
		"replicaset scaled to zero": {
			create: func(claimName string) error {
				_, err := s.kubeClient.AppsV1().ReplicaSets(ensureTestNamespace).Create(context.Background(), &appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{Name: "anycomp-old", Namespace: ensureTestNamespace},
					Spec:       appsv1.ReplicaSetSpec{Replicas: new(int32(0)), Template: template(claimName)},
				}, metav1.CreateOptions{})
				return err
			},
		},
	}
	for name, test := range tests {
		s.Run(name, func() {
			_, pvc := s.createLegacyPair(oldRd, &oldRd.Spec.Components[0], oldVm, "abcde", gcTestOld())
			s.Require().NoError(test.create(pvc.Name))

			s.runGC(activeRd)

			s.Equal(test.expectKept, s.pvcExists(pvc.Name))
		})
	}
}

func Test_DeletePersistentVolumesForDeletedNamespace(t *testing.T) {
	const namespace = "anyapp-anyenv"
	newPv := func(name string, labels map[string]string, phase corev1.PersistentVolumePhase) *corev1.PersistentVolume {
		return &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}, Status: corev1.PersistentVolumeStatus{Phase: phase}}
	}
	radixLabels := func(ns string) map[string]string {
		return map[string]string{"radix-app": "anyapp", "radix-namespace": ns, "radix-component": "anycomp", "radix-volume-mount-name": "vol"}
	}
	tests := map[string]struct {
		namespace     *corev1.Namespace
		expectDeleted bool
	}{
		"namespace is gone":           {namespace: nil, expectDeleted: true},
		"namespace is being deleted":  {namespace: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace, DeletionTimestamp: &metav1.Time{Time: gcTestNow}, Finalizers: []string{"kubernetes"}}}, expectDeleted: true},
		"namespace exists and active": {namespace: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset()
			if test.namespace != nil {
				_, err := kubeClient.CoreV1().Namespaces().Create(context.Background(), test.namespace, metav1.CreateOptions{})
				if err != nil {
					t.Fatal(err)
				}
			}
			pvs := []*corev1.PersistentVolume{
				newPv("pv-released", radixLabels(namespace), corev1.VolumeReleased),
				newPv("pv-bound", radixLabels(namespace), corev1.VolumeBound),
				newPv("pv-other-namespace", radixLabels("anyapp-otherenv"), corev1.VolumeReleased),
				newPv("pv-without-radix-labels", map[string]string{"radix-namespace": namespace}, corev1.VolumeReleased),
			}
			for _, pv := range pvs {
				if _, err := kubeClient.CoreV1().PersistentVolumes().Create(context.Background(), pv, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			}

			if err := DeletePersistentVolumesForDeletedNamespace(context.Background(), kubeClient, namespace); err != nil {
				t.Fatal(err)
			}

			exists := func(name string) bool {
				_, err := kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), name, metav1.GetOptions{})
				return err == nil
			}
			for _, name := range []string{"pv-released", "pv-bound"} {
				if exists(name) == test.expectDeleted {
					t.Errorf("PV %s: expected deleted=%v", name, test.expectDeleted)
				}
			}
			for _, name := range []string{"pv-other-namespace", "pv-without-radix-labels"} {
				if !exists(name) {
					t.Errorf("PV %s must not be deleted", name)
				}
			}
		})
	}
}

func (s *garbageCollectVolumesTestSuite) Test_DeleteIsSkippedWhenChangedSinceRead() {
	oldVm := blobFuse2VolumeMount("vol", "oldcontainer")
	oldRd := newGcTestRd("old", false, oldVm)
	activeRd := newGcTestRd("active", true)
	pv, pvc := s.createLegacyPair(oldRd, &oldRd.Spec.Components[0], oldVm, "abcde", gcTestOld())
	fakeClient := s.kubeClient.(*fake.Clientset)
	var preconditions *metav1.Preconditions
	fakeClient.PrependReactor("delete", "persistentvolumeclaims", func(action k8stesting.Action) (bool, runtime.Object, error) {
		preconditions = action.(k8stesting.DeleteAction).GetDeleteOptions().Preconditions
		return true, nil, k8serrors.NewConflict(corev1.Resource("persistentvolumeclaims"), pvc.Name, errors.New("precondition failed"))
	})

	s.runGC(activeRd)

	s.Require().NotNil(preconditions)
	s.Require().NotNil(preconditions.UID)
	s.Equal(pvc.UID, *preconditions.UID, "the PVC delete must have the uid of the PVC that was evaluated as precondition")
	s.True(s.pvcExists(pvc.Name))
	s.True(s.pvExists(pv.Name), "the PV must not be deleted when the PVC was changed since it was evaluated")
}
