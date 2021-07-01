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

func EqualStringsAsPtr(s1 *string, s2 *string) bool {
	return ((s1 == nil) == (s2 == nil)) && (s1 != nil && strings.EqualFold(*s1, *s2))
}

//EqualStringMaps Compare two string maps
func EqualStringMaps(map1 map[string]string, map2 map[string]string) bool {
	if len(map1) != len(map2) {
		return false
	}
	processedStringKeySet := make(map[string]bool)
	if !equalStringMapsWithCache(map1, map2, processedStringKeySet) {
		return false
	}
	return equalStringMapsWithCache(map2, map1, processedStringKeySet)
}

func equalStringMapsWithCache(map1 map[string]string, map2 map[string]string, processedKeySet map[string]bool) bool {
	for key, val1 := range map1 {
		if _, ok := processedKeySet[key]; ok {
			continue
		}
		val2, ok := map2[key]
		if !ok {
			return false
		}
		if !strings.EqualFold(val1, val2) {
			return false
		}
		processedKeySet[key] = false
	}
	return true
}

func ShortenString(s string, charsToCut int) string {
	return s[:len(s)-charsToCut]
}
