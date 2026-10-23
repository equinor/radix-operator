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
	"github.com/equinor/radix-operator/pkg/apis/utils/random"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	volumeTestTimeout       = 5 * time.Minute
	volumeTestPollInterval  = 2 * time.Second
	volumeTestEnvName       = "dev"
	volumeTestComponentName = "web"
	volumeTestMountPath     = "/mnt/blob"
	volumeTestImage         = "docker.io/nginxinc/nginx-unprivileged:1.27-alpine"
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
	assert.True(t, strings.HasPrefix(pvc.Name, "pvc-csi-blobfuse2-fuse2-web-blob-container1-"), "new PVCs should be named after a hash of the spec, got %s", pvc.Name)

	info := readVolumeInfoFromPodLogs(t, envNamespace, pod.Name)
	assert.Equal(t, pv.Spec.CSI.VolumeHandle, info.VolumeID, "the driver should mount the volume handle of the PV")
	assert.Equal(t, "container1", info.VolumeContext["containerName"])
	assert.Equal(t, "fuse2", info.VolumeContext["protocol"])
	assert.ElementsMatch(t, pv.Spec.MountOptions, info.MountFlags, "the driver should receive the mount options of the PV")
	assert.Contains(t, info.MountFlags, "--read-only=true")
}

// TestVolumeMountSpecChange changes the storage spec of a component, and verifies that a new PV/PVC pair is
// created and used by the rolled out Pod, and that the old pair is deleted after the rollout.
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
	claim2 := ""
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			claim2 = volume.PersistentVolumeClaim.ClaimName
		}
	}
	require.NotEmpty(t, claim2)
	require.NotEqual(t, claim1, claim2, "a changed spec should use a new PVC")

	pod2 := waitForComponentPodWithClaim(t, c, envNamespace, claim2)
	info := readVolumeInfoFromPodLogs(t, envNamespace, pod2.Name)
	assert.Equal(t, "container2", info.VolumeContext["containerName"])

	// Garbage collection runs when the RadixDeployment syncs, and keeps the old pair while a Pod mounts it.
	// Trigger syncs until the old pair is deleted after the rollout.
	err := wait.PollUntilContextTimeout(t.Context(), 5*time.Second, volumeTestTimeout, true, func(ctx context.Context) (bool, error) {
		patch := fmt.Appendf(nil, `{"metadata":{"annotations":{"e2e/resync":%q}}}`, time.Now().Format(time.RFC3339Nano))
		if err := c.Patch(ctx, &v1.RadixDeployment{Namespace: envNamespace, Name: rd2.Name}, client.RawPatch(types.MergePatchType, patch)); err != nil {
			return false, nil
		}
		pvcErr := c.Get(ctx, client.ObjectKey{Namespace: envNamespace, Name: claim1}, &corev1.PersistentVolumeClaim{})
		pvErr := c.Get(ctx, client.ObjectKey{Name: pv1Name}, &corev1.PersistentVolume{})
		return apierrors.IsNotFound(pvcErr) && apierrors.IsNotFound(pvErr), nil
	})
	if err != nil {
		logRadixOperatorLogs(t, c, "PersistentVolume")
	}
	require.NoError(t, err, "the old PVC %s and PV %s should be deleted after the rollout", claim1, pv1Name)

	pvc2 := &corev1.PersistentVolumeClaim{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: claim2}, pvc2), "the PVC in use should be kept")
	assert.Equal(t, corev1.ClaimBound, pvc2.Status.Phase)
}

// TestVolumesAreDeletedWithApplication deletes an application, and verifies that the PersistentVolumes of its
// environment namespace are deleted. PersistentVolumes are cluster scoped, and are not deleted with the namespace.
func TestVolumesAreDeletedWithApplication(t *testing.T) {
	t.Parallel()
	c := getClient(t)
	const appName = "volume-app-delete-test"

	envNamespace := setupVolumeTestApp(t, c, appName, "container1")
	rd := createVolumeTestRadixDeployment(t, c, appName, envNamespace, "rd-1", volumeTestVolumeMount("container1"))
	waitForActiveRadixDeployment(t, c, envNamespace, rd.Name)
	pod := waitForComponentPodWithClaim(t, c, envNamespace, "")
	pvc := &corev1.PersistentVolumeClaim{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: podClaimName(pod)}, pvc))
	pvName := pvc.Spec.VolumeName

	// Deleting the RadixRegistration deletes the RadixEnvironment, which owns the environment namespace
	require.NoError(t, c.Delete(t.Context(), &v1.RadixRegistration{Name: appName}))

	err := wait.PollUntilContextTimeout(t.Context(), volumeTestPollInterval, volumeTestTimeout, true, func(ctx context.Context) (bool, error) {
		nsErr := c.Get(ctx, client.ObjectKey{Name: envNamespace}, &corev1.Namespace{})
		pvErr := c.Get(ctx, client.ObjectKey{Name: pvName}, &corev1.PersistentVolume{})
		return apierrors.IsNotFound(nsErr) && apierrors.IsNotFound(pvErr), nil
	})
	if err != nil {
		logRadixOperatorLogs(t, c, "PersistentVolume")
	}
	require.NoError(t, err, "the namespace %s and PV %s should be deleted", envNamespace, pvName)
}

// TestLegacyVolumesAreReusedWhenMatching creates PV/PVC pairs as created by previous operator versions, before the
// component is deployed: one that matches the desired spec, and one for an older spec. It verifies that the operator
// reuses the matching pair as is, without creating a new pair or changing it, that the workload is not rolled out
// when the RadixDeployment syncs again, and that the pair for the older spec is garbage collected.
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

	pvcs := &corev1.PersistentVolumeClaimList{}
	require.NoError(t, c.List(t.Context(), pvcs, client.InNamespace(envNamespace)))
	var pvcNames []string
	for _, pvc := range pvcs.Items {
		pvcNames = append(pvcNames, pvc.Name)
	}
	// The pair for the older spec may already be garbage collected
	assert.Contains(t, pvcNames, legacyPvc.Name)
	assert.Subset(t, []string{oldSpecPvc.Name, legacyPvc.Name}, pvcNames, "no new PVC should be created")

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

	// Sync the RadixDeployment again until garbage collection has deleted the pair for the older spec
	err := wait.PollUntilContextTimeout(t.Context(), 5*time.Second, volumeTestTimeout, true, func(ctx context.Context) (bool, error) {
		patch := fmt.Appendf(nil, `{"metadata":{"annotations":{"e2e/resync":%q}}}`, time.Now().Format(time.RFC3339Nano))
		if err := c.Patch(ctx, &v1.RadixDeployment{Namespace: envNamespace, Name: rd.Name}, client.RawPatch(types.MergePatchType, patch)); err != nil {
			return false, nil
		}
		pvcErr := c.Get(ctx, client.ObjectKey{Namespace: envNamespace, Name: oldSpecPvc.Name}, &corev1.PersistentVolumeClaim{})
		pvErr := c.Get(ctx, client.ObjectKey{Name: oldSpecPv.Name}, &corev1.PersistentVolume{})
		return apierrors.IsNotFound(pvcErr) && apierrors.IsNotFound(pvErr), nil
	})
	if err != nil {
		logRadixOperatorLogs(t, c, "PersistentVolume")
	}
	require.NoError(t, err, "the legacy pair for the older spec should be deleted")

	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: volumeTestComponentName}, deployment))
	assert.Equal(t, generation, deployment.Generation, "the deployment should not be rolled out when the RadixDeployment syncs again")
	assert.Equal(t, legacyPvc.Name, podClaimName(&corev1.Pod{Spec: deployment.Spec.Template.Spec}))
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: legacyPvc.Name}, &corev1.PersistentVolumeClaim{}), "the legacy PVC in use should be kept")
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Name: legacyPv.Name}, &corev1.PersistentVolume{}), "the legacy PV in use should be kept")
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

func createVolumeTestRadixDeployment(t *testing.T, c client.Client, appName, envNamespace, rdName string, volumeMount v1.RadixVolumeMount) *v1.RadixDeployment {
	t.Helper()
	rd := &v1.RadixDeployment{
		Name: rdName, Namespace: envNamespace,
		Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixEnvLabel: volumeTestEnvName},
		Spec: v1.RadixDeploymentSpec{
			AppName:     appName, //nolint:staticcheck // required by the CRD
			Environment: volumeTestEnvName,
			Components: []v1.RadixDeployComponent{{
				Name:      volumeTestComponentName,
				Image:     volumeTestImage,
				Replicas:  new(1),
				RunAsUser: new(int64(101)),
				Runtime:   volumeTestRuntime(),
				// Print the volume info written by the fake CSI driver, so the test can read it from the Pod logs
				Command:      []string{"sh", "-c", fmt.Sprintf("cat %s/%s && echo && sleep 100000", volumeTestMountPath, internal.FakeBlobCSIVolumeInfoFileName)},
				VolumeMounts: []v1.RadixVolumeMount{volumeMount},
			}},
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

func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
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
