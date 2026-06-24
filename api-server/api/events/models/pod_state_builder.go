package models

import corev1 "k8s.io/api/core/v1"

// PodStateBuilder Build PodState DTOs
type PodStateBuilder interface {
	WithPod(*corev1.Pod) PodStateBuilder
	Build() *PodState
}

type podStateBuilder struct {
	pod *corev1.Pod
}

// NewPodStateBuilder Constructor for podStateBuilder
func NewPodStateBuilder() PodStateBuilder {
	return &podStateBuilder{}
}

func (b *podStateBuilder) WithPod(v *corev1.Pod) PodStateBuilder {
	b.pod = v
	return b
}

func (b *podStateBuilder) Build() *PodState {
	podState := PodState{}

	if b.pod != nil && len(b.pod.Status.ContainerStatuses) > 0 {
		cs := b.pod.Status.ContainerStatuses[0]
		podState.Ready = cs.Ready
		podState.Started = cs.Started
		podState.RestartCount = cs.RestartCount
	}

	return &podState
}
