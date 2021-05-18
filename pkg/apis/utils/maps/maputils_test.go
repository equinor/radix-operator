package maps

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
)

func Test_GetKeysFromMap(t *testing.T) {
	a := make(map[string][]byte)
	a["a"] = []byte("x")
	a["b"] = []byte("y")
	a["c"] = []byte("z")

	assert.True(t, utils.ArrayEqualElements([]string{"a", "b", "c"}, GetKeysFromByteMap(a)))
}

func TestMergeStringMaps(t *testing.T) {
	empty := make(map[string]string)
	expect := map[string]string{
		"a": "a", "x": "x", "b": "y",
	}

	map1 := map[string]string{"a": "a", "b": "c"}
	map2 := map[string]string{"x": "x", "b": "y"}

	result := MergeStringMaps(map1, map2)
	assert.Equal(t, expect, result)
	result = MergeStringMaps(map2, map1)
	assert.NotEqual(t, expect, result)

	result = MergeStringMaps(nil, map1)
	assert.Equal(t, map1, result)
	result = MergeStringMaps(map2, nil)
	assert.Equal(t, map2, result)

	result = MergeStringMaps(nil, nil)
	assert.Equal(t, empty, result)
}
