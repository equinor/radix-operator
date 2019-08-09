package antpath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidBranchPattern(t *testing.T) {
	assert.False(t, IsValidBranchPattern(""))
	assert.False(t, IsValidBranchPattern("/test"))
	assert.False(t, IsValidBranchPattern("\\"))

	assert.True(t, IsValidBranchPattern("😀"))
	assert.True(t, IsValidBranchPattern("t+est"))
	assert.True(t, IsValidBranchPattern("test"))
	assert.True(t, IsValidBranchPattern("test/tull"))
	assert.True(t, IsValidBranchPattern("hotfix/**/*"))
	assert.True(t, IsValidBranchPattern("release/*"))
	assert.True(t, IsValidBranchPattern("feature/*"))
	assert.True(t, IsValidBranchPattern("tes?"))
	assert.True(t, IsValidBranchPattern("te??"))
	assert.True(t, IsValidBranchPattern("??st"))
	assert.True(t, IsValidBranchPattern("?est/*"))
	assert.True(t, IsValidBranchPattern("te?t/*"))
}

func TestBranchMatchesPattern(t *testing.T) {
	assert.False(t, BranchMatchesPattern("Test", "test"))

	assert.True(t, BranchMatchesPattern("test", "test"))
	assert.True(t, BranchMatchesPattern("te??", "test"))
	assert.True(t, BranchMatchesPattern("??st", "test"))
	assert.True(t, BranchMatchesPattern("*", "test"))
	assert.True(t, BranchMatchesPattern("test*", "testTest"))
	assert.True(t, BranchMatchesPattern("test/*", "test/Test"))
	assert.True(t, BranchMatchesPattern("test/*", "test/t"))
	assert.True(t, BranchMatchesPattern("*test*", "AnothertestTest"))
	assert.True(t, BranchMatchesPattern("*test", "Anothertest"))
	assert.True(t, BranchMatchesPattern("test/*/tull", "test/test1/test2/tull"))
	assert.True(t, BranchMatchesPattern("test/**/tull", "test/test1/test2/tull"))
}
