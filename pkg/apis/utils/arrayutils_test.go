package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetKeysFromMap(t *testing.T) {
	a := make(map[string][]byte)
	a["a"] = []byte("x")
	a["b"] = []byte("y")
	a["c"] = []byte("z")

	assert.Equal(t, []string{"a", "b", "c"}, GetKeysFromByteMap(a))
}

func Test_Contains(t *testing.T) {
	a := []string{"a", "b", "c"}

	assert.True(t, ContainsString(a, "a"))
	assert.True(t, ContainsString(a, "b"))
	assert.True(t, ContainsString(a, "c"))
	assert.False(t, ContainsString(a, "d"))
}

func Test_array_equals(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := []string{"a", "b", "c"}

	assert.True(t, ArrayEqual(a, b))
	assert.True(t, ArrayEqualElements(a, b))
}

func Test_array_same_element_diff_order(t *testing.T) {
	a := []string{"a", "c", "b"}
	b := []string{"a", "b", "c"}

	assert.False(t, ArrayEqual(a, b))
	assert.True(t, ArrayEqualElements(a, b))
}

func Test_array_same_len_diff_element(t *testing.T) {
	a := []string{"a", "c", "z"}
	b := []string{"a", "b", "z"}

	assert.False(t, ArrayEqual(a, b))
	assert.False(t, ArrayEqualElements(a, b))
}

func Test_array_diff_len_and_elem(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := []string{"a", "b", "c", "d"}

	assert.False(t, ArrayEqual(a, b))
	assert.False(t, ArrayEqualElements(a, b))
}
