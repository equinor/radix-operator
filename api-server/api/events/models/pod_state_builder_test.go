package models

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"
)

func Test_PodStateBuilder(t *testing.T) {
	t.Run("WithPod_TwoComponents", func(t *testing.T) {
		started := false
		v := v1.Pod{
			Status: v1.PodStatus{
				ContainerStatuses: []v1.ContainerStatus{
					{Started: &started, Ready: true, RestartCount: 1},
					{Started: nil, Ready: true, RestartCount: 2},
				},
			},
		}
		e := NewPodStateBuilder().
			WithPod(&v).
			Build()
		assert.Equal(t, &started, e.Started)
		assert.Equal(t, true, e.Ready)
		assert.Equal(t, int32(1), e.RestartCount)
	})

	t.Run("WithPod_NoComponents", func(t *testing.T) {
		v := v1.Pod{}
		e := NewPodStateBuilder().
			WithPod(&v).
			Build()
		assert.Equal(t, (*bool)(nil), e.Started)
		assert.Equal(t, false, e.Ready)
		assert.Equal(t, int32(0), e.RestartCount)
	})
}
