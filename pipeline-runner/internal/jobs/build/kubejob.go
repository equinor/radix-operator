package build

// type kubeJobProps interface {
// 	JobName() string
// 	JobLabels() map[string]string
// 	JobAnnotations() map[string]string
// 	PodLabels() map[string]string
// 	PodAnnotations() map[string]string
// 	PodTolerations() []corev1.Toleration
// 	PodAffinity() *corev1.Affinity
// 	PodSecurityContext() *corev1.PodSecurityContext
// 	PodVolumes() []corev1.Volume
// 	PodInitContainers() []corev1.Container
// 	PodContainers() []corev1.Container
// }

// func buildKubeJob(props kubeJobProps) batchv1.Job {
// 	return batchv1.Job{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:        props.JobName(),
// 			Labels:      props.JobLabels(),
// 			Annotations: props.JobAnnotations(),
// 		},
// 		Spec: batchv1.JobSpec{
// 			BackoffLimit: pointers.Ptr[int32](0),
// 			Template: corev1.PodTemplateSpec{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Labels:      props.PodLabels(),
// 					Annotations: props.PodAnnotations(),
// 				},
// 				Spec: corev1.PodSpec{
// 					RestartPolicy:   corev1.RestartPolicyNever,
// 					Affinity:        props.PodAffinity(),
// 					Tolerations:     props.PodTolerations(),
// 					SecurityContext: props.PodSecurityContext(),
// 					Volumes:         props.PodVolumes(),
// 					InitContainers:  props.PodInitContainers(),
// 					Containers:      props.PodContainers(),
// 				},
// 			},
// 		},
// 	}
// }