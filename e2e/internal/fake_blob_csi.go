package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// FakeBlobCSIImageName is the local image name for the fake Azure Blob CSI node plugin.
	FakeBlobCSIImageName = "local-kind-repo/fake-blob-csi"
	// FakeBlobCSIVolumeInfoFileName is the file the fake driver writes to the root of each volume,
	// with the volume handle, volume attributes and mount options it received from kubelet.
	FakeBlobCSIVolumeInfoFileName = "fake-blob-csi.json"

	fakeBlobCSIDriverName     = "blob.csi.azure.com"
	fakeBlobCSINamespace      = "fake-blob-csi"
	fakeBlobCSIName           = "fake-blob-csi-node"
	fakeBlobCSIRegistrarImage = "registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.13.0"
	kubeletDir                = "/var/lib/kubelet"
	fakeBlobCSIPluginDir      = kubeletDir + "/plugins/" + fakeBlobCSIDriverName
	fakeBlobCSIDataDir        = "/var/lib/fake-blob-csi"
)

// FakeBlobCSIVolumeInfo is the content of FakeBlobCSIVolumeInfoFileName
type FakeBlobCSIVolumeInfo struct {
	VolumeID         string            `json:"volumeId"`
	VolumeContext    map[string]string `json:"volumeContext"`
	MountFlags       []string          `json:"mountFlags"`
	AccessMode       string            `json:"accessMode"`
	Readonly         bool              `json:"readonly"`
	NodePublishCount int               `json:"nodePublishCount"`
}

// BuildFakeBlobCSIImage builds the fake Azure Blob CSI node plugin image from e2e/testdata/fakeblobcsi.
func BuildFakeBlobCSIImage(ctx context.Context, imageTag string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	fullImageName := fmt.Sprintf("%s:%s", FakeBlobCSIImageName, imageTag)
	log.Info().Msgf("Building fake blob CSI image %s...", fullImageName)

	cmd := exec.CommandContext(ctx, "docker", "build", "-t", fullImageName, "-f", "testdata/fakeblobcsi/fake-blob-csi.Dockerfile", "testdata/fakeblobcsi")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build fake blob CSI image %s: %w", fullImageName, err)
	}

	log.Info().Msgf("Successfully built %s", fullImageName)
	return nil
}

// DeployFakeBlobCSIDriver deploys a fake CSI node plugin named blob.csi.azure.com to every node, so Pods can mount
// the statically provisioned PersistentVolumes created by the operator for BlobFuse2 volume mounts. It is idempotent.
func DeployFakeBlobCSIDriver(ctx context.Context, c client.Client, imageTag string) error {
	csiDriver := &storagev1.CSIDriver{Name: fakeBlobCSIDriverName}
	if _, err := controllerutil.CreateOrUpdate(ctx, c, csiDriver, func() error {
		csiDriver.Spec.AttachRequired = new(false)
		csiDriver.Spec.PodInfoOnMount = new(false)
		csiDriver.Spec.VolumeLifecycleModes = []storagev1.VolumeLifecycleMode{storagev1.VolumeLifecyclePersistent}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to apply CSIDriver %s: %w", fakeBlobCSIDriverName, err)
	}

	ns := &corev1.Namespace{Name: fakeBlobCSINamespace}
	if _, err := controllerutil.CreateOrUpdate(ctx, c, ns, func() error { return nil }); err != nil {
		return fmt.Errorf("failed to apply namespace %s: %w", fakeBlobCSINamespace, err)
	}

	image := fmt.Sprintf("%s:%s", FakeBlobCSIImageName, imageTag)
	labels := map[string]string{"app": fakeBlobCSIName}
	hostPathDirectoryOrCreate := corev1.HostPathDirectoryOrCreate
	hostPathDirectory := corev1.HostPathDirectory
	bidirectional := corev1.MountPropagationBidirectional
	daemonSet := &appsv1.DaemonSet{Name: fakeBlobCSIName, Namespace: fakeBlobCSINamespace}
	if _, err := controllerutil.CreateOrUpdate(ctx, c, daemonSet, func() error {
		daemonSet.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		daemonSet.Spec.Template.ObjectMeta.Labels = labels
		daemonSet.Spec.Template.Spec = corev1.PodSpec{
			Tolerations: []corev1.Toleration{{Operator: corev1.TolerationOpExists}},
			Containers: []corev1.Container{
				{
					Name:            "driver",
					Image:           image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args:            []string{"--endpoint=unix:///csi/csi.sock", "--nodeid=$(NODE_NAME)", "--drivername=" + fakeBlobCSIDriverName, "--datadir=" + fakeBlobCSIDataDir},
					Env:             []corev1.EnvVar{{Name: "NODE_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}}},
					SecurityContext: &corev1.SecurityContext{Privileged: new(true)},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "plugin-dir", MountPath: "/csi"},
						{Name: "kubelet-dir", MountPath: kubeletDir, MountPropagation: &bidirectional},
						{Name: "data-dir", MountPath: fakeBlobCSIDataDir},
					},
				},
				{
					Name:            "node-driver-registrar",
					Image:           fakeBlobCSIRegistrarImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args:            []string{"--csi-address=/csi/csi.sock", "--kubelet-registration-path=" + fakeBlobCSIPluginDir + "/csi.sock"},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "plugin-dir", MountPath: "/csi"},
						{Name: "registration-dir", MountPath: "/registration"},
					},
				},
			},
			Volumes: []corev1.Volume{
				{Name: "plugin-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: fakeBlobCSIPluginDir, Type: &hostPathDirectoryOrCreate}}},
				{Name: "registration-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: kubeletDir + "/plugins_registry", Type: &hostPathDirectory}}},
				{Name: "kubelet-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: kubeletDir, Type: &hostPathDirectory}}},
				{Name: "data-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: fakeBlobCSIDataDir, Type: &hostPathDirectoryOrCreate}}},
			},
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to apply fake blob CSI DaemonSet: %w", err)
	}

	if err := waitForDaemonSetReady(ctx, c, fakeBlobCSINamespace, fakeBlobCSIName, 3*time.Minute); err != nil {
		return fmt.Errorf("fake blob CSI DaemonSet not ready: %w", err)
	}
	return waitForCSINodeDriver(ctx, c, fakeBlobCSIDriverName, 2*time.Minute)
}

func waitForDaemonSetReady(ctx context.Context, c client.Client, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		daemonSet := &appsv1.DaemonSet{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, daemonSet); err != nil {
			return false, nil
		}
		s := daemonSet.Status
		return s.ObservedGeneration >= daemonSet.Generation &&
			s.DesiredNumberScheduled > 0 &&
			s.UpdatedNumberScheduled == s.DesiredNumberScheduled &&
			s.NumberReady == s.DesiredNumberScheduled, nil
	})
}

// waitForCSINodeDriver waits until kubelet on every node has registered the CSI driver
func waitForCSINodeDriver(ctx context.Context, c client.Client, driverName string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		nodes := &corev1.NodeList{}
		if err := c.List(ctx, nodes); err != nil || len(nodes.Items) == 0 {
			return false, nil
		}
		for _, node := range nodes.Items {
			csiNode := &storagev1.CSINode{}
			if err := c.Get(ctx, client.ObjectKey{Name: node.Name}, csiNode); err != nil {
				return false, nil
			}
			registered := false
			for _, d := range csiNode.Spec.Drivers {
				if d.Name == driverName {
					registered = true
					break
				}
			}
			if !registered {
				return false, nil
			}
		}
		return true, nil
	})
}
