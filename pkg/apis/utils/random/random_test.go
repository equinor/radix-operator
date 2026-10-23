package random_test

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils/random"
	"github.com/stretchr/testify/assert"
)

func Test_input_does_not_equal_output(t *testing.T) {
	s := "input"
	hash := random.RandStringStrSeed(5, s)

	assert.NotEqual(t, s, hash)
}

func Test_equals_hash(t *testing.T) {
	s := "some string"
	hash1 := random.RandStringStrSeed(5, s)
	hash2 := random.RandStringStrSeed(5, s)

	assert.Equal(t, hash1, hash2)
}

func Test_hash_len_equals_5(t *testing.T) {
	s := "some string"
	hash1 := random.RandStringStrSeed(5, s)

	assert.Equal(t, 5, len(hash1))
}

func Test_non_equal_hash(t *testing.T) {
	hash1 := random.RandStringStrSeed(5, "some string")
	hash2 := random.RandStringStrSeed(5, "some other string")

	assert.NotEqual(t, hash1, hash2)
}

func Test_random_string(t *testing.T) {
	rand1 := random.RandString(10)
	rand2 := random.RandString(10)

	assert.NotEqual(t, rand1, rand2)
}

func Test_GenerateRandomKey(t *testing.T) {
	key1 := random.GenerateRandomKey(20)
	key2 := random.GenerateRandomKey(20)
	key3 := random.GenerateRandomKey(15)
	assert.NotEqual(t, key1, key2)
	assert.Len(t, key1, 20)
	assert.Len(t, key2, 20)
	assert.Len(t, key3, 15)
}
