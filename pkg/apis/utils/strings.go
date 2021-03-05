package utils

import (
	"fmt"
	"regexp"
	"strings"
)

const githubRepoURL = "https://github.com/"
const githubSSHURL = "git@github.com:"

var githubRepoPattern = regexp.MustCompile(fmt.Sprintf("%s(.*?)", githubRepoURL))

// GetGithubCloneURLFromRepo Takes a https repo string as input and converts to a git clone url
func GetGithubCloneURLFromRepo(repo string) string {
	if repo == "" {
		return ""
	}

	cloneURL := githubRepoPattern.ReplaceAllString(repo, githubSSHURL)
	cloneURL += ".git"
	return cloneURL
}

// GetGithubRepositoryURLFromCloneURL Takes git clone url as input and converts to a https repo string
func GetGithubRepositoryURLFromCloneURL(cloneURL string) string {
	if cloneURL == "" {
		return ""
	}

	repoName := strings.TrimSuffix(strings.TrimPrefix(cloneURL, githubSSHURL), ".git")
	repo := fmt.Sprintf("%s%s", githubRepoURL, repoName)
	return repo
}

// TernaryString operator
func TernaryString(condition bool, trueValue, falseValue string) string {
	return map[bool]string{true: trueValue, false: falseValue}[condition]
}

// StringPtr returns a pointer to the passed string.
func StringPtr(s string) *string {
	return &s
}

// BoolPtr returns a pointer to the passed bool.
func BoolPtr(value bool) *bool {
	return &value
}
