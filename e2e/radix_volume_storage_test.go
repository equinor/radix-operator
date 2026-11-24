package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/equinor/radix-operator/e2e/internal"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/apis/utils/random"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	volumeTestTimeout      = 5 * time.Minute
	volumeTestPollInterval = 2 * time.Second
	// volumeTestDeletionGracePeriod must match config.operator.volumeDeletionGracePeriod in testdata/radix-operator-values.yaml
	volumeTestDeletionGracePeriod = 5 * time.Second
	volumeTestEnvName             = "dev"
	volumeTestComponentName       = "web"
	volumeTestMountPath           = "/mnt/blob"
	volumeTestImage               = "docker.io/nginxinc/nginx-unprivileged:1.27-alpine"
)

// TestVolumeMountIsMountedByCSIDriver deploys a component with a BlobFuse2 volume mount, and verifies that the
// operator creates a PV/PVC pair that Kubernetes binds, and that the Pod gets the volume mounted by the
// (fake) blob.csi.azure.com driver with the volume attributes and mount options of the PV.
func TestVolumeMountIsMountedByCSIDriver(t *testing.T) {
	t.Parallel()
	c := getClient(t)
	const appName = "volume-mount-test"

	envNamespace := setupVolumeTestApp(t, c, appName, "container1")
	rd := createVolumeTestRadixDeployment(t, c, appName, envNamespace, "rd-1", volumeTestVolumeMount("container1"))
	waitForActiveRadixDeployment(t, c, envNamespace, rd.Name)

	pod := waitForComponentPodWithClaim(t, c, envNamespace, "")
	claimName := podClaimName(pod)
	require.NotEmpty(t, claimName, "the pod should use a PersistentVolumeClaim")

	pvc := &corev1.PersistentVolumeClaim{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: claimName}, pvc))
	assert.Equal(t, corev1.ClaimBound, pvc.Status.Phase, "the PVC should be bound by Kubernetes")
	pv := &corev1.PersistentVolume{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Name: pvc.Spec.VolumeName}, pv))
	assert.Equal(t, corev1.VolumeBound, pv.Status.Phase)
	require.NotNil(t, pv.Spec.ClaimRef)
	assert.Equal(t, pvc.UID, pv.Spec.ClaimRef.UID, "the PV should be bound to the PVC")
	assert.True(t, strings.HasPrefix(pvc.Name, "pvc-web-blob-"), "new PVCs should be named pvc-<component>-<volumeMount>-<ulid>, got %s", pvc.Name)
	assert.True(t, strings.HasPrefix(pv.Name, "pv-"+appName+"-"), "new PVs should be named pv-<app>-<ulid>, got %s", pv.Name)

	info := readVolumeInfoFromPodLogs(t, envNamespace, pod.Name)
	assert.Equal(t, pv.Spec.CSI.VolumeHandle, info.VolumeID, "the driver should mount the volume handle of the PV")
	assert.Equal(t, "container1", info.VolumeContext["containerName"])
	assert.Equal(t, "fuse2", info.VolumeContext["protocol"])
	assert.ElementsMatch(t, pv.Spec.MountOptions, info.MountFlags, "the driver should receive the mount options of the PV")
	assert.Contains(t, info.MountFlags, "--read-only=true")
}

// TestVolumeMountSpecChange changes the storage spec of a component, and verifies that a new PV/PVC pair is
// created and used by the rolled out Pod, and that the old pair is deleted by a RadixDeployment sync after the deletion grace period,
// once the ReplicaSet of the previous revision is scaled down.
func TestVolumeMountSpecChange(t *testing.T) {
	t.Parallel()
	c := getClient(t)
	const appName = "volume-spec-change-test"

	envNamespace := setupVolumeTestApp(t, c, appName, "container1")
	rd1 := createVolumeTestRadixDeployment(t, c, appName, envNamespace, "rd-1", volumeTestVolumeMount("container1"))
	waitForActiveRadixDeployment(t, c, envNamespace, rd1.Name)
	pod1 := waitForComponentPodWithClaim(t, c, envNamespace, "")
	claim1 := podClaimName(pod1)
	pvc1 := &corev1.PersistentVolumeClaim{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: claim1}, pvc1))
	pv1Name := pvc1.Spec.VolumeName

	rd2 := createVolumeTestRadixDeployment(t, c, appName, envNamespace, "rd-2", volumeTestVolumeMount("container2"))
	waitForActiveRadixDeployment(t, c, envNamespace, rd2.Name)

	deployment := &appsv1.Deployment{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: volumeTestComponentName}, deployment))
	claim2 := podClaimName(&corev1.Pod{Spec: deployment.Spec.Template.Spec})
	require.NotEmpty(t, claim2)
	require.NotEqual(t, claim1, claim2, "a changed spec should use a new PVC")

	pod2 := waitForComponentPodWithClaim(t, c, envNamespace, claim2)
	info := readVolumeInfoFromPodLogs(t, envNamespace, pod2.Name)
	assert.Equal(t, "container2", info.VolumeContext["containerName"])

	waitForVolumeResourcesDeleted(t, c, envNamespace, rd2.Name, claim1, pv1Name)

	pvc2 := &corev1.PersistentVolumeClaim{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: claim2}, pvc2), "the PVC in use should be kept")
	assert.Equal(t, corev1.ClaimBound, pvc2.Status.Phase)
	assert.Nil(t, pvc2.DeletionTimestamp, "the PVC in use should not be deleted")
}

// TestVolumesOfRemovedComponentAreDeleted removes a component with a BlobFuse2 volume mount from the RadixDeployment,
// and verifies that its PV/PVC pair is deleted.
func TestVolumesOfRemovedComponentAreDeleted(t *testing.T) {
	t.Parallel()
	c := getClient(t)
	const appName = "volume-removed-component-test"

	envNamespace := setupVolumeTestApp(t, c, appName, "container1")
	rd1 := createVolumeTestRadixDeployment(t, c, appName, envNamespace, "rd-1", volumeTestVolumeMount("container1"))
	waitForActiveRadixDeployment(t, c, envNamespace, rd1.Name)
	pod := waitForComponentPodWithClaim(t, c, envNamespace, "")
	pvc := &corev1.PersistentVolumeClaim{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: podClaimName(pod)}, pvc))

	otherComponent := volumeTestComponent(nil)
	otherComponent.Name = "other"
	rd2 := createVolumeTestRadixDeploymentWithSpec(t, c, appName, envNamespace, "rd-2", []v1.RadixDeployComponent{otherComponent}, nil)
	waitForActiveRadixDeployment(t, c, envNamespace, rd2.Name)

	waitForVolumeResourcesDeleted(t, c, envNamespace, rd2.Name, pvc.Name, pvc.Spec.VolumeName)
}

// TestVolumeOfUnfinishedJobIsKeptUntilJobIsStopped starts a job from a RadixBatch, changes the storage spec of the job
// in a new RadixDeployment, and verifies that a RadixDeployment sync after the deletion grace period keeps the PV/PVC of the batch
// while its Kubernetes Job has not finished, as the Job can still create Pods that need them, and deletes them when the job is stopped.
func TestVolumeOfUnfinishedJobIsKeptUntilJobIsStopped(t *testing.T) {
	t.Parallel()
	c := getClient(t)
	const appName, jobName, batchName = "volume-batch-test", "compute", "batch-1"

	envNamespace := setupVolumeTestApp(t, c, appName, "container1")
	rd1 := createVolumeTestRadixDeploymentWithSpec(t, c, appName, envNamespace, "rd-1", nil, []v1.RadixDeployJobComponent{volumeTestJobComponent(jobName, "container1")})
	waitForActiveRadixDeployment(t, c, envNamespace, rd1.Name)

	// Labels as set by the job scheduler; the operator only syncs RadixBatches with the radix-app label
	batch := &v1.RadixBatch{
		Name: batchName, Namespace: envNamespace,
		Labels: radixlabels.Merge(radixlabels.ForApplicationName(appName), radixlabels.ForComponentName(jobName), radixlabels.ForBatchType(kube.RadixBatchTypeJob)),
		Spec: v1.RadixBatchSpec{
			RadixDeploymentJobRef: v1.RadixDeploymentJobComponentSelector{LocalObjectReference: v1.LocalObjectReference{Name: rd1.Name}, Job: jobName},
			Jobs:                  []v1.RadixBatchJob{{Name: "job1"}},
		},
	}
	require.NoError(t, c.Create(t.Context(), batch), "should create RadixBatch")
	jobPod := waitForBatchPodReady(t, c, envNamespace, batchName)
	claim1 := podClaimName(jobPod)
	require.NotEmpty(t, claim1, "the job pod should use a PersistentVolumeClaim")
	pvc1 := &corev1.PersistentVolumeClaim{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: claim1}, pvc1))
	pv1Name := pvc1.Spec.VolumeName
	info := readVolumeInfoFromPodLogs(t, envNamespace, jobPod.Name)
	assert.Equal(t, "container1", info.VolumeContext["containerName"])
	auxDeployment := &appsv1.Deployment{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: defaults.GetJobAuxKubeDeployName(jobName)}, auxDeployment))
	assert.Equal(t, claim1, podClaimName(&corev1.Pod{Spec: auxDeployment.Spec.Template.Spec}), "the job-aux deployment should use the claim of the job")
	require.Len(t, auxDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, 1, "the job-aux container should mount the job volume, as a canary for the jobs")
	assert.Equal(t, volumeTestMountPath, auxDeployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath)

	rd2 := createVolumeTestRadixDeploymentWithSpec(t, c, appName, envNamespace, "rd-2", nil, []v1.RadixDeployJobComponent{volumeTestJobComponent(jobName, "container2")})
	waitForActiveRadixDeployment(t, c, envNamespace, rd2.Name)

	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: defaults.GetJobAuxKubeDeployName(jobName)}, auxDeployment))
	require.NotEqual(t, claim1, podClaimName(&corev1.Pod{Spec: auxDeployment.Spec.Template.Spec}), "the job-aux deployment should use a new claim for the changed spec")

	// The PVC of the batch does not match the job spec of the active RadixDeployment, but is kept while the Kubernetes Job has not finished.
	// Sync after the deletion grace period (creationTimestamp has a precision of seconds), so only the Job protects the PVC.
	time.Sleep(time.Until(pvc1.CreationTimestamp.Add(volumeTestDeletionGracePeriod + time.Second)))
	resyncRadixDeployment(t, c, envNamespace, rd2.Name)
	currentPvc1 := &corev1.PersistentVolumeClaim{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: claim1}, currentPvc1), "the PVC of the unfinished job should be kept")
	assert.Nil(t, currentPvc1.DeletionTimestamp, "the PVC of the unfinished job should not be deleted")
	currentPv1 := &corev1.PersistentVolume{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Name: pv1Name}, currentPv1), "the PV of the unfinished job should be kept")
	assert.Nil(t, currentPv1.DeletionTimestamp, "the PV of the unfinished job should not be deleted")
	runningPod := &corev1.Pod{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: jobPod.Name}, runningPod))
	assert.True(t, isPodReady(runningPod), "the job pod should keep running")
	assert.Nil(t, runningPod.DeletionTimestamp)

	// Stopping the job deletes the Kubernetes Job and its Pod. Once the previous revision of the job-aux Deployment is gone too,
	// the next RadixDeployment sync deletes the PVC and PV
	require.NoError(t, retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &v1.RadixBatch{}
		if err := c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: batchName}, current); err != nil {
			return err
		}
		current.Spec.Jobs[0].Stop = new(true)
		return c.Update(t.Context(), current)
	}), "should stop the batch job")
	// TODO: Remove when the job-aux Deployment is architecture agnostic, see deleteReplicaSetsUsingClaim
	deleteReplicaSetsUsingClaim(t, c, envNamespace, claim1)
	waitForVolumeResourcesDeleted(t, c, envNamespace, rd2.Name, claim1, pv1Name)
}

// deleteReplicaSetsUsingClaim deletes the ReplicaSets in the namespace whose Pod template uses the PVC.
// The job-aux Deployment requires an amd64 node, so its Pods never start on a kind node with another architecture,
// e.g. arm64 on macOS. Its rollout then never completes, and the ReplicaSet of the previous revision keeps its replicas
// and protects the PVC of the previous job spec. Deleting the ReplicaSet simulates that the previous revision is scaled down.
//
// TODO: Temporary workaround. Remove this function and its call when the job-aux Deployment is architecture agnostic
// (handled in a separate PR), as the job-aux rollout then completes on any kind node.
func deleteReplicaSetsUsingClaim(t *testing.T, c client.Client, namespace, claimName string) {
	t.Helper()
	replicaSets := &appsv1.ReplicaSetList{}
	require.NoError(t, c.List(t.Context(), replicaSets, client.InNamespace(namespace)))
	for _, replicaSet := range replicaSets.Items {
		if podClaimName(&corev1.Pod{Spec: replicaSet.Spec.Template.Spec}) != claimName {
			continue
		}
		err := c.Delete(t.Context(), &replicaSet, client.PropagationPolicy(metav1.DeletePropagationBackground))
		require.NoError(t, client.IgnoreNotFound(err), "should delete ReplicaSet %s", replicaSet.Name)
	}
}

// TestLegacyVolumesAreReusedWhenMatching creates PV/PVC pairs as created by previous operator versions, before the
// component is deployed: one that matches the desired spec, and one for an older spec. It verifies that the operator
// reuses the matching pair as is, without creating a new pair or changing it, that the pair for the older spec is deleted,
// and that the workload is not rolled out when the RadixDeployment syncs again.
func TestLegacyVolumesAreReusedWhenMatching(t *testing.T) {
	t.Parallel()
	c := getClient(t)
	const appName = "volume-legacy-test"

	volumeMount := volumeTestVolumeMount("container1")
	volumeMount.BlobFuse2.BlockCacheOptions = &v1.BlobFuse2BlockCacheOptions{DiskSize: new(uint32(100))}
	envNamespace := setupVolumeTestApp(t, c, appName, "container1")

	oldSpecPv, oldSpecPvc := createLegacyVolumePair(t, c, appName, envNamespace, "container0")
	legacyPv, legacyPvc := createLegacyVolumePair(t, c, appName, envNamespace, "container1")
	for _, pvcName := range []string{oldSpecPvc.Name, legacyPvc.Name} {
		require.NoError(t, wait.PollUntilContextTimeout(t.Context(), volumeTestPollInterval, volumeTestTimeout, true, func(ctx context.Context) (bool, error) {
			pvc := &corev1.PersistentVolumeClaim{}
			return c.Get(ctx, client.ObjectKey{Namespace: envNamespace, Name: pvcName}, pvc) == nil && pvc.Status.Phase == corev1.ClaimBound, nil
		}), "legacy PVC %s should be bound", pvcName)
	}

	rd := createVolumeTestRadixDeployment(t, c, appName, envNamespace, "rd-1", volumeMount)
	waitForActiveRadixDeployment(t, c, envNamespace, rd.Name)
	pod := waitForComponentPodWithClaim(t, c, envNamespace, legacyPvc.Name)

	// The pair for the older spec is not used by any Pod, and is removed by the first sync after the deletion grace period
	waitForVolumeResourcesDeleted(t, c, envNamespace, rd.Name, oldSpecPvc.Name, oldSpecPv.Name)
	pvcs := &corev1.PersistentVolumeClaimList{}
	require.NoError(t, c.List(t.Context(), pvcs, client.InNamespace(envNamespace)))
	var pvcNames []string
	for _, pvc := range pvcs.Items {
		pvcNames = append(pvcNames, pvc.Name)
	}
	assert.Equal(t, []string{legacyPvc.Name}, pvcNames, "no new PVC should be created")

	currentLegacyPv := &corev1.PersistentVolume{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Name: legacyPv.Name}, currentLegacyPv))
	assert.Equal(t, legacyPv.Spec.CSI.VolumeHandle, currentLegacyPv.Spec.CSI.VolumeHandle, "the legacy PV should not be changed")
	assert.Equal(t, legacyPv.Spec.MountOptions, currentLegacyPv.Spec.MountOptions, "the legacy PV should not be changed")
	info := readVolumeInfoFromPodLogs(t, envNamespace, pod.Name)
	assert.Equal(t, legacyPv.Spec.CSI.VolumeHandle, info.VolumeID, "the pod should mount the legacy PV")
	assert.ElementsMatch(t, legacyPv.Spec.MountOptions, info.MountFlags)

	deployment := &appsv1.Deployment{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: volumeTestComponentName}, deployment))
	generation := deployment.Generation

	resyncRadixDeployment(t, c, envNamespace, rd.Name)

	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: volumeTestComponentName}, deployment))
	assert.Equal(t, generation, deployment.Generation, "the deployment should not be rolled out when the RadixDeployment syncs again")
	assert.Equal(t, legacyPvc.Name, podClaimName(&corev1.Pod{Spec: deployment.Spec.Template.Spec}))
	currentLegacyPvc := &corev1.PersistentVolumeClaim{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: legacyPvc.Name}, currentLegacyPvc), "the legacy PVC in use should be kept")
	assert.Nil(t, currentLegacyPvc.DeletionTimestamp, "the legacy PVC in use should not be deleted")
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Name: legacyPv.Name}, currentLegacyPv), "the legacy PV in use should be kept")
	assert.Nil(t, currentLegacyPv.DeletionTimestamp, "the legacy PV in use should not be deleted")
}

// waitForVolumeResourcesDeleted waits until the PVC and the PV are gone.
// See waitForConditionWhileResyncingRadixDeployment for why the RadixDeployment is synced while waiting.
func waitForVolumeResourcesDeleted(t *testing.T, c client.Client, namespace, rdName, pvcName, pvName string) {
	t.Helper()
	err := waitForConditionWhileResyncingRadixDeployment(t.Context(), c, namespace, rdName, func(ctx context.Context) bool {
		pvcErr := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: pvcName}, &corev1.PersistentVolumeClaim{})
		pvErr := c.Get(ctx, client.ObjectKey{Name: pvName}, &corev1.PersistentVolume{})
		return apierrors.IsNotFound(pvcErr) && apierrors.IsNotFound(pvErr)
	})
	if err != nil {
		logRadixOperatorLogs(t, c, "PersistentVolume")
	}
	require.NoError(t, err, "the PVC %s and PV %s should be deleted", pvcName, pvName)
}

// waitForConditionWhileResyncingRadixDeployment waits until condition is true, and triggers a sync of the RadixDeployment
// at each poll while it is false. Unused PVs/PVCs created less than volumeDeletionGracePeriod ago are kept by a sync, and the
// operator does not sync the RadixDeployment again by itself, so a sync is triggered until the grace period has passed.
func waitForConditionWhileResyncingRadixDeployment(ctx context.Context, c client.Client, namespace, rdName string, condition func(ctx context.Context) bool) error {
	return wait.PollUntilContextTimeout(ctx, volumeTestPollInterval, volumeTestTimeout, true, func(ctx context.Context) (bool, error) {
		if condition(ctx) {
			return true, nil
		}
		_ = triggerRadixDeploymentSync(ctx, c, namespace, rdName)
		return false, nil
	})
}

// triggerRadixDeploymentSync changes an annotation of the RadixDeployment, which makes the operator sync it
func triggerRadixDeploymentSync(ctx context.Context, c client.Client, namespace, name string) error {
	patch := fmt.Appendf(nil, `{"metadata":{"annotations":{"e2e/resync":%q}}}`, time.Now().Format(time.RFC3339Nano))
	return c.Patch(ctx, &v1.RadixDeployment{Namespace: namespace, Name: name}, client.RawPatch(types.MergePatchType, patch))
}

// resyncRadixDeployment triggers a sync of the RadixDeployment, and waits until it has been reconciled
func resyncRadixDeployment(t *testing.T, c client.Client, namespace, name string) {
	t.Helper()
	rd := &v1.RadixDeployment{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: namespace, Name: name}, rd))
	lastReconciled := rd.Status.Reconciled
	// Reconciled has a precision of seconds, so the sync is triggered until it changes
	err := wait.PollUntilContextTimeout(t.Context(), volumeTestPollInterval, volumeTestTimeout, true, func(ctx context.Context) (bool, error) {
		if err := triggerRadixDeploymentSync(ctx, c, namespace, name); err != nil {
			return false, nil
		}
		current := &v1.RadixDeployment{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, current); err != nil {
			return false, nil
		}
		return !current.Status.Reconciled.Equal(&lastReconciled) && current.Status.ReconcileStatus == v1.RadixDeploymentReconcileSucceeded, nil
	})
	require.NoError(t, err, "RadixDeployment %s should be reconciled again", name)
}

// waitForBatchPodReady waits for a single running and ready Pod of the RadixBatch
func waitForBatchPodReady(t *testing.T, c client.Client, namespace, batchName string) *corev1.Pod {
	t.Helper()
	var readyPod *corev1.Pod
	err := wait.PollUntilContextTimeout(t.Context(), volumeTestPollInterval, volumeTestTimeout, true, func(ctx context.Context) (bool, error) {
		pods := &corev1.PodList{}
		if err := c.List(ctx, pods, client.InNamespace(namespace), client.MatchingLabels{kube.RadixBatchNameLabel: batchName}); err != nil {
			return false, nil
		}
		if len(pods.Items) != 1 || !isPodReady(&pods.Items[0]) {
			return false, nil
		}
		readyPod = &pods.Items[0]
		return true, nil
	})
	if err != nil {
		logRadixOperatorLogs(t, c, batchName)
		logPodLogs(t, "fake-blob-csi", "app=fake-blob-csi-node", "")
	}
	require.NoError(t, err, "a single ready pod for RadixBatch %s should exist", batchName)
	return readyPod
}

// createLegacyVolumePair creates a PV and PVC for the BlobFuse2 volume mount "blob" of the component, with a block cache disk,
// as created by previous operator versions: the PVC name has a random suffix, the PV name and volume handle contain a UUID,
// and the block cache path contains another UUID. The PV also has volume attributes that are set by a provisioner.
func createLegacyVolumePair(t *testing.T, c client.Client, appName, namespace, container string) (*corev1.PersistentVolume, *corev1.PersistentVolumeClaim) {
	t.Helper()
	const volumeMountName = "blob"
	pvcName := fmt.Sprintf("pvc-csi-blobfuse2-fuse2-%s-%s-%s-%s", volumeTestComponentName, volumeMountName, container, strings.ToLower(random.RandString(5)))
	pvName := "pv-radixvolumemount-" + uuid.NewString()
	volumeHandle := fmt.Sprintf("radixvolumemount#%s#%s#%s#%s#%s", appName, volumeTestEnvName, volumeTestComponentName, volumeMountName, uuid.NewString())

	pv := &corev1.PersistentVolume{
		Name: pvName,
		Labels: map[string]string{
			kube.RadixAppLabel:             appName,
			kube.RadixNamespace:            namespace,
			kube.RadixComponentLabel:       volumeTestComponentName,
			kube.RadixVolumeMountNameLabel: volumeMountName,
		},
		Spec: corev1.PersistentVolumeSpec{
			StorageClassName:              "",
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			Capacity:                      corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")},
			AccessModes:                   []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
			MountOptions: []string{
				"--disable-writeback-cache=true", "--cancel-list-on-mount-seconds=0", "--allow-other", "--attr-timeout=0", "--entry-timeout=0",
				"--negative-timeout=0", "--use-adls=false", "--read-only=true", "--attr-cache-timeout=0",
				"--block-cache", "--block-cache-block-size=4", "--block-cache-pool-size=48", "--block-cache-prefetch=11",
				"--block-cache-prefetch-on-open=false", "--block-cache-parallelism=8",
				fmt.Sprintf("--block-cache-path=/mnt/radixvolumemount#%s#%s#%s#%s#%s#blockcache", appName, volumeTestEnvName, volumeTestComponentName, volumeMountName, uuid.NewString()),
				"--block-cache-disk-size=100", "--block-cache-disk-timeout=120",
			},
			ClaimRef: &corev1.ObjectReference{APIVersion: "v1", Kind: "PersistentVolumeClaim", Namespace: namespace, Name: pvcName},
			PersistentVolumeSource: corev1.PersistentVolumeSource{CSI: &corev1.CSIPersistentVolumeSource{
				Driver:       "blob.csi.azure.com",
				VolumeHandle: volumeHandle,
				VolumeAttributes: map[string]string{
					"containerName":              container,
					"protocol":                   "fuse2",
					"secretnamespace":            namespace,
					"csi.storage.k8s.io/pv/name": pvName,
					"storage.kubernetes.io/csiProvisionerIdentity": "1700000000000-1234-blob.csi.azure.com",
				},
				NodeStageSecretRef: &corev1.SecretReference{Name: defaults.GetCsiAzureVolumeMountCredsSecretName(volumeTestComponentName, volumeMountName), Namespace: namespace},
			}},
		},
	}
	require.NoError(t, c.Create(t.Context(), pv), "should create legacy PV")

	pvc := &corev1.PersistentVolumeClaim{
		Name: pvcName, Namespace: namespace,
		Labels: map[string]string{
			kube.RadixAppLabel:             appName,
			kube.RadixComponentLabel:       volumeTestComponentName,
			kube.RadixVolumeMountNameLabel: volumeMountName,
			kube.RadixMountTypeLabel:       string(v1.MountTypeBlobFuse2Fuse2CsiAzure),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
			Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")}},
			VolumeName:       pvName,
			StorageClassName: new(""),
			VolumeMode:       new(corev1.PersistentVolumeFilesystem),
		},
	}
	require.NoError(t, c.Create(t.Context(), pvc), "should create legacy PVC")
	return pv, pvc
}

// setupVolumeTestApp creates a RadixRegistration and a RadixApplication with a component with a BlobFuse2 volume mount,
// and waits for the environment namespace to be created
func setupVolumeTestApp(t *testing.T, c client.Client, appName, container string) string {
	t.Helper()
	readyCtx, cancelReady := context.WithTimeout(t.Context(), volumeTestTimeout)
	defer cancelReady()
	require.NoError(t, WaitForDeploymentReady(readyCtx, c, "radix-system", "radix-operator"), "radix-operator deployment should be ready")

	appNamespace := createRadixRegistrationAndNamespaceForTest(t, c, appName)
	ra := &v1.RadixApplication{
		Name: appName, Namespace: appNamespace,
		Spec: v1.RadixApplicationSpec{
			Environments: []v1.Environment{{Name: volumeTestEnvName}},
			Components: []v1.RadixComponent{{
				Name:         volumeTestComponentName,
				Image:        volumeTestImage,
				Runtime:      volumeTestRuntime(),
				VolumeMounts: []v1.RadixVolumeMount{volumeTestVolumeMount(container)},
			}},
		},
	}
	err := createWhenWebhookCacheIsSynced(t, "no corresponding radix application found", func(ctx context.Context) error {
		return c.Create(ctx, ra.DeepCopy())
	})
	require.NoError(t, err, "should create RadixApplication")

	envNamespace := utils.GetEnvironmentNamespace(appName, volumeTestEnvName)
	require.NoError(t, wait.PollUntilContextTimeout(t.Context(), volumeTestPollInterval, volumeTestTimeout, true, func(ctx context.Context) (bool, error) {
		err := c.Get(ctx, client.ObjectKey{Name: envNamespace}, &corev1.Namespace{})
		return err == nil, nil
	}), "environment namespace %s should be created", envNamespace)
	return envNamespace
}

// volumeTestRuntime schedules the component on the architecture of the local kind node
func volumeTestRuntime() *v1.Runtime {
	return &v1.Runtime{Architecture: v1.RuntimeArchitecture(runtime.GOARCH)}
}

func volumeTestVolumeMount(container string) v1.RadixVolumeMount {
	return v1.RadixVolumeMount{
		Name:      "blob",
		Path:      volumeTestMountPath,
		BlobFuse2: &v1.RadixBlobFuse2VolumeMount{Container: container},
	}
}

// volumeTestComponent returns the component, which prints the volume info written by the fake CSI driver,
// so the test can read it from the Pod logs
func volumeTestComponent(volumeMounts []v1.RadixVolumeMount) v1.RadixDeployComponent {
	return v1.RadixDeployComponent{
		Name:         volumeTestComponentName,
		Image:        volumeTestImage,
		Replicas:     new(1),
		RunAsUser:    new(int64(101)),
		Runtime:      volumeTestRuntime(),
		Command:      volumeTestCommand(volumeMounts),
		VolumeMounts: volumeMounts,
	}
}

// volumeTestJobComponent returns a job with a BlobFuse2 volume mount, which prints the volume info written by the fake CSI driver
func volumeTestJobComponent(name, container string) v1.RadixDeployJobComponent {
	volumeMounts := []v1.RadixVolumeMount{volumeTestVolumeMount(container)}
	return v1.RadixDeployJobComponent{
		Name:         name,
		Image:        volumeTestImage,
		RunAsUser:    new(int64(101)),
		Runtime:      volumeTestRuntime(),
		Command:      volumeTestCommand(volumeMounts),
		VolumeMounts: volumeMounts,
	}
}

func volumeTestCommand(volumeMounts []v1.RadixVolumeMount) []string {
	if len(volumeMounts) == 0 {
		return []string{"sh", "-c", "sleep 100000"}
	}
	return []string{"sh", "-c", fmt.Sprintf("cat %s/%s && echo && sleep 100000", volumeTestMountPath, internal.FakeBlobCSIVolumeInfoFileName)}
}

func createVolumeTestRadixDeployment(t *testing.T, c client.Client, appName, envNamespace, rdName string, volumeMount v1.RadixVolumeMount) *v1.RadixDeployment {
	t.Helper()
	return createVolumeTestRadixDeploymentWithSpec(t, c, appName, envNamespace, rdName, []v1.RadixDeployComponent{volumeTestComponent([]v1.RadixVolumeMount{volumeMount})}, nil)
}

func createVolumeTestRadixDeploymentWithSpec(t *testing.T, c client.Client, appName, envNamespace, rdName string, components []v1.RadixDeployComponent, jobs []v1.RadixDeployJobComponent) *v1.RadixDeployment {
	t.Helper()
	rd := &v1.RadixDeployment{
		Name: rdName, Namespace: envNamespace,
		Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixEnvLabel: volumeTestEnvName},
		Spec: v1.RadixDeploymentSpec{
			AppName:     appName, //nolint:staticcheck // required by the CRD
			Environment: volumeTestEnvName,
			Components:  components,
			Jobs:        jobs,
		},
	}
	require.NoError(t, c.Create(t.Context(), rd), "should create RadixDeployment %s", rdName)
	return rd
}

func waitForActiveRadixDeployment(t *testing.T, c client.Client, namespace, name string) {
	t.Helper()
	err := wait.PollUntilContextTimeout(t.Context(), volumeTestPollInterval, volumeTestTimeout, true, func(ctx context.Context) (bool, error) {
		rd := &v1.RadixDeployment{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, rd); err != nil {
			return false, nil
		}
		return rd.Status.Condition == v1.DeploymentActive && rd.Status.ReconcileStatus == v1.RadixDeploymentReconcileSucceeded && rd.Status.ObservedGeneration == rd.Generation, nil
	})
	if err != nil {
		logRadixOperatorLogs(t, c, name)
	}
	require.NoError(t, err, "RadixDeployment %s should be active and reconciled", name)
}

// waitForComponentPodWithClaim waits for a single running and ready Pod of the component, mounting claimName if it is set
func waitForComponentPodWithClaim(t *testing.T, c client.Client, namespace, claimName string) *corev1.Pod {
	t.Helper()
	var readyPod *corev1.Pod
	err := wait.PollUntilContextTimeout(t.Context(), volumeTestPollInterval, volumeTestTimeout, true, func(ctx context.Context) (bool, error) {
		pods := &corev1.PodList{}
		if err := c.List(ctx, pods, client.InNamespace(namespace), client.MatchingLabels{kube.RadixComponentLabel: volumeTestComponentName}); err != nil {
			return false, nil
		}
		var matching []corev1.Pod
		for _, pod := range pods.Items {
			if pod.DeletionTimestamp != nil {
				return false, nil
			}
			if claimName != "" && podClaimName(&pod) != claimName {
				return false, nil
			}
			matching = append(matching, pod)
		}
		if len(matching) != 1 || !isPodReady(&matching[0]) {
			return false, nil
		}
		readyPod = &matching[0]
		return true, nil
	})
	if err != nil {
		t.Logf("pods: %s", describeDeploymentPods(t.Context(), c, namespace, volumeTestComponentName))
		logPodLogs(t, "fake-blob-csi", "app=fake-blob-csi-node", "")
	}
	require.NoError(t, err, "a single ready pod for %s should exist", volumeTestComponentName)
	return readyPod
}

func podClaimName(pod *corev1.Pod) string {
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			return volume.PersistentVolumeClaim.ClaimName
		}
	}
	return ""
}

// readVolumeInfoFromPodLogs reads the volume info written by the fake CSI driver, which the component prints at startup
func readVolumeInfoFromPodLogs(t *testing.T, namespace, podName string) internal.FakeBlobCSIVolumeInfo {
	t.Helper()
	clientset, err := kubernetes.NewForConfig(testManager.GetConfig())
	require.NoError(t, err)
	var info internal.FakeBlobCSIVolumeInfo
	err = wait.PollUntilContextTimeout(t.Context(), volumeTestPollInterval, time.Minute, true, func(ctx context.Context) (bool, error) {
		raw, err := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{}).DoRaw(ctx)
		if err != nil {
			return false, nil
		}
		line, _, _ := strings.Cut(strings.TrimSpace(string(raw)), "\n")
		return json.Unmarshal([]byte(line), &info) == nil, nil
	})
	require.NoError(t, err, "pod %s should print the volume info", podName)
	return info
}
