package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_input_does_not_equal_output(t *testing.T) {
	s := "input"
	hash := RandStringStrSeed(5, s)

	assert.NotEqual(t, s, hash)
}

func Test_equals_hash(t *testing.T) {
	s := "some string"
	hash1 := RandStringStrSeed(5, s)
	hash2 := RandStringStrSeed(5, s)

	assert.Equal(t, hash1, hash2)
}

func Test_hash_len_equals_5(t *testing.T) {
	s := "some string"
	hash1 := RandStringStrSeed(5, s)

	assert.Equal(t, 5, len(hash1))
}

func Test_non_equal_hash(t *testing.T) {
	hash1 := RandStringStrSeed(5, "some string")
	hash2 := RandStringStrSeed(5, "some other string")

	assert.NotEqual(t, hash1, hash2)
}

func Test_random_string(t *testing.T) {
	rand1 := RandString(10)
	rand2 := RandString(10)

	assert.NotEqual(t, rand1, rand2)
}
