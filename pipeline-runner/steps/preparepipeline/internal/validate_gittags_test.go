package internal_test

import (
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/steps/preparepipeline/internal"
	"github.com/stretchr/testify/assert"
)

func Test_invalid_git_tags_returns_error(t *testing.T) {
	validGitTags := "v1.12 v1.13"
	err := internal.GitTagsContainIllegalChars(validGitTags)
	assert.NoError(t, err)

	invalidGitTags := "v1.12\" v1.13"
	err = internal.GitTagsContainIllegalChars(invalidGitTags)
	assert.Error(t, err)
	assert.EqualError(t, err, "git tags v1.12\" v1.13 contained one or more illegal characters \"'$")

	invalidGitTags2 := "v1.12' v1.13"
	err = internal.GitTagsContainIllegalChars(invalidGitTags2)
	assert.Error(t, err)
	assert.EqualError(t, err, "git tags v1.12' v1.13 contained one or more illegal characters \"'$")
}
