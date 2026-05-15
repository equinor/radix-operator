package sort

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
)

type Direction int

const (
	Ascending Direction = iota
	Descending
)

// PodLessFunc compares pod and compareWith, and must return true if pod is less than compareWith.
type PodLessFunc func(pod corev1.Pod, compareWith corev1.Pod) bool

// ByPodCreationTimestamp compares CreationTimestamp of pod and compareWith.
// Returns true if CreationTimestamp for pod is less than compareWith.
func ByPodCreationTimestamp(pod corev1.Pod, compareWith corev1.Pod) bool {
	return pod.CreationTimestamp.Before(&compareWith.CreationTimestamp)
}

// Pods sorts the slice of pods with the provided lessFunc.
func Pods(pods []corev1.Pod, lessFunc PodLessFunc, direction Direction) {
	sort.Slice(pods, func(i, j int) bool {
		isLess := lessFunc(pods[i], pods[j])
		if direction == Descending {
			isLess = !isLess
		}
		return isLess
	})
}
