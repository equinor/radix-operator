package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ObjectStateBuilder(t *testing.T) {
	t.Run("WithPodState", func(t *testing.T) {
		v := PodState{}
		e := NewObjectStateBuilder().
			WithPodState(&v).
			Build()
		assert.Equal(t, &v, e.Pod)
	})
}
