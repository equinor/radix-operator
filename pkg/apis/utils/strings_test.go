package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetGithubCloneURLFromRepo_ValidRepo_CreatesValidClone(t *testing.T) {
	expected := "git@github.com:Equinor/my-app.git"
	actual := GetGithubCloneURLFromRepo("https://github.com/Equinor/my-app")

	assert.Equal(t, expected, actual, "getCloneURLFromRepo - not equal")
}

func TestGetGithubCloneURLFromRepo_EmptyRepo_CreatesEmptyClone(t *testing.T) {
	expected := ""
	actual := GetGithubCloneURLFromRepo("")

	assert.Equal(t, expected, actual, "getCloneURLFromRepo - not equal")
}

func TestCloneToRepositoryURL_ValidUrl(t *testing.T) {
	cloneURL := "git@github.com:equinor/radix-api.git"
	repo := GetGithubRepositoryURLFromCloneURL(cloneURL)

	assert.Equal(t, "https://github.com/equinor/radix-api", repo)
}

func TestCloneToRepositoryURL_EmptyURL(t *testing.T) {
	cloneURL := ""
	repo := GetGithubRepositoryURLFromCloneURL(cloneURL)

	assert.Equal(t, "", repo)
}
