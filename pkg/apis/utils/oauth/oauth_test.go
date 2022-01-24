package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_SanitizePathPrefix(t *testing.T) {
	assert.Equal(t, "/foo", SanitizePathPrefix("foo"))
	assert.Equal(t, "/foo", SanitizePathPrefix("/foo/"))
	assert.Equal(t, "/foo", SanitizePathPrefix("/foo/bar/.."))
	assert.Equal(t, "/", SanitizePathPrefix("/foo/../../.."))
	assert.Equal(t, "/foo%3Fx=y", SanitizePathPrefix("/foo?x=y"))
}
