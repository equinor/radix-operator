package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetComponentHostname(t *testing.T) {
	assert.Equal(t, "component-namespace", GetComponentHostname("component", "namespace"))
}
