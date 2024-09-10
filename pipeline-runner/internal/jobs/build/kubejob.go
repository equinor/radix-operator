package build

import (
	"github.com/equinor/radix-common/utils/pointers"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type kubeJobBuilderSource interface {
	JobName() string
	JobLabels() map[string]string
	JobAnnotations() map[string]string
	PodLabels() map[string]string
	PodAnnotations() map[string]string
	PodTolerations() []corev1.Toleration
	PodAffinity() *corev1.Affinity
	PodSecurityContext() *corev1.PodSecurityContext
	PodVolumes() []corev1.Volume
	PodInitContainers() []corev1.Container
	PodContainers() []corev1.Container
}

type kubeJobBuilder struct {
	source kubeJobBuilderSource
}

func (b *kubeJobBuilder) GetJob() batchv1.Job {
	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        b.source.JobName(),
			Labels:      b.source.JobLabels(),
			Annotations: b.source.JobAnnotations(),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: pointers.Ptr[int32](0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      b.source.PodLabels(),
					Annotations: b.source.PodAnnotations(),
				},
				Spec: corev1.PodSpec{
					RestartPolicy:   corev1.RestartPolicyNever,
					Affinity:        b.source.PodAffinity(),
					Tolerations:     b.source.PodTolerations(),
					SecurityContext: b.source.PodSecurityContext(),
					Volumes:         b.source.PodVolumes(),
					InitContainers:  b.source.PodInitContainers(),
					Containers:      b.source.PodContainers(),
				},
			},
		},
	}
}
