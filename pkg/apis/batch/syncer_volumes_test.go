//nolint:staticcheck
package batch

import (
	"context"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/apis/volumemount"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	volumesTestAppName      = "any-app"
	volumesTestNamespace    = "any-app-dev"
	volumesTestJobComponent = "compute"
	volumesTestBatchName    = "any-batch"
	volumesTestBatchJobName = "job1"
)

func volumesTestRd(name, container string) *radixv1.RadixDeployment {
	return &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: volumesTestNamespace},
		Spec: radixv1.RadixDeploymentSpec{
			AppName:     volumesTestAppName,
			Environment: "dev",
			Jobs: []radixv1.RadixDeployJobComponent{{
				Name:    volumesTestJobComponent,
				Payload: &radixv1.RadixJobComponentPayload{Path: "/mnt/payload"},
				VolumeMounts: []radixv1.RadixVolumeMount{
					{Name: "blob", Path: "/mnt/blob", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Container: container}},
					{Name: "tmp", Path: "/mnt/tmp", EmptyDir: &radixv1.RadixEmptyDirVolumeMount{SizeLimit: resource.MustParse("1Mi")}},
				},
			}},
		},
	}
}

func volumesTestBatch(rdName string) *radixv1.RadixBatch {
	return &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: volumesTestBatchName, Namespace: volumesTestNamespace, Labels: radixlabels.ForJobScheduleJobType()},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  volumesTestJobComponent,
			},
			Jobs: []radixv1.RadixBatchJob{{Name: volumesTestBatchJobName, PayloadSecretRef: &radixv1.PayloadSecretKeySelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: "payload-secret"}, Key: "payload-key",
			}}},
		},
	}
}

func (s *syncerTestSuite) createVolumesTestResources(batch *radixv1.RadixBatch, rds ...*radixv1.RadixDeployment) *radixv1.RadixBatch {
	for _, rd := range rds {
		_, err := s.radixClient.RadixV1().RadixDeployments(volumesTestNamespace).Create(context.Background(), rd, metav1.CreateOptions{})
		s.Require().NoError(err)
	}
	payloadSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "payload-secret", Namespace: volumesTestNamespace, Labels: radixlabels.Merge(radixlabels.ForComponentName(volumesTestJobComponent), radixlabels.ForJobScheduleJobType(), radixlabels.ForBatchName(volumesTestBatchName))},
		Data:       map[string][]byte{"payload-key": []byte("any")},
	}
	_, err := s.kubeClient.CoreV1().Secrets(volumesTestNamespace).Create(context.Background(), payloadSecret, metav1.CreateOptions{})
	s.Require().NoError(err)
	batch, err = s.radixClient.RadixV1().RadixBatches(volumesTestNamespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	return batch
}

func (s *syncerTestSuite) getSingleKubeJob() batchv1.Job {
	jobs, err := s.kubeClient.BatchV1().Jobs(volumesTestNamespace).List(context.Background(), metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().Len(jobs.Items, 1)
	return jobs.Items[0]
}

func blobClaimName(job batchv1.Job) string {
	for _, v := range job.Spec.Template.Spec.Volumes {
		if v.PersistentVolumeClaim != nil {
			return v.PersistentVolumeClaim.ClaimName
		}
	}
	return ""
}

func (s *syncerTestSuite) assertVolumesAndMountsAreConsistent(job batchv1.Job) {
	volumeNames := map[string]struct{}{}
	for _, v := range job.Spec.Template.Spec.Volumes {
		volumeNames[v.Name] = struct{}{}
	}
	s.Len(volumeNames, len(job.Spec.Template.Spec.Volumes), "volume names must be unique")
	for _, m := range job.Spec.Template.Spec.Containers[0].VolumeMounts {
		s.Contains(volumeNames, m.Name, "volume mount %s has no volume", m.Name)
	}
	s.Len(job.Spec.Template.Spec.Containers[0].VolumeMounts, len(job.Spec.Template.Spec.Volumes))
}

func (s *syncerTestSuite) Test_Volumes_EnsuredBeforeJobIsCreated() {
	rd := volumesTestRd("any-rd", "container1")
	batch := s.createVolumesTestResources(volumesTestBatch(rd.Name), rd)

	sut := s.createSyncer(batch, config.Config{})
	s.Require().NoError(sut.OnSync(context.Background()))

	job := s.getSingleKubeJob()
	s.assertVolumesAndMountsAreConsistent(job)
	s.Len(job.Spec.Template.Spec.Volumes, 3, "blob, emptyDir and payload volumes")
	claimName := blobClaimName(job)
	s.Require().NotEmpty(claimName)
	pvc, err := s.kubeClient.CoreV1().PersistentVolumeClaims(volumesTestNamespace).Get(context.Background(), claimName, metav1.GetOptions{})
	s.Require().NoError(err, "the claim used by the job must exist")
	s.Equal(volumesTestJobComponent, pvc.Labels[kube.RadixComponentLabel])
	pv, err := s.kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(claimName, pv.Spec.ClaimRef.Name)
	s.Equal("container1", pv.Spec.CSI.VolumeAttributes["containerName"])
}

func (s *syncerTestSuite) Test_Volumes_BatchForOlderRdGetsStorageOfThatRd() {
	olderRd := volumesTestRd("older-rd", "oldcontainer")
	activeRd := volumesTestRd("active-rd", "newcontainer")
	activeRd.Status.Condition = radixv1.DeploymentActive
	batch := s.createVolumesTestResources(volumesTestBatch(olderRd.Name), olderRd, activeRd)

	sut := s.createSyncer(batch, config.Config{})
	s.Require().NoError(sut.OnSync(context.Background()))

	job := s.getSingleKubeJob()
	pvc, err := s.kubeClient.CoreV1().PersistentVolumeClaims(volumesTestNamespace).Get(context.Background(), blobClaimName(job), metav1.GetOptions{})
	s.Require().NoError(err)
	pv, err := s.kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal("oldcontainer", pv.Spec.CSI.VolumeAttributes["containerName"])
}

func (s *syncerTestSuite) Test_Volumes_JobSharesClaimOfAuxDeploymentWhenSpecMatches() {
	rd := volumesTestRd("any-rd", "container1")
	batch := s.createVolumesTestResources(volumesTestBatch(rd.Name), rd)

	// Create the hash-named pair, and a copy of it with other names, like a pair created by a previous operator version
	_, _, err := volumemount.EnsureComponentVolumes(context.Background(), s.kubeUtil, rd, &rd.Spec.Jobs[0], nil)
	s.Require().NoError(err)
	pvcs, err := s.kubeClient.CoreV1().PersistentVolumeClaims(volumesTestNamespace).List(context.Background(), metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().Len(pvcs.Items, 1)
	hashPvc := pvcs.Items[0]
	hashPv, err := s.kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), hashPvc.Spec.VolumeName, metav1.GetOptions{})
	s.Require().NoError(err)
	legacyPv := hashPv.DeepCopy()
	legacyPv.ObjectMeta = metav1.ObjectMeta{Name: "pv-legacy", Labels: hashPv.Labels, CreationTimestamp: metav1.NewTime(time.Now())}
	legacyPv.Spec.ClaimRef.Name = "pvc-legacy"
	legacyPv.Spec.CSI.VolumeHandle = "legacy-handle"
	_, err = s.kubeClient.CoreV1().PersistentVolumes().Create(context.Background(), legacyPv, metav1.CreateOptions{})
	s.Require().NoError(err)
	legacyPvc := hashPvc.DeepCopy()
	legacyPvc.ObjectMeta = metav1.ObjectMeta{Name: "pvc-legacy", Namespace: volumesTestNamespace, Labels: hashPvc.Labels, CreationTimestamp: metav1.NewTime(time.Now())}
	legacyPvc.Spec.VolumeName = legacyPv.Name
	_, err = s.kubeClient.CoreV1().PersistentVolumeClaims(volumesTestNamespace).Create(context.Background(), legacyPvc, metav1.CreateOptions{})
	s.Require().NoError(err)

	auxDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: defaults.GetJobAuxKubeDeployName(volumesTestJobComponent), Namespace: volumesTestNamespace},
		Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Volumes: []corev1.Volume{
			{Name: "any", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: legacyPvc.Name}}},
		}}}},
	}
	_, err = s.kubeClient.AppsV1().Deployments(volumesTestNamespace).Create(context.Background(), auxDeployment, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(batch, config.Config{})
	s.Require().NoError(sut.OnSync(context.Background()))

	job := s.getSingleKubeJob()
	s.Equal(legacyPvc.Name, blobClaimName(job), "the claim used by the aux deployment must be preferred")
	pvcs, err = s.kubeClient.CoreV1().PersistentVolumeClaims(volumesTestNamespace).List(context.Background(), metav1.ListOptions{})
	s.Require().NoError(err)
	s.Len(pvcs.Items, 2, "no new PVC must be created")
}

func (s *syncerTestSuite) Test_Volumes_NotEnsuredWhenNoJobIsCreated() {
	rd := volumesTestRd("any-rd", "container1")
	batch := volumesTestBatch(rd.Name)
	batch = s.createVolumesTestResources(batch, rd)
	existingJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: getKubeJobName(volumesTestBatchName, volumesTestBatchJobName), Namespace: volumesTestNamespace,
		Labels: radixlabels.Merge(radixlabels.ForBatchName(volumesTestBatchName), radixlabels.ForBatchJobName(volumesTestBatchJobName)),
	}}
	_, err := s.kubeClient.BatchV1().Jobs(volumesTestNamespace).Create(context.Background(), existingJob, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(batch, config.Config{})
	s.Require().NoError(sut.OnSync(context.Background()))

	pvcs, err := s.kubeClient.CoreV1().PersistentVolumeClaims(volumesTestNamespace).List(context.Background(), metav1.ListOptions{})
	s.Require().NoError(err)
	s.Empty(pvcs.Items)
	pvs, err := s.kubeClient.CoreV1().PersistentVolumes().List(context.Background(), metav1.ListOptions{})
	s.Require().NoError(err)
	s.Empty(pvs.Items)
}

func (s *syncerTestSuite) Test_Volumes_EnsuredOnRestart() {
	rd := volumesTestRd("any-rd", "container1")
	batch := volumesTestBatch(rd.Name)
	batch.Spec.Jobs[0].Restart = "2020-01-01T08:00:00Z"
	batch.Status = radixv1.RadixBatchStatus{
		Condition:   radixv1.RadixBatchCondition{Type: radixv1.BatchConditionTypeCompleted},
		JobStatuses: []radixv1.RadixBatchJobStatus{{Name: volumesTestBatchJobName, Phase: radixv1.BatchJobPhaseSucceeded}},
	}
	batch = s.createVolumesTestResources(batch, rd)

	sut := s.createSyncer(batch, config.Config{})
	s.Require().NoError(sut.OnSync(context.Background()))

	job := s.getSingleKubeJob()
	s.assertVolumesAndMountsAreConsistent(job)
	_, err := s.kubeClient.CoreV1().PersistentVolumeClaims(volumesTestNamespace).Get(context.Background(), blobClaimName(job), metav1.GetOptions{})
	s.NoError(err)
}

func (s *syncerTestSuite) createOrphanPvc(name string) {
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: volumesTestNamespace,
		Labels: map[string]string{kube.RadixAppLabel: volumesTestAppName, kube.RadixComponentLabel: volumesTestJobComponent, kube.RadixVolumeMountNameLabel: "removed"},
	}}
	_, err := s.kubeClient.CoreV1().PersistentVolumeClaims(volumesTestNamespace).Create(context.Background(), pvc, metav1.CreateOptions{})
	s.Require().NoError(err)
}

func (s *syncerTestSuite) pvcExists(name string) bool {
	_, err := s.kubeClient.CoreV1().PersistentVolumeClaims(volumesTestNamespace).Get(context.Background(), name, metav1.GetOptions{})
	return err == nil
}

func (s *syncerTestSuite) Test_Volumes_GarbageCollectedWhenBatchCompletes() {
	olderRd := volumesTestRd("older-rd", "oldcontainer")
	activeRd := volumesTestRd("active-rd", "newcontainer")
	activeRd.Status.Condition = radixv1.DeploymentActive
	batch := s.createVolumesTestResources(volumesTestBatch(olderRd.Name), olderRd, activeRd)
	s.createOrphanPvc("pvc-orphan")

	// The batch is active: garbage collection does not run, and the storage of the batch is created
	s.Require().NoError(s.createSyncer(batch, config.Config{}).OnSync(context.Background()))
	batchClaimName := blobClaimName(s.getSingleKubeJob())
	s.True(s.pvcExists("pvc-orphan"), "garbage collection must not run while the batch is not completed")
	s.True(s.pvcExists(batchClaimName))

	// Stopping the job deletes the Kubernetes Job, and completes the batch
	batch, err := s.radixClient.RadixV1().RadixBatches(volumesTestNamespace).Get(context.Background(), batch.Name, metav1.GetOptions{})
	s.Require().NoError(err)
	batch.Spec.Jobs[0].Stop = new(true)
	batch, err = s.radixClient.RadixV1().RadixBatches(volumesTestNamespace).Update(context.Background(), batch, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.Require().NoError(s.createSyncer(batch, config.Config{}).OnSync(context.Background()))
	batch, err = s.radixClient.RadixV1().RadixBatches(volumesTestNamespace).Get(context.Background(), batch.Name, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Require().Equal(radixv1.BatchConditionTypeCompleted, batch.Status.Condition.Type)

	s.False(s.pvcExists("pvc-orphan"))
	s.False(s.pvcExists(batchClaimName), "storage only used by the completed batch must be deleted")
}

func (s *syncerTestSuite) Test_Volumes_GarbageCollectionSkippedWithoutActiveRd() {
	rd := volumesTestRd("any-rd", "container1")
	batch := volumesTestBatch(rd.Name)
	batch.Spec.Jobs[0].Stop = new(true)
	batch = s.createVolumesTestResources(batch, rd)
	s.createOrphanPvc("pvc-orphan")

	s.Require().NoError(s.createSyncer(batch, config.Config{}).OnSync(context.Background()))
	batch, err := s.radixClient.RadixV1().RadixBatches(volumesTestNamespace).Get(context.Background(), batch.Name, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Require().Equal(radixv1.BatchConditionTypeCompleted, batch.Status.Condition.Type)
	s.True(s.pvcExists("pvc-orphan"))
}
