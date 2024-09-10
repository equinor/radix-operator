package build

import (
	"github.com/equinor/radix-common/utils/pointers"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type kubeJobBuilder struct {
	spec struct {
		name               string
		labels             map[string]string
		annotations        map[string]string
		podLabels          map[string]string
		podAnnotations     map[string]string
		podAffinity        *corev1.Affinity
		podTolerations     []corev1.Toleration
		podSecurityContext *corev1.PodSecurityContext
		podVolumes         []corev1.Volume
		podInitContainers  []corev1.Container
		podContainers      []corev1.Container
	}
}

func (b *kubeJobBuilder) SetName(name string) {
	b.spec.name = name
}

func (b *kubeJobBuilder) SetLabels(labels map[string]string) {
	b.spec.labels = labels
}

func (b *kubeJobBuilder) SetAnnotations(annotations map[string]string) {
	b.spec.annotations = annotations
}

func (b *kubeJobBuilder) SetPodLabels(labels map[string]string) {
	b.spec.podLabels = labels
}

func (b *kubeJobBuilder) SetPodAnnotations(annotations map[string]string) {
	b.spec.podAnnotations = annotations
}

func (b *kubeJobBuilder) SetPodTolerations(tolerations []corev1.Toleration) {
	b.spec.podTolerations = tolerations
}

func (b *kubeJobBuilder) SetPodAffinity(affinity *corev1.Affinity) {
	b.spec.podAffinity = affinity
}

func (b *kubeJobBuilder) SetPodSecurityContext(securityContext *corev1.PodSecurityContext) {
	b.spec.podSecurityContext = securityContext
}

func (b *kubeJobBuilder) SetPodVolumes(volumes []corev1.Volume) {
	b.spec.podVolumes = volumes
}

func (b *kubeJobBuilder) SetPodInitContainers(containers []corev1.Container) {
	b.spec.podInitContainers = containers
}

func (b *kubeJobBuilder) SetPodContainers(containers []corev1.Container) {
	b.spec.podContainers = containers
}

func (b *kubeJobBuilder) GetJob() batchv1.Job {
	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        b.spec.name,
			Labels:      b.spec.labels,
			Annotations: b.spec.annotations,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: pointers.Ptr[int32](0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      b.spec.podLabels,
					Annotations: b.spec.podAnnotations,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:   corev1.RestartPolicyNever,
					Affinity:        b.spec.podAffinity,
					Tolerations:     b.spec.podTolerations,
					SecurityContext: b.spec.podSecurityContext,
					Volumes:         b.spec.podVolumes,
					InitContainers:  b.spec.podInitContainers,
					Containers:      b.spec.podContainers,
				},
			},
		},
	}
}
