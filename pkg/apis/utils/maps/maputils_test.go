package maps

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
