package utils

import (
	"fmt"
	"regexp"
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
