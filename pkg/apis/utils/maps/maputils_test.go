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
