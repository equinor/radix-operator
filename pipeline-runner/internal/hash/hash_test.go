package hash_test

import (
	"fmt"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/internal/hash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_It(t *testing.T) {
	s, err := hash.ToHashString(hash.SHA256, 11)
	require.NoError(t, err)
	assert.NotEmpty(t, s)
	fmt.Println(s)
}
