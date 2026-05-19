package sort

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_ByPodCreationTimestamp(t *testing.T) {
	newTime := time.Now()
	oldTime := newTime.Add(-1 * time.Second)

	oldPod := corev1.Pod{ObjectMeta: v1.ObjectMeta{CreationTimestamp: v1.NewTime(oldTime)}}
	newPod := corev1.Pod{ObjectMeta: v1.ObjectMeta{CreationTimestamp: v1.NewTime(newTime)}}

	assert.True(t, ByPodCreationTimestamp(oldPod, newPod))
	assert.False(t, ByPodCreationTimestamp(newPod, oldPod))
}

func Test_Pod(t *testing.T) {
	less := func(pod corev1.Pod, compareWith corev1.Pod) bool {
		return pod.Name < compareWith.Name
	}
	pod_a, pod_b, pod_c := corev1.Pod{ObjectMeta: v1.ObjectMeta{Name: "a"}}, corev1.Pod{ObjectMeta: v1.ObjectMeta{Name: "b"}}, corev1.Pod{ObjectMeta: v1.ObjectMeta{Name: "c"}}

	type list struct {
		pods []corev1.Pod
	}

	// Sort ascending
	podList := list{pods: []corev1.Pod{pod_b, pod_c, pod_a}}
	Pods(podList.pods, less, Ascending)
	assert.Equal(t, []corev1.Pod{pod_a, pod_b, pod_c}, podList.pods)

	// Sort descending
	podList = list{pods: []corev1.Pod{pod_b, pod_c, pod_a}}
	Pods(podList.pods, less, Descending)
	assert.Equal(t, []corev1.Pod{pod_c, pod_b, pod_a}, podList.pods)
}
