package kubemerge_test

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils/kubemerge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func Test_MergePodTemplate_HappyPath(t *testing.T) {
	base := corev1.PodTemplateSpec{
		Labels:      map[string]string{"app": "base", "keep": "yes"},
		Annotations: map[string]string{"base-annotation": "1"},
		Spec: corev1.PodSpec{
			ServiceAccountName: "base-sa",
			NodeSelector:       map[string]string{"pool": "base"},
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: "base-secret"},
				{Name: "shared-secret"},
			},
			Volumes: []corev1.Volume{
				{Name: "tmp", EmptyDir: &corev1.EmptyDirVolumeSource{}},
			},
			Containers: []corev1.Container{
				{
					Name:  "main",
					Image: "main:1",
					Env: []corev1.EnvVar{
						{Name: "KEEP", Value: "base"},
						{Name: "OVERRIDE", Value: "base"},
					},
					Ports: []corev1.ContainerPort{
						{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
						{Name: "metrics", ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
					},
					VolumeMounts: []corev1.VolumeMount{{Name: "tmp", MountPath: "/tmp"}},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
					},
				},
				{
					Name:  "sidecar",
					Image: "sidecar:1",
				},
			},
		},
	}
	overlay := corev1.PodTemplateSpec{
		Labels: map[string]string{"app": "overlay", "extra": "yes"},
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{"zone": "a"},
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: "shared-secret"},
				{Name: "overlay-secret"},
			},
			Volumes: []corev1.Volume{
				{Name: "config", ConfigMap: &corev1.ConfigMapVolumeSource{Name: "cm"}},
			},
			Containers: []corev1.Container{
				{
					Name:  "main",
					Image: "main:2",
					Env: []corev1.EnvVar{
						{Name: "OVERRIDE", Value: "overlay"},
						{Name: "NEW", Value: "overlay"},
					},
					Ports: []corev1.ContainerPort{
						{Name: "http-renamed", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
						{Name: "admin", ContainerPort: 7070, Protocol: corev1.ProtocolTCP},
					},
					VolumeMounts: []corev1.VolumeMount{{Name: "config", MountPath: "/config"}},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("128Mi")},
					},
				},
				{
					Name:  "added",
					Image: "added:1",
				},
			},
		},
	}

	overlay2 := corev1.PodTemplateSpec{
		Labels:      map[string]string{"app": "overlay2"},
		Annotations: map[string]string{"overlay2-annotation": "1"},
		Spec: corev1.PodSpec{
			NodeSelector:     map[string]string{"zone": "b"},
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "overlay2-secret"}},
			Containers: []corev1.Container{
				{
					Name:  "main",
					Image: "main:3",
					Env:   []corev1.EnvVar{{Name: "OVERRIDE", Value: "overlay2"}},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("256Mi")},
					},
				},
				{
					Name:  "sidecar",
					Image: "sidecar:2",
				},
			},
		},
	}

	got, err := kubemerge.MergePodTemplate(base, overlay, overlay2)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"app": "overlay2", "keep": "yes", "extra": "yes"}, got.Labels)
	assert.Equal(t, map[string]string{"base-annotation": "1", "overlay2-annotation": "1"}, got.Annotations)
	assert.Equal(t, "base-sa", got.Spec.ServiceAccountName, "fields not set in overlays must be kept")
	assert.Equal(t, map[string]string{"pool": "base", "zone": "b"}, got.Spec.NodeSelector)

	assert.ElementsMatch(t, []corev1.LocalObjectReference{
		{Name: "base-secret"}, {Name: "shared-secret"}, {Name: "overlay-secret"}, {Name: "overlay2-secret"},
	}, got.Spec.ImagePullSecrets, "imagePullSecrets are merged by name without duplicates")

	require.Len(t, got.Spec.Volumes, 2)
	volumes := map[string]corev1.Volume{}
	for _, v := range got.Spec.Volumes {
		volumes[v.Name] = v
	}
	assert.NotNil(t, volumes["tmp"].EmptyDir, "emptyDir: {} must survive the merge")
	assert.NotNil(t, volumes["config"].ConfigMap)

	require.Len(t, got.Spec.Containers, 3, "containers are merged by name")
	containers := map[string]corev1.Container{}
	for _, c := range got.Spec.Containers {
		containers[c.Name] = c
	}
	assert.Equal(t, "sidecar:2", containers["sidecar"].Image, "container skipped by overlay is still overridden by overlay2")
	assert.Equal(t, "added:1", containers["added"].Image, "container added by overlay is kept when not in overlay2")

	main := containers["main"]
	assert.Equal(t, "main:3", main.Image, "last overlay wins")
	assert.ElementsMatch(t, []corev1.EnvVar{
		{Name: "KEEP", Value: "base"},
		{Name: "OVERRIDE", Value: "overlay2"},
		{Name: "NEW", Value: "overlay"},
	}, main.Env, "env is merged by name and overlay wins")
	assert.ElementsMatch(t, []corev1.ContainerPort{
		{Name: "http-renamed", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
		{Name: "admin", ContainerPort: 7070, Protocol: corev1.ProtocolTCP},
	}, main.Ports, "ports are merged by containerPort")
	assert.ElementsMatch(t, []corev1.VolumeMount{
		{Name: "tmp", MountPath: "/tmp"},
		{Name: "config", MountPath: "/config"},
	}, main.VolumeMounts)
	assert.True(t, main.Resources.Requests.Cpu().Equal(resource.MustParse("100m")))
	assert.True(t, main.Resources.Limits.Memory().Equal(resource.MustParse("256Mi")))
}

func Test_MergePodTemplate_EnvValueFromIsMergedWithValue(t *testing.T) {
	base := podWithContainer(corev1.Container{Name: "main", Env: []corev1.EnvVar{
		{Name: "SECRET", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Name: "s", Key: "k"}}},
	}})
	overlay := podWithContainer(corev1.Container{Name: "main", Env: []corev1.EnvVar{{Name: "SECRET", Value: "plain"}}})

	got, err := kubemerge.MergePodTemplate(base, overlay)
	require.NoError(t, err)

	require.Len(t, got.Spec.Containers[0].Env, 1)
	env := got.Spec.Containers[0].Env[0]
	assert.Equal(t, "plain", env.Value)
	assert.NotNil(t, env.ValueFrom, "overlay cannot remove valueFrom by setting value; both end up set")
}

func Test_MergePodTemplate_AtomicListsAreReplaced(t *testing.T) {
	base := podWithContainer(corev1.Container{
		Name:    "main",
		Command: []string{"base-cmd"},
		Args:    []string{"--a", "--b"},
	})
	base.Spec.Tolerations = []corev1.Toleration{{Key: "base", Operator: corev1.TolerationOpExists}}

	overlay := podWithContainer(corev1.Container{
		Name:    "main",
		Command: []string{"overlay-cmd"},
		Args:    []string{"--c"},
	})
	overlay.Spec.Tolerations = []corev1.Toleration{{Key: "overlay", Operator: corev1.TolerationOpExists}}

	got, err := kubemerge.MergePodTemplate(base, overlay)
	require.NoError(t, err)

	assert.Equal(t, []string{"overlay-cmd"}, got.Spec.Containers[0].Command, "command is not merged")
	assert.Equal(t, []string{"--c"}, got.Spec.Containers[0].Args, "args are not merged")
	assert.Equal(t, []corev1.Toleration{{Key: "overlay", Operator: corev1.TolerationOpExists}}, got.Spec.Tolerations, "tolerations are not merged")
}

func Test_MergePodTemplate_EmptyOverlayValuesDoNotClearBase(t *testing.T) {
	base := podWithContainer(corev1.Container{
		Name:            "main",
		Image:           "main:1",
		ImagePullPolicy: corev1.PullAlways,
		Env:             []corev1.EnvVar{{Name: "KEEP", Value: "base"}},
		Ports:           []corev1.ContainerPort{{ContainerPort: 8080, Protocol: corev1.ProtocolTCP}},
	})
	base.Spec.ServiceAccountName = "base-sa"
	base.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "base-secret"}}
	base.Labels = map[string]string{"app": "base"}

	overlay := podWithContainer(corev1.Container{Name: "main"})

	got, err := kubemerge.MergePodTemplate(base, overlay)
	require.NoError(t, err)

	c := got.Spec.Containers[0]
	assert.Equal(t, "main:1", c.Image)
	assert.Equal(t, corev1.PullAlways, c.ImagePullPolicy)
	assert.Equal(t, []corev1.EnvVar{{Name: "KEEP", Value: "base"}}, c.Env)
	assert.Equal(t, []corev1.ContainerPort{{ContainerPort: 8080, Protocol: corev1.ProtocolTCP}}, c.Ports)
	assert.Equal(t, "base-sa", got.Spec.ServiceAccountName)
	assert.Equal(t, []corev1.LocalObjectReference{{Name: "base-secret"}}, got.Spec.ImagePullSecrets)
	assert.Equal(t, map[string]string{"app": "base"}, got.Labels)
}

func Test_MergePodTemplate_OverlayWithoutContainersKeepsBaseContainers(t *testing.T) {
	base := podWithContainer(corev1.Container{Name: "main", Image: "main:1"})
	overlay := corev1.PodTemplateSpec{Spec: corev1.PodSpec{ServiceAccountName: "sa"}}

	got, err := kubemerge.MergePodTemplate(base, overlay)
	require.NoError(t, err)

	assert.Equal(t, []corev1.Container{{Name: "main", Image: "main:1"}}, got.Spec.Containers)
	assert.Equal(t, "sa", got.Spec.ServiceAccountName)
}

func Test_MergePodTemplate_NestedNilFieldsKeepBaseValues(t *testing.T) {
	sources := []corev1.VolumeProjection{{ConfigMap: &corev1.ConfigMapProjection{Name: "cm"}}}
	base := podWithContainer(corev1.Container{Name: "main"})
	base.Spec.Volumes = []corev1.Volume{{Name: "proj", Projected: &corev1.ProjectedVolumeSource{Sources: sources}}}

	overlay := podWithContainer(corev1.Container{Name: "main"})
	overlay.Spec.Volumes = []corev1.Volume{{Name: "proj", Projected: &corev1.ProjectedVolumeSource{DefaultMode: new(int32(0o444))}}}

	got, err := kubemerge.MergePodTemplate(base, overlay)
	require.NoError(t, err)

	require.Len(t, got.Spec.Volumes, 1)
	assert.Equal(t, sources, got.Spec.Volumes[0].Projected.Sources)
	assert.Equal(t, new(int32(0o444)), got.Spec.Volumes[0].Projected.DefaultMode)
}

func Test_MergePodTemplate_VolumeSourcesAreMerged(t *testing.T) {
	base := podWithContainer(corev1.Container{Name: "main"})
	base.Spec.Volumes = []corev1.Volume{{Name: "data", EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}}}

	overlay := podWithContainer(corev1.Container{Name: "main"})
	overlay.Spec.Volumes = []corev1.Volume{{Name: "data", CSI: &corev1.CSIVolumeSource{Driver: "csi.example.com"}}}

	got, err := kubemerge.MergePodTemplate(base, overlay)
	require.NoError(t, err)

	require.Len(t, got.Spec.Volumes, 1)
	assert.Equal(t, &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}, got.Spec.Volumes[0].EmptyDir)
	assert.Equal(t, &corev1.CSIVolumeSource{Driver: "csi.example.com"}, got.Spec.Volumes[0].CSI)
}

func Test_MergePodTemplate_ProbeHandlerIsNotReplaced(t *testing.T) {
	base := podWithContainer(corev1.Container{Name: "main", ReadinessProbe: &corev1.Probe{
		TCPSocket:           &corev1.TCPSocketAction{Port: intstr.FromInt32(6379)},
		InitialDelaySeconds: 10,
		PeriodSeconds:       10,
	}})
	overlay := podWithContainer(corev1.Container{Name: "main", ReadinessProbe: &corev1.Probe{
		TCPSocket: nil, HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromInt32(8080)},
		PeriodSeconds: 5,
	}})

	got, err := kubemerge.MergePodTemplate(base, overlay)
	require.NoError(t, err)

	probe := got.Spec.Containers[0].ReadinessProbe
	require.NotNil(t, probe)
	assert.Equal(t, &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromInt32(8080)}, probe.HTTPGet)
	assert.Equal(t, &corev1.TCPSocketAction{Port: intstr.FromInt32(6379)}, probe.TCPSocket, "nil is omitted from the patch, so base handler is kept and the probe ends up invalid")
	assert.Equal(t, int32(10), probe.InitialDelaySeconds)
	assert.Equal(t, int32(5), probe.PeriodSeconds)
}

func podWithContainer(c corev1.Container) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{c}}}
}
