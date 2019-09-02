package slice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Contains(t *testing.T) {
	a := []string{"a", "b", "c"}

	assert.True(t, ContainsString(a, "a"))
	assert.True(t, ContainsString(a, "b"))
	assert.True(t, ContainsString(a, "c"))
	assert.False(t, ContainsString(a, "d"))
}
