package branch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidPattern(t *testing.T) {
	assert.False(t, IsValidPattern(""))
	assert.False(t, IsValidPattern("/test"))
	assert.False(t, IsValidPattern("\\"))

	assert.True(t, IsValidPattern("😀"))
	assert.True(t, IsValidPattern("t+est"))
	assert.True(t, IsValidPattern("test"))
	assert.True(t, IsValidPattern("test/tull"))
	assert.True(t, IsValidPattern("hotfix/**/*"))
	assert.True(t, IsValidPattern("release/*"))
	assert.True(t, IsValidPattern("feature/*"))
	assert.True(t, IsValidPattern("tes?"))
	assert.True(t, IsValidPattern("te??"))
	assert.True(t, IsValidPattern("??st"))
	assert.True(t, IsValidPattern("?est/*"))
	assert.True(t, IsValidPattern("te?t/*"))
}

func TestMatchesPattern(t *testing.T) {
	assert.False(t, MatchesPattern("Test", "test"))
	assert.False(t, MatchesPattern("release", "release/0.1.3"))
	assert.False(t, MatchesPattern("release", "release/q3/0.1.3"))
	assert.False(t, MatchesPattern("release/*", "release/q3/0.1.3"))
	assert.False(t, MatchesPattern("release/*", "release"))
	assert.False(t, MatchesPattern("release/**/*", "release"))
	assert.False(t, MatchesPattern("test/*/tull", "test/test1/test2/tull"))

	assert.True(t, MatchesPattern("test", "test"))
	assert.True(t, MatchesPattern("te??", "test"))
	assert.True(t, MatchesPattern("??st", "test"))
	assert.True(t, MatchesPattern("*", "test"))
	assert.True(t, MatchesPattern("test*", "testTest"))
	assert.True(t, MatchesPattern("test/*", "test/Test"))
	assert.True(t, MatchesPattern("test/*", "test/t"))
	assert.True(t, MatchesPattern("*test*", "AnothertestTest"))
	assert.True(t, MatchesPattern("*test", "Anothertest"))
	assert.True(t, MatchesPattern("test/**/tull", "test/test1/test2/tull"))
	assert.True(t, MatchesPattern("release/**/*", "release/0.1.3"))
	assert.True(t, MatchesPattern("release/**/*", "release/q3/0.1.3"))
	assert.True(t, MatchesPattern("v\\d+\\.\\d+\\.\\d+", "v1.0.2"))
	assert.True(t, MatchesPattern("v\\d+\\.\\d+\\.\\d+", "v123.033.2112"))
	assert.False(t, MatchesPattern("v\\d+\\.\\d+\\.\\d+", "v1q.0.2"))
	assert.False(t, MatchesPattern("v\\d+\\.\\d+\\.\\d+", "v1..2"))
	assert.False(t, MatchesPattern("v\\d+\\.\\d+\\.\\d+", "v1.2"))
}
