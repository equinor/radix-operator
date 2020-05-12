package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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

func Test_array_same_len_diff_repeated_element(t *testing.T) {
	a := []string{"a", "a", "a"}
	b := []string{"a", "b", "c"}

	assert.False(t, ArrayEqual(a, b))
	assert.False(t, ArrayEqualElements(a, b))
}
