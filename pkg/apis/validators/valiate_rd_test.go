package validators

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_invalid_replicas(t *testing.T) {
	err := validateReplica(-1)

	assert.NotNil(t, err)
}

func Test_zero_replicas_means_default(t *testing.T) {
	err := validateReplica(0)

	assert.Nil(t, err)
}
