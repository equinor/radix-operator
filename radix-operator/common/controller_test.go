package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Dummy(t *testing.T) {
	assert.True(t, true, "for now - hard to test as the RateLimitingInterface doesn't have a fake and queue.Get(..) times out")
}
